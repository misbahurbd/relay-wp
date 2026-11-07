package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// unreachable is a provider whose API can't be reached right now.
type unreachable struct{ *payment.Fake }

func (unreachable) FetchCheckout(context.Context, string) (payment.Event, error) {
	return payment.Event{}, payment.ErrUnavailable
}

func (unreachable) FetchSubscription(context.Context, string) (payment.Event, error) {
	return payment.Event{}, payment.ErrUnavailable
}

// runQueued processes every payment event queued since the queue held seen jobs.
func (e *t3Env) runQueued(q *recordingQueue, seen int) int {
	e.t.Helper()
	jobs := q.all()
	for _, j := range jobs[seen:] {
		if args, ok := j.(model.ProcessPaymentEventArgs); ok {
			if err := e.svc.ProcessPaymentEvent(e.ctx, args.PaymentEventID, args.Event); err != nil {
				e.t.Fatalf("process %s: %v", args.Event.Type, err)
			}
		}
	}
	return len(jobs)
}

func (e *t3Env) sessionStatus(id uuid.UUID) string {
	e.t.Helper()
	s, err := e.svc.CheckoutSession(e.ctx, e.ws, id)
	if err != nil {
		e.t.Fatal(err)
	}
	return s.Status
}

func (e *t3Env) invoiceCount() int {
	e.t.Helper()
	var n int
	if err := e.pool.QueryRow(e.ctx, `SELECT count(*) FROM invoices WHERE workspace_id = $1`, e.ws).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// The customer paid but the webhook never came: Relay asks the provider and
// completes the checkout as the webhook would have. The webhook arriving late
// changes nothing.
func TestPaidCheckoutWithoutItsWebhookCompletes(t *testing.T) {
	e := newT3Env(t)
	q := &recordingQueue{}
	e.svc.queue = q
	p := e.plan(t3PlanOpts{Monthly: 1000})
	c := e.checkout(p.Monthly, "")
	late, err := e.fake.CompleteCheckout(*c.Session.ExternalSessionID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := e.svc.ReconcileCheckouts(e.ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.sessionStatus(c.Session.ID); got != model.CheckoutPending {
		t.Fatalf("asked the provider before its webhook had time: session %s", got)
	}

	e.tick(checkoutSettle + time.Minute)
	if _, err := e.svc.ReconcileCheckouts(e.ctx); err != nil {
		t.Fatal(err)
	}
	seen := e.runQueued(q, 0)
	if got := e.sessionStatus(c.Session.ID); got != model.CheckoutCompleted {
		t.Fatalf("session after reconcile = %s", got)
	}
	e.wantCache(e.ws, p.Code)

	if _, err := e.svc.ReconcileCheckouts(e.ctx); err != nil {
		t.Fatal(err)
	}
	e.inTx(func(tx pgx.Tx) error { return e.svc.CompleteCheckout(e.ctx, tx, e.ws, c.Session.ID, late) })
	e.runQueued(q, seen)
	if n := e.invoiceCount(); n != 1 {
		t.Fatalf("%d invoices after the late webhook, want 1", n)
	}
}

// A checkout is only expired once the provider confirms it was never paid.
func TestCheckoutIsNotExpiredWhileTheProviderCantBeAsked(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	c := e.checkout(p.Monthly, "")
	e.tick(2 * time.Hour)

	healthy := e.svc.payments
	down, err := payment.NewRegistry(unreachable{e.fake})
	if err != nil {
		t.Fatal(err)
	}
	e.svc.payments = down
	if _, err := e.svc.ReconcileCheckouts(e.ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.sessionStatus(c.Session.ID); got != model.CheckoutPending {
		t.Fatalf("expired without asking the provider: %s", got)
	}

	e.svc.payments = healthy
	if _, err := e.svc.ReconcileCheckouts(e.ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.sessionStatus(c.Session.ID); got != model.CheckoutExpired {
		t.Fatalf("unpaid checkout past expiry = %s", got)
	}
}

// The provider canceled a subscription but its webhook was lost: once the
// period has ended, Relay asks and ends the plan.
func TestLostCancelWebhookIsReconciled(t *testing.T) {
	e := newT3Env(t)
	q := &recordingQueue{}
	e.svc.queue = q
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	ps, ok := e.fake.Subscription(*sub.ExternalSubscriptionID)
	if !ok {
		t.Fatal("fake has no subscription")
	}
	canceledAt := *sub.CurrentPeriodEnd
	ps.Status, ps.ProviderStatus, ps.CanceledAt = payment.SubscriptionCanceled, "canceled", &canceledAt
	e.fake.SetSubscription(ps)

	if _, err := e.svc.ReconcileSubscriptions(e.ctx); err != nil {
		t.Fatal(err)
	}
	e.runQueued(q, 0)
	e.wantCache(e.ws, p.Code) // mid-period: nothing is overdue, so nothing is asked

	e.clock = sub.CurrentPeriodEnd.Add(renewalSettle + time.Minute)
	if _, err := e.svc.ReconcileSubscriptions(e.ctx); err != nil {
		t.Fatal(err)
	}
	e.runQueued(q, 0)
	e.wantCache(e.ws, model.FreePlan)
	if got, err := e.svc.Subscription(e.ctx, e.ws); err != nil || got.Status != model.SubscriptionCanceled {
		t.Fatalf("subscription after the provider canceled it: %+v, %v", got.Status, err)
	}
}
