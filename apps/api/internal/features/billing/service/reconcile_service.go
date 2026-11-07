package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

const (
	// checkoutSettle is how long a checkout's own webhook gets before Relay asks the provider.
	checkoutSettle = 5 * time.Minute
	// renewalSettle is how long after a period ends a renewal's webhooks get before Relay asks.
	renewalSettle = time.Hour
)

// ReconcileCheckouts settles pending checkouts whose webhook hasn't come. Each
// one older than checkoutSettle is read back from the provider: a paid one is
// applied as its webhook would be, and an unpaid one past expires_at is
// expired, releasing its discount. A provider that can't be asked right now
// leaves its sessions for the next run, so "you haven't been charged" is only
// ever said of a checkout the provider confirmed open, or one it refuses for
// good to read back (a 4xx), whose payment would still come by webhook. It returns how many
// it settled. Run by the ReconcileCheckouts worker.
func (s *Service) ReconcileCheckouts(ctx context.Context) (int, error) {
	settled := 0
	after := uuid.Nil
	for {
		now := s.now()
		rows, err := repository.NewRepository(s.pool).ListOpenCheckoutSessions(ctx, gen.ListOpenCheckoutSessionsParams{
			StartedBefore: now.Add(-checkoutSettle), AfterID: after, MaxRows: jobBatch,
		})
		if err != nil {
			return settled, fmt.Errorf("list open checkouts: %w", err)
		}
		for _, row := range rows {
			after = row.ID
			done, err := s.reconcileCheckout(ctx, row, now)
			if err != nil {
				return settled, err
			}
			if done {
				settled++
			}
		}
		if len(rows) < jobBatch {
			return settled, nil
		}
	}
}

func (s *Service) reconcileCheckout(ctx context.Context, row gen.ListOpenCheckoutSessionsRow, now time.Time) (bool, error) {
	var refused error
	if row.ExternalSessionID != nil {
		ev, err := s.fetchCheckout(ctx, row.Provider, *row.ExternalSessionID)
		switch {
		case errors.Is(err, payment.ErrUnsupported), errors.Is(err, payment.ErrNotFound):
			// Nothing to ask, or nothing was ever opened: expiry decides.
		case payment.Permanent(err):
			// Asking again gets the same refusal: expiry decides, and the
			// refusal is logged once, when the session expires.
			refused = err
		case err != nil:
			s.log.WarnContext(ctx, "billing: couldn't read a checkout back; trying next run",
				"checkout_session_id", row.ID, "error", err)
			return false, nil
		case ev.Type == payment.EventCheckoutCompleted:
			stored, err := s.ingest(ctx, ev)
			if err != nil {
				return false, err
			}
			if stored {
				s.log.WarnContext(ctx, "billing: paid checkout found without its webhook", "event", ev,
					"checkout_session_id", row.ID)
			}
			return stored, nil
		}
	}
	if row.ExpiresAt.After(now) {
		return false, nil
	}
	if refused != nil {
		s.log.ErrorContext(ctx, "billing: payment provider refused to read a checkout back; expiring it",
			"checkout_session_id", row.ID, "error", refused)
	}
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		return s.FailCheckout(ctx, tx, row.WorkspaceID, row.ID, model.CheckoutExpired)
	})
	return err == nil, err
}

func (s *Service) fetchCheckout(ctx context.Context, provider, externalSessionID string) (payment.Event, error) {
	g, ok := s.payments.Get(payment.Provider(provider))
	if !ok {
		return payment.Event{}, payment.ErrUnsupported
	}
	ev, err := g.FetchCheckout(ctx, externalSessionID)
	if err != nil {
		return payment.Event{}, fmt.Errorf("fetch checkout: %w", err)
	}
	ev.Provider = g.Provider()
	return ev, nil
}

// ReconcileSubscriptions reads back each live subscription whose period ended
// more than renewalSettle ago with no news: a lost renewal, past-due or cancel
// webhook. What the provider says is applied as its webhook would be; an
// unchanged state is a replay and costs only the read, and each read of one
// period waits longer than the last (ListOverdueSubscriptions). It returns how
// many brought news. Run hourly by the ReconcileSubscriptions worker.
func (s *Service) ReconcileSubscriptions(ctx context.Context) (int, error) {
	news := 0
	after := uuid.Nil
	repo := repository.NewRepository(s.pool)
	for {
		now := s.now()
		before := now.Add(-renewalSettle)
		rows, err := repo.ListOverdueSubscriptions(ctx, gen.ListOverdueSubscriptionsParams{
			EndedBefore: &before, AfterID: after, Now: now, MaxRows: jobBatch,
		})
		if err != nil {
			return news, fmt.Errorf("list overdue subscriptions: %w", err)
		}
		for _, row := range rows {
			after = row.ID
			g, ok := s.payments.Get(payment.Provider(row.Provider))
			if !ok || row.ExternalSubscriptionID == nil {
				continue
			}
			ev, err := g.FetchSubscription(ctx, *row.ExternalSubscriptionID)
			if errors.Is(err, payment.ErrUnsupported) {
				continue
			}
			// Asked, whatever the answer: the next read of this period backs off.
			if _, merr := repo.MarkSubscriptionReconciled(ctx, gen.MarkSubscriptionReconciledParams{
				ReconciledAt: &now, WorkspaceID: row.WorkspaceID, SubscriptionID: row.ID,
			}); merr != nil {
				return news, fmt.Errorf("mark subscription reconciled: %w", merr)
			}
			if err != nil {
				s.log.WarnContext(ctx, "billing: couldn't read a subscription back; trying next run",
					"subscription_id", row.ID, "error", err)
				continue
			}
			ev.Provider = g.Provider()
			stored, err := s.ingest(ctx, ev)
			if err != nil {
				return news, err
			}
			if stored {
				news++
				s.log.WarnContext(ctx, "billing: subscription changed without its webhook", "event", ev,
					"subscription_id", row.ID)
			}
		}
		if len(rows) < jobBatch {
			return news, nil
		}
	}
}

// RetryRefusedCancels asks the provider again to stop billing for each
// subscription and installment schedule it refused for good to cancel after
// Relay ended it (stopProviderSubscription, stopSchedule). One the provider
// now takes, or no longer has, is cleared; one still refused stays for the
// next run, so it keeps being asked until a person fixes what the provider
// objects to. It returns how many it cleared. Run hourly by the
// ReconcileSubscriptions worker.
func (s *Service) RetryRefusedCancels(ctx context.Context) (int, error) {
	repo := repository.NewRepository(s.pool)
	cleared := 0
	retry := func(op string, id uuid.UUID, provider string, call func(payment.Gateway) error, forget func() (int64, error)) error {
		g, ok := s.payments.Get(payment.Provider(provider))
		if !ok {
			return nil
		}
		if err := call(g); err != nil && !errors.Is(err, payment.ErrNotFound) {
			s.logger().WarnContext(ctx, "billing: payment provider still refuses to "+op+"; trying next run", "id", id, "err", err)
			return nil
		}
		if _, err := forget(); err != nil {
			return fmt.Errorf("clear refused cancel: %w", err)
		}
		cleared++
		s.logger().InfoContext(ctx, "billing: payment provider took a cancel it had refused", "op", op, "id", id)
		return nil
	}

	for after := uuid.Nil; ; {
		rows, err := repo.ListCancelRefusedSubscriptions(ctx, gen.ListCancelRefusedSubscriptionsParams{AfterID: after, MaxRows: jobBatch})
		if err != nil {
			return cleared, fmt.Errorf("list refused subscription cancels: %w", err)
		}
		for _, row := range rows {
			after = row.ID
			if err := retry("cancel a subscription", row.ID, row.Provider, func(g payment.Gateway) error {
				if row.ExternalSubscriptionID == nil {
					return nil
				}
				_, err := g.CancelSubscription(ctx, payment.CancelRequest{ExternalSubscriptionID: *row.ExternalSubscriptionID, Mode: payment.CancelNow})
				return err
			}, func() (int64, error) {
				return repo.ClearSubscriptionCancelRefused(ctx, gen.ClearSubscriptionCancelRefusedParams{WorkspaceID: row.WorkspaceID, SubscriptionID: row.ID})
			}); err != nil {
				return cleared, err
			}
		}
		if len(rows) < jobBatch {
			break
		}
	}
	for after := uuid.Nil; ; {
		rows, err := repo.ListCancelRefusedOrders(ctx, gen.ListCancelRefusedOrdersParams{AfterID: after, MaxRows: jobBatch})
		if err != nil {
			return cleared, fmt.Errorf("list refused schedule cancels: %w", err)
		}
		for _, row := range rows {
			after = row.ID
			if err := retry("cancel an installment schedule", row.ID, row.Provider, func(g payment.Gateway) error {
				if row.ExternalRef == nil {
					return nil
				}
				return g.CancelSchedule(ctx, *row.ExternalRef)
			}, func() (int64, error) {
				return repo.ClearOrderCancelRefused(ctx, gen.ClearOrderCancelRefusedParams{WorkspaceID: row.WorkspaceID, OrderID: row.ID})
			}); err != nil {
				return cleared, err
			}
		}
		if len(rows) < jobBatch {
			return cleared, nil
		}
	}
}
