package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

// Track T6 builds these. Until then each answers 501 not_implemented.

// profile: GET /profile (svc.Profile).
func (h *Handler) profile(context.Context, *dto.WorkspaceInput) (*dto.ProfileOutput, error) {
	return nil, model.ErrNotImplemented
}

// updateProfile: PUT /profile (svc.UpdateProfile).
func (h *Handler) updateProfile(context.Context, *dto.UpdateProfileInput) (*dto.ProfileOutput, error) {
	return nil, model.ErrNotImplemented
}
