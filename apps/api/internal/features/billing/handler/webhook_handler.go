package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

// Track T5 builds these. Until then each answers 501 not_implemented.

// receiveWebhook: POST /v1/webhooks/payments/{provider}: svc.ReceiveWebhook with in.Provider, in.Header and in.RawBody, then 200.
func (h *Handler) receiveWebhook(context.Context, *dto.PaymentWebhookInput) (*struct{}, error) {
	return nil, model.ErrNotImplemented
}
