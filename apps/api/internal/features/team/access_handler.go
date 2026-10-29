package team

import (
	"context"
)

// ---------- access ----------

func (h *Handler) myAccess(ctx context.Context, _ *workspaceInput) (*accessOutput, error) {
	return &accessOutput{Body: toAccessDTO(actor(ctx))}, nil
}

func (h *Handler) setAvailability(ctx context.Context, in *availabilityInput) (*struct{}, error) {
	return nil, h.svc.SetAvailability(ctx, actor(ctx), in.Body.Availability)
}

func (h *Handler) permissions(ctx context.Context, _ *struct{}) (*permissionsOutput, error) {
	list, err := h.svc.Permissions(ctx)
	if err != nil {
		return nil, err
	}
	out := &permissionsOutput{}
	out.Body.Permissions = make([]permissionDTO, len(list))
	for i, p := range list {
		out.Body.Permissions[i] = permissionDTO(p)
	}
	return out, nil
}

func (h *Handler) seats(ctx context.Context, in *workspaceInput) (*seatsOutput, error) {
	s, err := h.svc.Seats(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return &seatsOutput{Body: seatsDTO(s)}, nil
}
