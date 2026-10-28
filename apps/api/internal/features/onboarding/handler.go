package onboarding

import (
	"context"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Handler turns HTTP requests into service calls.
type Handler struct {
	svc *Service
}

// NewHandler builds the onboarding HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) get(ctx context.Context, in *workspaceInput) (*onboardingOutput, error) {
	return respond(h.svc.Get(ctx, in.WorkspaceID, userID(ctx)))
}

func (h *Handler) skipWhatsApp(ctx context.Context, in *workspaceInput) (*onboardingOutput, error) {
	return respond(h.svc.SkipWhatsApp(ctx, in.WorkspaceID, userID(ctx)))
}

func (h *Handler) complete(ctx context.Context, in *workspaceInput) (*onboardingOutput, error) {
	return respond(h.svc.Complete(ctx, in.WorkspaceID, userID(ctx)))
}

func (h *Handler) updateMemberState(ctx context.Context, in *memberStateInput) (*onboardingOutput, error) {
	return respond(h.svc.UpdateMemberState(ctx, in.WorkspaceID, userID(ctx), MemberStateInput{
		TourSeen: in.Body.TourSeen, ChecklistHidden: in.Body.ChecklistHidden,
	}))
}

func respond(o Onboarding, err error) (*onboardingOutput, error) {
	if err != nil {
		return nil, err
	}
	return toOutput(o), nil
}

// userID is set by middleware.RequireAuth on every route here.
func userID(ctx context.Context) uuid.UUID {
	id, _ := authctx.UserID(ctx)
	return id
}
