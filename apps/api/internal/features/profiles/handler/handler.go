// Package handler serves profiles' HTTP endpoints: it decodes requests, calls the service and
// encodes responses. Routes are registered in routes.go.
package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/profiles/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/profiles/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/profiles/service"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Handler turns HTTP requests into service calls.
type Handler struct {
	svc *service.Service
}

// NewHandler builds the profiles HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *service.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) me(ctx context.Context, _ *struct{}) (*dto.MeOutput, error) {
	m, err := h.svc.Me(ctx, userID(ctx))
	if err != nil {
		return nil, err
	}
	return dto.ToMeOutput(m), nil
}

func (h *Handler) updateProfile(ctx context.Context, in *dto.UpdateProfileInput) (*dto.MeOutput, error) {
	m, err := h.svc.UpdateProfile(ctx, userID(ctx), model.ProfileUpdate{
		FirstName: in.Body.FirstName, LastName: in.Body.LastName, DisplayName: in.Body.DisplayName,
	})
	if err != nil {
		return nil, err
	}
	return dto.ToMeOutput(m), nil
}

func (h *Handler) updatePreferences(ctx context.Context, in *dto.UpdatePreferencesInput) (*dto.MeOutput, error) {
	m, err := h.svc.UpdatePreferences(ctx, userID(ctx), model.PreferencesUpdate{Locale: in.Body.Locale, Timezone: in.Body.Timezone})
	if err != nil {
		return nil, err
	}
	return dto.ToMeOutput(m), nil
}

// userID is set by middleware.RequireAuth on every route here.
func userID(ctx context.Context) uuid.UUID {
	id, _ := authctx.UserID(ctx)
	return id
}
