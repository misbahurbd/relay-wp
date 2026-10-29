package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
)

// listPlans: GET /v1/billing/plans: the public catalog (svc.Catalog).
func (h *Handler) listPlans(ctx context.Context, _ *struct{}) (*dto.CatalogOutput, error) {
	c, err := h.svc.Catalog(ctx)
	if err != nil {
		return nil, err
	}
	return dto.ToCatalogOutput(c), nil
}
