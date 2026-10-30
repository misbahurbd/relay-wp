package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
)

// receiveWebhook: POST /v1/webhooks/payments/{provider}. The service verifies,
// stores and enqueues the event; the provider gets 200 as soon as it is stored
// (or recognised as a replay).
func (h *Handler) receiveWebhook(ctx context.Context, in *dto.PaymentWebhookInput) (*struct{}, error) {
	if err := h.svc.ReceiveWebhook(ctx, in.Provider, in.Header, in.RawBody); err != nil {
		return nil, err
	}
	return &struct{}{}, nil
}
