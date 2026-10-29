package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Track T3: checkout sessions.

// CreateCheckout starts a checkout for a price: a subscription (purpose new),
// or a lifetime deal in full or in installments (purpose ltd, which creates
// the pending order through CreateOrder). It applies and reserves a discount
// code, offers any enabled provider that has a price (Q10 deferred), and calls
// the gateway. The same IdempotencyKey returns the first session.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) CreateCheckout(ctx context.Context, workspaceID, userID uuid.UUID, in model.CheckoutInput) (model.StartedCheckout, error) {
	return model.StartedCheckout{}, model.ErrNotImplemented
}

// StartPaymentCheckout starts a one-off payment on an existing order: an
// app_link installment or a defaulted order's balance (purpose installment).
// Track T4's PayInstallment and ResumeOrder call it.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) StartPaymentCheckout(ctx context.Context, workspaceID, userID uuid.UUID, in model.PaymentCheckoutInput) (model.StartedCheckout, error) {
	return model.StartedCheckout{}, model.ErrNotImplemented
}

// CheckoutSession returns one of the workspace's checkout sessions, so the
// browser can see it complete. model.ErrCheckoutNotFound if there is none.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) CheckoutSession(ctx context.Context, workspaceID, sessionID uuid.UUID) (model.CheckoutSession, error) {
	return model.CheckoutSession{}, model.ErrNotImplemented
}

// CompleteCheckout applies checkout.completed in tx: the session becomes
// completed, its discount redeemed, and then by purpose a subscription is
// activated with its grant (T3) or an order is paid through
// CompleteOrderCheckout (T4). A session that already finished is a no-op.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) CompleteCheckout(ctx context.Context, tx pgx.Tx, workspaceID, sessionID uuid.UUID, ev payment.Event) error {
	return model.ErrNotImplemented
}

// FailCheckout ends a pending session as model.CheckoutFailed or
// model.CheckoutExpired in tx and releases its reserved discount.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) FailCheckout(ctx context.Context, tx pgx.Tx, workspaceID, sessionID uuid.UUID, status string) error {
	return model.ErrNotImplemented
}

// ExpireCheckouts expires pending sessions past expires_at, releasing their
// discounts, and returns how many. Run by the ExpireCheckouts worker.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) ExpireCheckouts(ctx context.Context) (int, error) {
	return 0, model.ErrNotImplemented
}
