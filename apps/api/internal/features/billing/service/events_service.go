package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Track T5: webhooks and the jobs that keep billing tidy.

// eventHandlers maps each normalised event type to the rules that apply it,
// calling into the T3 and T4 methods that take a pgx.Tx. New calls it once.
func (s *Service) eventHandlers() map[payment.EventType]eventHandler {
	return map[payment.EventType]eventHandler{}
}

// ReceiveWebhook verifies a webhook with the provider's gateway, stores it in
// payment_events and enqueues ProcessPaymentEventArgs in one transaction, then
// returns so the handler can answer 200 quickly. A replay (same provider event
// ID) is a no-op. A bad signature returns model.ErrWebhookSignature and stores
// nothing; a provider that isn't enabled returns model.ErrProviderNotAvailable.
// header is the request's headers (http.Header).
//
//nolint:revive // stub: track T5 builds it
func (s *Service) ReceiveWebhook(ctx context.Context, provider string, header map[string][]string, body []byte) error {
	p, ok := payment.ParseProvider(provider)
	if !ok {
		return model.ErrProviderNotAvailable
	}
	if _, err := s.gateway(p); err != nil {
		return err
	}
	return model.ErrNotImplemented
}

// ProcessPaymentEvent applies one stored event: it finds the workspace through
// the event's reference (our checkout session ID) or a provider ID we stored,
// dispatches to the event's handler, and sets processed_at, all in one
// transaction. Already processed events are skipped. The ProcessPaymentEvent
// worker calls it; River retries on error.
//
//nolint:revive // stub: track T5 builds it
func (s *Service) ProcessPaymentEvent(ctx context.Context, paymentEventID uuid.UUID, ev payment.Event) error {
	return model.ErrNotImplemented
}

// PurgeEventPayloads nulls payment_events.payload older than cfg.EventPayloadTTL
// and returns how many it cleared. Run by the PurgeEventPayloads worker.
//
//nolint:revive // stub: track T5 builds it
func (s *Service) PurgeEventPayloads(ctx context.Context) (int64, error) {
	return 0, model.ErrNotImplemented
}

// CheckPlanCacheDrift logs every workspace whose workspaces.plan disagrees with
// its current grant (Q8) and returns how many it found. Run nightly by the
// PlanCacheDrift worker.
//
//nolint:revive // stub: track T5 builds it
func (s *Service) CheckPlanCacheDrift(ctx context.Context) (int, error) {
	return 0, model.ErrNotImplemented
}
