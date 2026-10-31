package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// WorkspaceHeader names the workspace a request is scoped to. Every route under
// /v1/workspace needs it; the web and mobile apps send the same header.
const WorkspaceHeader = "X-Workspace-ID"

// AccessVersionHeader tells the web app when the member's access changed, so it
// can refetch GET /me/access.
const AccessVersionHeader = "X-Access-Version"

// AccessResolver returns a member's access in a workspace; ok is false when they
// are not a member. Implemented by features/team with one query (ADR 0002).
type AccessResolver interface {
	ResolveAccess(ctx context.Context, workspaceID, userID uuid.UUID) (a authctx.Access, suspended, ok bool, err error)
}

var (
	errWorkspaceRequired = apperr.Validation("workspace_required", "Send the workspace ID in the "+WorkspaceHeader+" header")
	errWorkspaceNotFound = apperr.NotFound("workspace_not_found", "Workspace not found")
	errMemberSuspended   = apperr.Forbidden("member_suspended",
		"Your access to this workspace is paused. Ask a workspace admin to restore it.")
	errPermissionDenied = apperr.Forbidden("permission_denied", "Your role in this workspace can't do that")
	errOwnerOnly        = apperr.Forbidden("owner_only", "Only the workspace owner can do that")
	errAccessCheck      = apperr.Unavailable("membership_check_failed", "Could not check your workspace access. Try again.")
	errPlanRequired     = apperr.PaymentRequired("plan_required", "Your plan doesn't include this. Upgrade to use it.")
	errPlanCheck        = apperr.Unavailable("plan_check_failed", "Could not check your plan. Try again.")
)

// RequireWorkspace is a huma middleware for routes under /v1/workspace. It runs
// after RequireAuth and reads the workspace from WorkspaceHeader (an API key's
// request must name the key's workspace); a missing
// header gets 422. A user who isn't a member gets 404, so workspace IDs can't be
// probed; a suspended member gets 403. The member's access goes into the
// context (authctx.AccessFrom).
func RequireWorkspace(api huma.API, r AccessResolver) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		// The same URL answers differently per workspace; keep shared caches apart.
		ctx.AppendHeader("Vary", WorkspaceHeader)
		raw := strings.TrimSpace(ctx.Header(WorkspaceHeader))
		if raw == "" {
			_ = huma.WriteErr(api, ctx, http.StatusUnprocessableEntity, "", errWorkspaceRequired)
			return
		}
		userID, signedIn := authctx.UserID(ctx.Context())
		workspaceID, err := uuid.Parse(raw)
		if !signedIn || err != nil {
			_ = huma.WriteErr(api, ctx, http.StatusNotFound, "", errWorkspaceNotFound)
			return
		}
		// An API key works only in the workspace it was made in.
		if keyWorkspace, byKey := authctx.APIKeyWorkspace(ctx.Context()); byKey && keyWorkspace != workspaceID {
			_ = huma.WriteErr(api, ctx, http.StatusNotFound, "", errWorkspaceNotFound)
			return
		}
		a, suspended, ok, err := r.ResolveAccess(ctx.Context(), workspaceID, userID)
		switch {
		case err != nil:
			_ = huma.WriteErr(api, ctx, http.StatusServiceUnavailable, "", errAccessCheck.Wrap(err))
			return
		case !ok:
			_ = huma.WriteErr(api, ctx, http.StatusNotFound, "", errWorkspaceNotFound)
			return
		case suspended:
			_ = huma.WriteErr(api, ctx, http.StatusForbidden, "", errMemberSuspended)
			return
		}
		ctx.SetHeader(AccessVersionHeader, fmt.Sprintf("%d.%d", a.RoleVersion, a.AccessVersion))
		next(huma.WithContext(ctx, authctx.WithWorkspace(ctx.Context(), a)))
	}
}

// RequirePermission lets the request through when the member has every one of
// perms. It runs after RequireWorkspace. The error names what's missing so the
// web app can say so.
func RequirePermission(api huma.API, perms ...authctx.Permission) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		a, _ := authctx.AccessFrom(ctx.Context())
		var missing []string
		for _, p := range perms {
			if !a.Can(p) {
				missing = append(missing, p.Key())
			}
		}
		if len(missing) > 0 {
			_ = huma.WriteErr(api, ctx, http.StatusForbidden, "",
				errPermissionDenied.WithDetails(apperr.Detail{Field: "permission", Message: strings.Join(missing, ",")}))
			return
		}
		next(ctx)
	}
}

// PlanAccess says whether a workspace's plan includes an access resource (ADR
// 0008). Implemented by features/billing.
type PlanAccess interface {
	HasAccess(ctx context.Context, workspaceID uuid.UUID, key string) (bool, error)
}

// RequirePlan lets the request through when the workspace's plan includes key,
// and answers 402 plan_required when it doesn't. It runs after RequireWorkspace.
func RequirePlan(api huma.API, p PlanAccess, key string) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		a, _ := authctx.AccessFrom(ctx.Context())
		ok, err := p.HasAccess(ctx.Context(), a.WorkspaceID, key)
		switch {
		case err != nil:
			_ = huma.WriteErr(api, ctx, http.StatusServiceUnavailable, "", errPlanCheck.Wrap(err))
		case !ok:
			_ = huma.WriteErr(api, ctx, http.StatusPaymentRequired, "",
				errPlanRequired.WithDetails(apperr.Detail{Field: "resource", Message: key}))
		default:
			next(ctx)
		}
	}
}

// RequireOwner lets only the workspace owner through. It runs after RequireWorkspace.
func RequireOwner(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if a, _ := authctx.AccessFrom(ctx.Context()); !a.Owner {
			_ = huma.WriteErr(api, ctx, http.StatusForbidden, "", errOwnerOnly)
			return
		}
		next(ctx)
	}
}
