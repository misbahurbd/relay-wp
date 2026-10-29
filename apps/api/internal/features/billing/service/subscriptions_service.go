package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Track T3: subscriptions.

// Subscription returns the workspace's live subscription, or its latest one if
// none is live. model.ErrNoSubscription if it never had one.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) Subscription(ctx context.Context, workspaceID uuid.UUID) (model.Subscription, error) {
	return model.Subscription{}, model.ErrNotImplemented
}

// PreviewChange says what moving the live subscription to planPriceID would
// do: an upgrade now with proration, or a downgrade at the period end (Q6).
//
//nolint:revive // stub: track T3 builds it
func (s *Service) PreviewChange(ctx context.Context, workspaceID, planPriceID uuid.UUID) (model.ChangeQuote, error) {
	return model.ChangeQuote{}, model.ErrNotImplemented
}

// ChangePlan moves the live subscription to planPriceID: an upgrade applies now
// (prorated, with a new grant), a downgrade is scheduled through
// pending_plan_price_id. Both are logged in subscription_changes.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) ChangePlan(ctx context.Context, workspaceID, userID, planPriceID uuid.UUID) (model.Subscription, error) {
	return model.Subscription{}, model.ErrNotImplemented
}

// CancelPendingChange drops a scheduled downgrade. model.ErrNoPendingChange if there is none.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) CancelPendingChange(ctx context.Context, workspaceID, userID uuid.UUID) (model.Subscription, error) {
	return model.Subscription{}, model.ErrNotImplemented
}

// CancelSubscription cancels the live subscription at the period end, or now
// (its grant ends with it).
//
//nolint:revive // stub: track T3 builds it
func (s *Service) CancelSubscription(ctx context.Context, workspaceID, userID uuid.UUID, atPeriodEnd bool) (model.Subscription, error) {
	return model.Subscription{}, model.ErrNotImplemented
}

// ResumeSubscription undoes a cancel at the period end. model.ErrNotCanceling if none is set.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) ResumeSubscription(ctx context.Context, workspaceID, userID uuid.UUID) (model.Subscription, error) {
	return model.Subscription{}, model.ErrNotImplemented
}

// SubscriptionCredit is what CancelForLTD would credit right now: the unused
// time on the live subscription. Zero with no live subscription. Track T4's
// QuoteOrder uses it.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) SubscriptionCredit(ctx context.Context, workspaceID uuid.UUID) (model.SubscriptionCredit, error) {
	return model.SubscriptionCredit{}, model.ErrNotImplemented
}

// CancelForLTD cancels the live subscription now, in tx, because the workspace
// is buying a lifetime deal (Q13), and returns the unused time as credit for
// orders.credit_minor. Zero with no live subscription. Track T4 calls it.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) CancelForLTD(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID) (model.SubscriptionCredit, error) {
	return model.SubscriptionCredit{}, model.ErrNotImplemented
}

// SyncSubscription copies the provider's view of a subscription in tx
// (subscription.updated and subscription.canceled events) and changes the
// grant to match: a new period keeps it, past_due keeps it until
// grace_ends_at, canceled ends it.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) SyncSubscription(ctx context.Context, tx pgx.Tx, workspaceID, subscriptionID uuid.UUID, sub payment.Subscription) error {
	return model.ErrNotImplemented
}

// ApplySubscriptionPayment records a renewal payment in tx (payment.succeeded
// or payment.failed for a subscription): an invoice with bill_to, and a
// pending downgrade applied when its period starts.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) ApplySubscriptionPayment(ctx context.Context, tx pgx.Tx, workspaceID, subscriptionID uuid.UUID, p payment.Payment, paid bool) error {
	return model.ErrNotImplemented
}
