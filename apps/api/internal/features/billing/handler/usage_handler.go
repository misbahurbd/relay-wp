package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
)

// usage: GET /v1/workspace/billing/usage (svc.Usage).
func (h *Handler) usage(ctx context.Context, in *dto.WorkspaceInput) (*dto.UsageOutput, error) {
	u, err := h.svc.Usage(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return dto.ToUsageOutput(u), nil
}
