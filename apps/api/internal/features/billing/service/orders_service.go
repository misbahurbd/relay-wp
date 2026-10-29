package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Track T4: lifetime-deal orders.

// QuoteOrder says what a lifetime deal would cost the workspace now: the
// price or offer total, less a discount code, less credit from a live
// subscription (Q13) or the live order it would upgrade (Q16), split into the
// installment schedule.
//
//nolint:revive // stub: track T4 builds it
func (s *Service) QuoteOrder(ctx context.Context, workspaceID uuid.UUID, in model.QuoteInput) (model.OrderQuote, error) {
	return model.OrderQuote{}, model.ErrNotImplemented
}

// CurrentOrder returns the workspace's live order with its installments.
// model.ErrNoOrder if there is none.
//
//nolint:revive // stub: track T4 builds it
func (s *Service) CurrentOrder(ctx context.Context, workspaceID uuid.UUID) (model.Order, error) {
	return model.Order{}, model.ErrNotImplemented
}

// Order returns one of the workspace's orders with its installments.
//
//nolint:revive // stub: track T4 builds it
func (s *Service) Order(ctx context.Context, workspaceID, orderID uuid.UUID) (model.Order, error) {
	return model.Order{}, model.ErrNotImplemented
}

// CreateOrder creates a pending order and every installment row from a quote
// in tx, when a lifetime-deal checkout starts. Track T3's CreateCheckout calls it.
//
//nolint:revive // stub: track T4 builds it
func (s *Service) CreateOrder(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, in model.NewOrder) (model.Order, error) {
	return model.Order{}, model.ErrNotImplemented
}

// CompleteOrderCheckout applies a paid ltd or installment checkout in tx: the
// first payment makes the order in_installments (or paid) with a lifetime
// grant (Q1), cancels a replaced subscription through CancelForLTD (Q13) or
// upgrades the old order (Q16). Track T3's CompleteCheckout calls it.
//
//nolint:revive // stub: track T4 builds it
func (s *Service) CompleteOrderCheckout(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, session model.CheckoutSession, p payment.Payment) error {
	return model.ErrNotImplemented
}

// ResumeOrder starts a checkout for a defaulted order's remaining balance;
// paying it restores the lifetime grant (Q12).
//
//nolint:revive // stub: track T4 builds it
func (s *Service) ResumeOrder(ctx context.Context, workspaceID, userID, orderID uuid.UUID, idempotencyKey string) (model.StartedCheckout, error) {
	return model.StartedCheckout{}, model.ErrNotImplemented
}
