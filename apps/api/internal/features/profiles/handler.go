package profiles

import (
	"context"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Handler turns HTTP requests into service calls.
type Handler struct {
	svc *Service
}

// NewHandler builds the profiles HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) me(ctx context.Context, _ *struct{}) (*meOutput, error) {
	m, err := h.svc.Me(ctx, userID(ctx))
	if err != nil {
		return nil, err
	}
	return toMeOutput(m), nil
}

func (h *Handler) updateProfile(ctx context.Context, in *updateProfileInput) (*meOutput, error) {
	m, err := h.svc.UpdateProfile(ctx, userID(ctx), ProfileUpdate{
		FirstName: in.Body.FirstName, LastName: in.Body.LastName, DisplayName: in.Body.DisplayName,
	})
	if err != nil {
		return nil, err
	}
	return toMeOutput(m), nil
}

func (h *Handler) updatePreferences(ctx context.Context, in *updatePreferencesInput) (*meOutput, error) {
	m, err := h.svc.UpdatePreferences(ctx, userID(ctx), PreferencesUpdate{Locale: in.Body.Locale, Timezone: in.Body.Timezone})
	if err != nil {
		return nil, err
	}
	return toMeOutput(m), nil
}

// userID is set by middleware.RequireAuth on every route here.
func userID(ctx context.Context) uuid.UUID {
	id, _ := authctx.UserID(ctx)
	return id
}
