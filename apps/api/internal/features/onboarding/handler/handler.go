// Package handler serves onboarding's HTTP endpoints: it decodes requests, calls the service and
// encodes responses. Routes are registered in routes.go.
package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/onboarding/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/onboarding/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/onboarding/service"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Handler turns HTTP requests into service calls.
type Handler struct {
	svc *service.Service
}

// NewHandler builds the onboarding HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *service.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) get(ctx context.Context, in *dto.WorkspaceInput) (*dto.OnboardingOutput, error) {
	return respond(h.svc.Get(ctx, in.WorkspaceID, userID(ctx)))
}

func (h *Handler) skipWhatsApp(ctx context.Context, in *dto.WorkspaceInput) (*dto.OnboardingOutput, error) {
	return respond(h.svc.SkipWhatsApp(ctx, in.WorkspaceID, userID(ctx)))
}

func (h *Handler) complete(ctx context.Context, in *dto.WorkspaceInput) (*dto.OnboardingOutput, error) {
	return respond(h.svc.Complete(ctx, in.WorkspaceID, userID(ctx)))
}

func (h *Handler) updateMemberState(ctx context.Context, in *dto.MemberStateInput) (*dto.OnboardingOutput, error) {
	return respond(h.svc.UpdateMemberState(ctx, in.WorkspaceID, userID(ctx), model.MemberStateInput{
		TourSeen: in.Body.TourSeen, ChecklistHidden: in.Body.ChecklistHidden,
	}))
}

func respond(o model.Onboarding, err error) (*dto.OnboardingOutput, error) {
	if err != nil {
		return nil, err
	}
	return dto.ToOutput(o), nil
}

// userID is set by middleware.RequireAuth on every route here.
func userID(ctx context.Context) uuid.UUID {
	id, _ := authctx.UserID(ctx)
	return id
}
