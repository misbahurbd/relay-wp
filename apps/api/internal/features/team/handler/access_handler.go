package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/dto"
)

// ---------- access ----------

func (h *Handler) myAccess(ctx context.Context, _ *dto.WorkspaceInput) (*dto.AccessOutput, error) {
	return &dto.AccessOutput{Body: dto.ToAccessDTO(actor(ctx))}, nil
}

func (h *Handler) setAvailability(ctx context.Context, in *dto.AvailabilityInput) (*struct{}, error) {
	return nil, h.svc.SetAvailability(ctx, actor(ctx), in.Body.Availability)
}

func (h *Handler) permissions(ctx context.Context, _ *struct{}) (*dto.PermissionsOutput, error) {
	list, err := h.svc.Permissions(ctx)
	if err != nil {
		return nil, err
	}
	out := &dto.PermissionsOutput{}
	out.Body.Permissions = make([]dto.PermissionDTO, len(list))
	for i, p := range list {
		out.Body.Permissions[i] = dto.PermissionDTO(p)
	}
	return out, nil
}

func (h *Handler) seats(ctx context.Context, in *dto.WorkspaceInput) (*dto.SeatsOutput, error) {
	s, err := h.svc.Seats(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return &dto.SeatsOutput{Body: dto.SeatsDTO(s)}, nil
}
