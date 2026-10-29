package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

// Track T6 builds these. Until then each answers 501 not_implemented.

// previewDiscount: POST /discounts/preview (svc.PreviewDiscount).
func (h *Handler) previewDiscount(context.Context, *dto.PreviewDiscountInput) (*dto.DiscountPreviewOutput, error) {
	return nil, model.ErrNotImplemented
}
