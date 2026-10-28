package middleware

import (
	"context"
	"net/http"
	"slices"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// WorkspaceParam is the path parameter every workspace-scoped route uses.
const WorkspaceParam = "workspaceId"

// MembershipChecker returns the user's role in a workspace; ok is false when
// they are not a member.
type MembershipChecker interface {
	MemberRole(ctx context.Context, workspaceID, userID uuid.UUID) (role string, ok bool, err error)
}

var (
	errWorkspaceNotFound = apperr.NotFound("workspace_not_found", "Workspace not found")
	errRoleForbidden     = apperr.Forbidden("forbidden", "Your role in this workspace can't do that")
	errMembershipCheck   = apperr.Unavailable("membership_check_failed", "Could not check your workspace access. Try again.")
)

// RequireWorkspace is a huma middleware for routes under /v1/workspaces/{workspaceId}.
// It runs after RequireAuth. A user who isn't a member gets 404, so workspace IDs
// can't be probed; with roles given, other members get 403. The workspace and
// role go into the context (authctx.WorkspaceID, authctx.Role).
func RequireWorkspace(api huma.API, m MembershipChecker, roles ...string) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		userID, signedIn := authctx.UserID(ctx.Context())
		workspaceID, err := uuid.Parse(ctx.Param(WorkspaceParam))
		if !signedIn || err != nil {
			_ = huma.WriteErr(api, ctx, http.StatusNotFound, "", errWorkspaceNotFound)
			return
		}
		role, ok, err := m.MemberRole(ctx.Context(), workspaceID, userID)
		switch {
		case err != nil:
			_ = huma.WriteErr(api, ctx, http.StatusServiceUnavailable, "", errMembershipCheck.Wrap(err))
			return
		case !ok:
			_ = huma.WriteErr(api, ctx, http.StatusNotFound, "", errWorkspaceNotFound)
			return
		case len(roles) > 0 && !slices.Contains(roles, role):
			_ = huma.WriteErr(api, ctx, http.StatusForbidden, "", errRoleForbidden)
			return
		}
		next(huma.WithContext(ctx, authctx.WithWorkspace(ctx.Context(), workspaceID, role)))
	}
}
