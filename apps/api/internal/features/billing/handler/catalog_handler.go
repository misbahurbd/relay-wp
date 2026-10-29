package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

// Track T6 builds these. Until then each answers 501 not_implemented.

// listPlans: GET /v1/billing/plans: the public catalog (svc.Catalog).
func (h *Handler) listPlans(context.Context, *struct{}) (*dto.CatalogOutput, error) {
	return nil, model.ErrNotImplemented
}
