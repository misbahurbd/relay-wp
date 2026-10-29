package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

// Track T6 builds these. Until then each answers 501 not_implemented.

// createCheckout: POST /checkout-sessions (svc.CreateCheckout).
func (h *Handler) createCheckout(context.Context, *dto.CreateCheckoutInput) (*dto.StartedCheckoutOutput, error) {
	return nil, model.ErrNotImplemented
}

// checkoutSession: GET /checkout-sessions/{sessionId} (svc.CheckoutSession).
func (h *Handler) checkoutSession(context.Context, *dto.CheckoutSessionInput) (*dto.CheckoutSessionOutput, error) {
	return nil, model.ErrNotImplemented
}
