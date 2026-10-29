package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
)

// previewDiscount: POST /discounts/preview (svc.PreviewDiscount). A code that
// doesn't apply answers 422 with the service's reason as the code.
func (h *Handler) previewDiscount(ctx context.Context, in *dto.PreviewDiscountInput) (*dto.DiscountPreviewOutput, error) {
	p, err := h.svc.PreviewDiscount(ctx, in.WorkspaceID, in.ToModel())
	if err != nil {
		return nil, err
	}
	return dto.ToDiscountPreviewOutput(p), nil
}
