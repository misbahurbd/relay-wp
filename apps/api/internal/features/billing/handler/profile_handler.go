package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
)

// profile: GET /profile (svc.Profile).
func (h *Handler) profile(ctx context.Context, in *dto.WorkspaceInput) (*dto.ProfileOutput, error) {
	p, err := h.svc.Profile(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return dto.ToProfileOutput(p), nil
}

// updateProfile: PUT /profile (svc.UpdateProfile).
func (h *Handler) updateProfile(ctx context.Context, in *dto.UpdateProfileInput) (*dto.ProfileOutput, error) {
	p, err := h.svc.UpdateProfile(ctx, in.WorkspaceID, in.ToModel())
	if err != nil {
		return nil, err
	}
	return dto.ToProfileOutput(p), nil
}
