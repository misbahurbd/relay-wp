package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// midPeriod moves the clock to the middle of sub's period.
func (e *t3Env) midPeriod(sub model.Subscription) {
	e.clock = sub.CurrentPeriodStart.Add(sub.CurrentPeriodEnd.Sub(*sub.CurrentPeriodStart) / 2)
}

type t3Change struct {
	Kind           string
	ToPriceID      uuid.UUID
	EffectiveAt    time.Time
	ProrationMinor int64
}

func (e *t3Env) changes(subID uuid.UUID) []t3Change {
	e.t.Helper()
	rows, err := e.pool.Query(e.ctx, `SELECT kind, to_price_id, effective_at, proration_minor
		FROM subscription_changes WHERE subscription_id = $1 ORDER BY created_at`, subID)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []t3Change
	for rows.Next() {
		var c t3Change
		if err := rows.Scan(&c.Kind, &c.ToPriceID, &c.EffectiveAt, &c.ProrationMinor); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

func TestSubscribeActivatesSubscriptionAndGrant(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	if sub.Status != model.SubscriptionActive || sub.PlanPriceID != p.Monthly || sub.ExternalSubscriptionID == nil {
		t.Fatalf("subscription = %+v", sub)
	}
	e.wantCache(e.ws, p.Code)
	if g := e.grant(e.ws); g == nil || !sameID(g.SubscriptionID, sub.ID) {
		t.Fatalf("grant = %+v", g)
	}
	page, err := e.svc.Invoices(e.ctx, e.ws, "")
	if err != nil || len(page.Invoices) != 1 || page.Invoices[0].TotalMinor != 1000 {
		t.Fatalf("invoices = %+v, %v", page.Invoices, err)
	}
}

func TestUpgradeAppliesNowWithProration(t *testing.T) {
	e := newT3Env(t)
	basic := e.plan(t3PlanOpts{Monthly: 1000})
	pro := e.plan(t3PlanOpts{Monthly: 3000})
	sub := e.subscribe(basic.Monthly)
	e.midPeriod(sub)

	q, err := e.svc.PreviewChange(e.ctx, e.ws, pro.Monthly)
	if err != nil {
		t.Fatal(err)
	}
	// Half the period left: half of the 2000 difference.
	if q.Kind != model.ChangeUpgrade || q.ProrationMinor != 1000 || !q.EffectiveAt.Equal(e.clock) {
		t.Fatalf("quote = %+v", q)
	}

	got, err := e.svc.ChangePlan(e.ctx, e.ws, e.user, pro.Monthly)
	if err != nil {
		t.Fatal(err)
	}
	if got.PlanPriceID != pro.Monthly || got.PendingPlanPriceID != nil {
		t.Fatalf("after upgrade: %+v", got)
	}
	e.wantCache(e.ws, pro.Code)
	if fs, _ := e.fake.Subscription(*sub.ExternalSubscriptionID); fs.ExternalPriceID == "" {
		t.Fatal("provider subscription has no price")
	}
	ch := e.changes(sub.ID)
	if len(ch) != 1 || ch[0].Kind != model.ChangeUpgrade || ch[0].ProrationMinor != 1000 || ch[0].ToPriceID != pro.Monthly {
		t.Fatalf("changes = %+v", ch)
	}
}

// racingChange is a provider whose proration payment webhook is processed
// while ChangeSubscription is still answering, as Stripe's invoice.paid for
// an upgrade can be. It gives the payment a moment to land first.
type racingChange struct {
	*payment.Fake
	pay    func() error
	result chan error
}

func (g racingChange) ChangeSubscription(ctx context.Context, req payment.ChangeRequest) (payment.Subscription, error) {
	out, err := g.Fake.ChangeSubscription(ctx, req)
	go func() { g.result <- g.pay() }()
	select {
	case res := <-g.result:
		g.result <- res
	case <-time.After(300 * time.Millisecond):
	}
	return out, err
}

// The upgrade's proration invoice names the plan it paid for, even when its
// webhook is processed before ChangePlan records the change (G3c).
func TestUpgradeInvoiceNamesTheNewPlanWhenItsWebhookWins(t *testing.T) {
	e := newT3Env(t)
	basic := e.plan(t3PlanOpts{Monthly: 1000})
	pro := e.plan(t3PlanOpts{Monthly: 3000})
	sub := e.subscribe(basic.Monthly)
	e.midPeriod(sub)
	prorationPay := "pay_" + uuid.NewString()
	g := racingChange{Fake: e.fake, result: make(chan error, 1), pay: func() error {
		return database.WithTx(e.ctx, e.pool, func(tx pgx.Tx) error {
			return e.svc.ApplySubscriptionPayment(e.ctx, tx, e.ws, sub.ID, payment.Payment{
				ExternalID: prorationPay, SubtotalMinor: 1000, TotalMinor: 1000, Currency: "USD",
				PeriodStart: &e.clock, PeriodEnd: sub.CurrentPeriodEnd,
			}, true)
		})
	}}
	reg, err := payment.NewRegistry(g)
	if err != nil {
		t.Fatal(err)
	}
	e.svc.payments = reg

	if _, err := e.svc.ChangePlan(e.ctx, e.ws, e.user, pro.Monthly); err != nil {
		t.Fatal(err)
	}
	if err := <-g.result; err != nil {
		t.Fatalf("proration webhook: %v", err)
	}
	var priceID uuid.UUID
	var proration bool
	if err := e.pool.QueryRow(e.ctx, `SELECT plan_price_id, proration FROM invoices WHERE external_payment_id = $1`,
		prorationPay).Scan(&priceID, &proration); err != nil {
		t.Fatal(err)
	}
	if priceID != pro.Monthly || !proration {
		t.Errorf("upgrade invoice: price %s (pro %s, basic %s), proration %v; want pro, prorated",
			priceID, pro.Monthly, basic.Monthly, proration)
	}
	e.wantCache(e.ws, pro.Code)
}

func TestDowngradeWaitsForPeriodEnd(t *testing.T) {
	e := newT3Env(t)
	basic := e.plan(t3PlanOpts{Monthly: 1000})
	pro := e.plan(t3PlanOpts{Monthly: 3000})
	sub := e.subscribe(pro.Monthly)
	e.midPeriod(sub)

	got, err := e.svc.ChangePlan(e.ctx, e.ws, e.user, basic.Monthly)
	if err != nil {
		t.Fatal(err)
	}
	if got.PlanPriceID != pro.Monthly || got.PendingPlanPriceID == nil || *got.PendingPlanPriceID != basic.Monthly ||
		!got.PendingChangeAt.Equal(*sub.CurrentPeriodEnd) {
		t.Fatalf("after downgrade: %+v", got)
	}
	e.wantCache(e.ws, pro.Code)

	// The provider reports the lower price straight away (it bills it from the
	// next period): the customer keeps the plan they paid for until then.
	var lowerExt string
	if err := e.pool.QueryRow(e.ctx, `SELECT external_price_id FROM provider_prices WHERE plan_price_id = $1`,
		basic.Monthly).Scan(&lowerExt); err != nil {
		t.Fatal(err)
	}
	e.inTx(func(tx pgx.Tx) error {
		return e.svc.SyncSubscription(e.ctx, tx, e.ws, sub.ID, payment.Subscription{
			ExternalID: *sub.ExternalSubscriptionID, Status: payment.SubscriptionActive, ExternalPriceID: lowerExt,
			CurrentPeriodStart: sub.CurrentPeriodStart, CurrentPeriodEnd: sub.CurrentPeriodEnd,
		})
	})
	if mid, err := e.svc.Subscription(e.ctx, e.ws); err != nil || mid.PlanPriceID != pro.Monthly || mid.PendingPlanPriceID == nil {
		t.Fatalf("after the provider's update: %+v, %v", mid, err)
	}
	e.wantCache(e.ws, pro.Code)

	ch := e.changes(sub.ID)
	if len(ch) != 1 || ch[0].Kind != model.ChangeDowngrade || ch[0].ProrationMinor != 0 || !ch[0].EffectiveAt.Equal(*sub.CurrentPeriodEnd) {
		t.Fatalf("changes = %+v", ch)
	}

	// The renewal for the next period applies it.
	e.clock = sub.CurrentPeriodEnd.Add(time.Minute)
	start, end := *sub.CurrentPeriodEnd, sub.CurrentPeriodEnd.AddDate(0, 1, 0)
	e.inTx(func(tx pgx.Tx) error {
		return e.svc.ApplySubscriptionPayment(e.ctx, tx, e.ws, sub.ID, payment.Payment{
			ExternalID: "pay_" + uuid.NewString(), SubtotalMinor: 1000, TotalMinor: 1000, Currency: "USD",
			PeriodStart: &start, PeriodEnd: &end,
		}, true)
	})
	after, err := e.svc.Subscription(e.ctx, e.ws)
	if err != nil {
		t.Fatal(err)
	}
	if after.PlanPriceID != basic.Monthly || after.PendingPlanPriceID != nil {
		t.Fatalf("after renewal: %+v", after)
	}
	e.wantCache(e.ws, basic.Code)
}

func TestCancelPendingChange(t *testing.T) {
	e := newT3Env(t)
	basic := e.plan(t3PlanOpts{Monthly: 1000})
	pro := e.plan(t3PlanOpts{Monthly: 3000})
	e.subscribe(pro.Monthly)
	if _, err := e.svc.CancelPendingChange(e.ctx, e.ws, e.user); !errors.Is(err, model.ErrNoPendingChange) {
		t.Fatalf("nothing pending: %v", err)
	}
	if _, err := e.svc.ChangePlan(e.ctx, e.ws, e.user, basic.Monthly); err != nil {
		t.Fatal(err)
	}
	got, err := e.svc.CancelPendingChange(e.ctx, e.ws, e.user)
	if err != nil || got.PendingPlanPriceID != nil || got.PlanPriceID != pro.Monthly {
		t.Fatalf("cancel pending: %+v, %v", got, err)
	}
}

func TestChangePlanRejectsSamePrice(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	e.subscribe(p.Monthly)
	if _, err := e.svc.ChangePlan(e.ctx, e.ws, e.user, p.Monthly); !errors.Is(err, model.ErrSamePrice) {
		t.Fatalf("same price: %v", err)
	}
}

func TestQuoteChangeIntervals(t *testing.T) {
	plan := uuid.New()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	monthly := model.Price{ID: uuid.New(), PlanID: plan, BillingType: model.BillingRecurring, Interval: model.IntervalMonth, Currency: "USD", AmountMinor: 1000}
	yearly := model.Price{ID: uuid.New(), PlanID: plan, BillingType: model.BillingRecurring, Interval: model.IntervalYear, Currency: "USD", AmountMinor: 10000}
	sub := model.Subscription{Status: model.SubscriptionActive, CurrentPeriodStart: &start, CurrentPeriodEnd: &end, Price: &monthly}
	mid := start.Add(end.Sub(start) / 2)

	q, err := quoteChange(sub, loadedPrice{Price: yearly, IsActive: true}, mid)
	if err != nil {
		t.Fatal(err)
	}
	// A new yearly period starts now, less half a month unused.
	if q.Kind != model.ChangeInterval || !q.AppliesNow || !q.EffectiveAt.Equal(mid) || q.ProrationMinor != 10000-500 {
		t.Fatalf("month to year: %+v", q)
	}

	yend := start.AddDate(1, 0, 0)
	ysub := model.Subscription{Status: model.SubscriptionActive, CurrentPeriodStart: &start, CurrentPeriodEnd: &yend, Price: &yearly}
	q, err = quoteChange(ysub, loadedPrice{Price: monthly, IsActive: true}, mid)
	if err != nil || q.Kind != model.ChangeInterval || q.AppliesNow || !q.EffectiveAt.Equal(yend) || q.ProrationMinor != 0 {
		t.Fatalf("year to month: %+v, %v", q, err)
	}

	sub.Status = model.SubscriptionTrialing
	other := model.Price{ID: uuid.New(), PlanID: uuid.New(), BillingType: model.BillingRecurring, Interval: model.IntervalMonth, Currency: "USD", AmountMinor: 5000}
	if q, _ := quoteChange(sub, loadedPrice{Price: other, IsActive: true}, mid); q.Kind != model.ChangeUpgrade || !q.AppliesNow || q.ProrationMinor != 0 {
		t.Fatalf("trial upgrade: %+v", q)
	}
	eur := other
	eur.Currency = "EUR"
	if _, err := quoteChange(sub, loadedPrice{Price: eur, IsActive: true}, mid); !errors.Is(err, model.ErrChangeNotSupported) {
		t.Fatalf("other currency: %v", err)
	}
}

func TestCancelAtPeriodEndAndResume(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	if _, err := e.svc.ResumeSubscription(e.ctx, e.ws, e.user); !errors.Is(err, model.ErrNotCanceling) {
		t.Fatalf("resume before cancel: %v", err)
	}
	got, err := e.svc.CancelSubscription(e.ctx, e.ws, e.user, true)
	if err != nil || !got.CancelAtPeriodEnd || got.Status != model.SubscriptionActive {
		t.Fatalf("cancel at period end: %+v, %v", got, err)
	}
	e.wantCache(e.ws, p.Code)
	if fs, _ := e.fake.Subscription(*sub.ExternalSubscriptionID); !fs.CancelAtPeriodEnd {
		t.Fatal("provider not told")
	}
	got, err = e.svc.ResumeSubscription(e.ctx, e.ws, e.user)
	if err != nil || got.CancelAtPeriodEnd {
		t.Fatalf("resume: %+v, %v", got, err)
	}
}

func TestCancelNowEndsTheGrant(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	e.subscribe(p.Monthly)
	got, err := e.svc.CancelSubscription(e.ctx, e.ws, e.user, false)
	if err != nil || got.Status != model.SubscriptionCanceled {
		t.Fatalf("cancel now: %+v, %v", got, err)
	}
	e.wantCache(e.ws, model.FreePlan)
	// With nothing live, Subscription shows the one that just ended.
	if latest, err := e.svc.Subscription(e.ctx, e.ws); err != nil || latest.Status != model.SubscriptionCanceled {
		t.Fatalf("latest after cancel: %+v, %v", latest, err)
	}
}

func TestCancelForLTDCreditsUnusedTime(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	e.midPeriod(sub)

	preview, err := e.svc.SubscriptionCredit(e.ctx, e.ws)
	if err != nil || preview.CreditMinor != 500 {
		t.Fatalf("credit preview = %+v, %v", preview, err)
	}
	var credit model.SubscriptionCredit
	e.inTx(func(tx pgx.Tx) error {
		var err error
		credit, err = e.svc.CancelForLTD(e.ctx, tx, e.ws)
		return err
	})
	if credit.CreditMinor != 500 || credit.Currency != "USD" || !sameID(credit.SubscriptionID, sub.ID) {
		t.Fatalf("credit = %+v", credit)
	}
	e.wantCache(e.ws, model.FreePlan)
	if fs, _ := e.fake.Subscription(*sub.ExternalSubscriptionID); fs.Status != payment.SubscriptionCanceled {
		t.Fatalf("provider status = %s", fs.Status)
	}

	// Nothing live: nothing to credit.
	e.inTx(func(tx pgx.Tx) error {
		c, err := e.svc.CancelForLTD(e.ctx, tx, e.ws)
		if c.SubscriptionID != nil || c.CreditMinor != 0 {
			t.Errorf("second credit = %+v", c)
		}
		return err
	})
}

func TestSyncSubscriptionPastDueSuspendsAfterGrace(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	ps := payment.Subscription{ExternalID: *sub.ExternalSubscriptionID, Status: payment.SubscriptionPastDue}
	sync := func() {
		e.inTx(func(tx pgx.Tx) error { return e.svc.SyncSubscription(e.ctx, tx, e.ws, sub.ID, ps) })
	}

	sync()
	e.wantCache(e.ws, p.Code) // still within grace
	e.tick(pastDueGrace + time.Hour)
	sync()
	e.wantCache(e.ws, model.FreePlan)
	if g := e.grant(e.ws); g == nil || g.Status != model.GrantSuspended {
		t.Fatalf("grant = %+v", g)
	}

	ps.Status = payment.SubscriptionActive
	sync()
	e.wantCache(e.ws, p.Code)

	ps.Status = payment.SubscriptionCanceled
	sync()
	e.wantCache(e.ws, model.FreePlan)
}

// Providers send nothing when grace runs out, so the sweep must suspend the plan.
func TestPastDueSweepSuspendsAfterGrace(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	ps := payment.Subscription{ExternalID: *sub.ExternalSubscriptionID, Status: payment.SubscriptionPastDue}
	e.inTx(func(tx pgx.Tx) error { return e.svc.SyncSubscription(e.ctx, tx, e.ws, sub.ID, ps) })

	if _, err := e.svc.SuspendPastDue(e.ctx); err != nil {
		t.Fatal(err)
	}
	e.wantCache(e.ws, p.Code) // still within grace

	e.tick(pastDueGrace + time.Hour)
	if _, err := e.svc.SuspendPastDue(e.ctx); err != nil {
		t.Fatal(err)
	}
	e.wantCache(e.ws, model.FreePlan)
	if g := e.grant(e.ws); g == nil || !g.SuspendedFor(model.SuspendPastDue) {
		t.Fatalf("grant = %+v", g)
	}

	// Paying again restores it.
	ps.Status = payment.SubscriptionActive
	e.inTx(func(tx pgx.Tx) error { return e.svc.SyncSubscription(e.ctx, tx, e.ws, sub.ID, ps) })
	e.wantCache(e.ws, p.Code)
}

// A renewal paid after grace ends brings the plan back for good: the sweep
// must not suspend it again, and the paid period becomes the current one.
func TestRenewalPaidAfterGraceStaysActive(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	ps := payment.Subscription{ExternalID: *sub.ExternalSubscriptionID, Status: payment.SubscriptionPastDue}
	e.inTx(func(tx pgx.Tx) error { return e.svc.SyncSubscription(e.ctx, tx, e.ws, sub.ID, ps) })
	e.tick(pastDueGrace + time.Hour)
	if _, err := e.svc.SuspendPastDue(e.ctx); err != nil {
		t.Fatal(err)
	}
	e.wantCache(e.ws, model.FreePlan)

	start, end := *sub.CurrentPeriodEnd, sub.CurrentPeriodEnd.AddDate(0, 1, 0)
	e.inTx(func(tx pgx.Tx) error {
		return e.svc.ApplySubscriptionPayment(e.ctx, tx, e.ws, sub.ID, payment.Payment{
			ExternalID: "pay_" + uuid.NewString(), SubtotalMinor: 1000, TotalMinor: 1000, Currency: "USD",
			PeriodStart: &start, PeriodEnd: &end,
		}, true)
	})
	e.wantCache(e.ws, p.Code)

	if n, err := e.svc.SuspendPastDue(e.ctx); err != nil || n != 0 {
		t.Fatalf("sweep after payment suspended %d, %v", n, err)
	}
	e.wantCache(e.ws, p.Code)
	got, err := e.svc.Subscription(e.ctx, e.ws)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.SubscriptionActive || got.GraceEndsAt != nil ||
		got.CurrentPeriodEnd == nil || !got.CurrentPeriodEnd.Equal(end) {
		t.Fatalf("after payment: status %s, grace %v, period end %v", got.Status, got.GraceEndsAt, got.CurrentPeriodEnd)
	}
}

// A renewal payment that carries the subscription (Creem's subscription.paid)
// syncs it too, even when the payment itself is already recorded.
func TestRenewalPaymentSyncsTheSubscriptionItCarries(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	start, end := *sub.CurrentPeriodEnd, sub.CurrentPeriodEnd.AddDate(0, 1, 0)
	pay := payment.Payment{
		ExternalID: "pay_" + uuid.NewString(), ExternalSubscriptionID: *sub.ExternalSubscriptionID,
		SubtotalMinor: 1000, TotalMinor: 1000, Currency: "USD", PeriodStart: &start, PeriodEnd: &end,
	}
	ps := payment.Subscription{
		ExternalID: *sub.ExternalSubscriptionID, Status: payment.SubscriptionActive,
		CurrentPeriodStart: &start, CurrentPeriodEnd: &end,
	}
	ev := func() payment.Event {
		return payment.Event{Provider: payment.Stripe, Type: payment.EventPaymentSucceeded, Payment: &pay, Subscription: &ps}
	}
	e.inTx(func(tx pgx.Tx) error { return e.svc.onPayment(true)(e.ctx, tx, e.ws, ev()) })

	ps.CancelAtPeriodEnd = true
	e.inTx(func(tx pgx.Tx) error { return e.svc.onPayment(true)(e.ctx, tx, e.ws, ev()) })
	got, err := e.svc.Subscription(e.ctx, e.ws)
	if err != nil {
		t.Fatal(err)
	}
	if !got.CancelAtPeriodEnd || got.CurrentPeriodEnd == nil || !got.CurrentPeriodEnd.Equal(end) {
		t.Fatalf("after payment events: cancel %v, period end %v", got.CancelAtPeriodEnd, got.CurrentPeriodEnd)
	}
}

// A lost dispute suspends the plan until support restores it, renewals or not.
func TestLostDisputeSuspensionSurvivesRenewal(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	g := e.grant(e.ws)
	e.inTx(func(tx pgx.Tx) error { return e.svc.SuspendGrant(e.ctx, tx, e.ws, g.ID, model.SuspendDispute) })

	ps := payment.Subscription{ExternalID: *sub.ExternalSubscriptionID, Status: payment.SubscriptionActive}
	e.inTx(func(tx pgx.Tx) error { return e.svc.SyncSubscription(e.ctx, tx, e.ws, sub.ID, ps) })
	e.wantCache(e.ws, model.FreePlan)
	if got := e.grant(e.ws); got == nil || got.ID != g.ID || !got.SuspendedFor(model.SuspendDispute) {
		t.Fatalf("grant after renewal = %+v", got)
	}
}

func TestRefundingTheLatestSubscriptionInvoiceCancelsIt(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	var pay string
	var total int64
	if err := e.pool.QueryRow(e.ctx, `SELECT external_payment_id, total_minor FROM invoices
		WHERE workspace_id = $1 AND subscription_id = $2`, e.ws, sub.ID).Scan(&pay, &total); err != nil {
		t.Fatal(err)
	}

	// A partial refund keeps the plan.
	refund := func(amount int64) {
		ev := refundEvent(pay, "re_"+uuid.NewString(), amount, payment.RefundSucceeded)
		ev.Provider = payment.Stripe
		e.inTx(func(tx pgx.Tx) error { return e.svc.onRefund(e.ctx, tx, e.ws, ev) })
	}
	refund(total / 2)
	e.wantCache(e.ws, p.Code)

	refund(total - total/2)
	e.wantCache(e.ws, model.FreePlan)
	if got, err := e.svc.Subscription(e.ctx, e.ws); err != nil || got.Status != model.SubscriptionCanceled {
		t.Fatalf("subscription after a full refund = %s, %v; want canceled", got.Status, err)
	}
	if fs, _ := e.fake.Subscription(*sub.ExternalSubscriptionID); fs.Status != payment.SubscriptionCanceled {
		t.Fatalf("provider status = %s, want canceled", fs.Status)
	}
}

// The provider key for a plan change stays the same until a change is logged,
// so a retry after a failed database write can't change or charge twice.
func TestChangeKeyRepeatsUntilAChangeIsLogged(t *testing.T) {
	e := newT3Env(t)
	small := e.plan(t3PlanOpts{Monthly: 1000})
	big := e.plan(t3PlanOpts{Monthly: 3000})
	sub := e.subscribe(small.Monthly)
	r := repository.NewRepository(e.pool)

	k1, err := changeKey(e.ctx, r, sub, "change", big.Monthly)
	if err != nil {
		t.Fatal(err)
	}
	k2, _ := changeKey(e.ctx, r, sub, "change", big.Monthly)
	if k1 != k2 {
		t.Fatalf("keys differ on retry: %q vs %q", k1, k2)
	}
	if _, err := e.svc.ChangePlan(e.ctx, e.ws, e.user, big.Monthly); err != nil {
		t.Fatal(err)
	}
	k3, _ := changeKey(e.ctx, r, sub, "change", big.Monthly)
	if k3 == k1 {
		t.Fatal("key unchanged after a logged change")
	}
}

// Creem marks no payment as a proration. Relay tells one by its period, which
// starts inside a period already invoiced, so refunding a Creem plan change
// in full keeps the plan, and refunding the period's invoice still ends it.
func TestRefundingAnUnmarkedProrationKeepsThePlan(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	var periodPay string
	if err := e.pool.QueryRow(e.ctx, `SELECT external_payment_id FROM invoices WHERE subscription_id = $1`,
		sub.ID).Scan(&periodPay); err != nil {
		t.Fatal(err)
	}
	e.tick(time.Hour)
	prorationPay := "pay_" + uuid.NewString()
	e.inTx(func(tx pgx.Tx) error {
		return e.svc.ApplySubscriptionPayment(e.ctx, tx, e.ws, sub.ID, payment.Payment{
			ExternalID: prorationPay, SubtotalMinor: 500, TotalMinor: 500, Currency: "USD",
			PeriodStart: &e.clock, PeriodEnd: sub.CurrentPeriodEnd,
		}, true)
	})
	refund := func(pay string, amount int64) {
		ev := refundEvent(pay, "re_"+uuid.NewString(), amount, payment.RefundSucceeded)
		ev.Provider = payment.Stripe
		e.inTx(func(tx pgx.Tx) error { return e.svc.onRefund(e.ctx, tx, e.ws, ev) })
	}

	refund(prorationPay, 500)
	e.wantCache(e.ws, p.Code)
	refund(periodPay, 1000)
	e.wantCache(e.ws, model.FreePlan)
}

// A failed renewal records an invoice nobody paid. Refunding the period the
// customer did pay still ends the subscription: the failed invoice isn't
// the latest period invoice (G3b).
func TestRefundingThePaidPeriodIgnoresAFailedRenewal(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	var periodPay string
	if err := e.pool.QueryRow(e.ctx, `SELECT external_payment_id FROM invoices WHERE subscription_id = $1`,
		sub.ID).Scan(&periodPay); err != nil {
		t.Fatal(err)
	}
	next := sub.CurrentPeriodEnd.AddDate(0, 1, 0)
	e.inTx(func(tx pgx.Tx) error {
		return e.svc.ApplySubscriptionPayment(e.ctx, tx, e.ws, sub.ID, payment.Payment{
			ExternalID: "pay_failed_" + uuid.NewString(), SubtotalMinor: 1000, TotalMinor: 1000, Currency: "USD",
			PeriodStart: sub.CurrentPeriodEnd, PeriodEnd: &next,
		}, false)
	})

	ev := refundEvent(periodPay, "re_"+uuid.NewString(), 1000, payment.RefundSucceeded)
	ev.Provider = payment.Stripe
	e.inTx(func(tx pgx.Tx) error { return e.svc.onRefund(e.ctx, tx, e.ws, ev) })
	e.wantCache(e.ws, model.FreePlan)
	if got, err := e.svc.Subscription(e.ctx, e.ws); err != nil || got.Status != model.SubscriptionCanceled {
		t.Fatalf("subscription after refunding its paid period = %s, %v; want canceled", got.Status, err)
	}
}

// A lost dispute on a subscription invoice stops the provider billing it:
// the subscription is canceled now, and the plan stays suspended for support.
func TestLostDisputeCancelsTheSubscription(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	var pay string
	if err := e.pool.QueryRow(e.ctx, `SELECT external_payment_id FROM invoices WHERE subscription_id = $1`,
		sub.ID).Scan(&pay); err != nil {
		t.Fatal(err)
	}
	dispute := func(typ payment.EventType, outcome string) payment.Event {
		return payment.Event{Provider: payment.Stripe, Type: typ, Payment: &payment.Payment{ExternalID: pay, Status: outcome}}
	}
	e.inTx(func(tx pgx.Tx) error {
		return e.svc.onDisputeOpened(e.ctx, tx, e.ws, dispute(payment.EventDisputeOpened, ""))
	})
	e.inTx(func(tx pgx.Tx) error {
		return e.svc.onDisputeClosed(e.ctx, tx, e.ws, dispute(payment.EventDisputeClosed, "lost"))
	})

	if fs, _ := e.fake.Subscription(*sub.ExternalSubscriptionID); fs.Status != payment.SubscriptionCanceled {
		t.Errorf("provider subscription %s after a lost dispute, want canceled", fs.Status)
	}
	if got, err := e.svc.Subscription(e.ctx, e.ws); err != nil || got.Status != model.SubscriptionCanceled {
		t.Errorf("subscription %s, %v after a lost dispute, want canceled", got.Status, err)
	}
	if g := e.grant(e.ws); g == nil || g.Status != model.GrantSuspended || !g.SuspendedFor(model.SuspendDispute) {
		t.Errorf("grant = %+v, want suspended for the dispute", g)
	}
}

// A past-due period was never paid, so a lifetime deal gets no credit for it.
func TestPastDueSubscriptionHasNoCredit(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	e.midPeriod(sub)
	e.pastDue(sub)
	if c, err := e.svc.SubscriptionCredit(e.ctx, e.ws); err != nil || c.CreditMinor != 0 {
		t.Fatalf("credit for a past-due subscription = %+v, %v; want 0", c, err)
	}
}

// renew pays sub's next period, as the provider's renewal webhook would.
func (e *t3Env) renew(sub model.Subscription, start, end time.Time) {
	e.t.Helper()
	e.inTx(func(tx pgx.Tx) error {
		return e.svc.ApplySubscriptionPayment(e.ctx, tx, e.ws, sub.ID, payment.Payment{
			ExternalID: "pay_" + uuid.NewString(), SubtotalMinor: 1000, TotalMinor: 1000, Currency: "USD",
			PeriodStart: &start, PeriodEnd: &end,
		}, true)
	})
}

func (e *t3Env) pastDue(sub model.Subscription) {
	e.t.Helper()
	ps := payment.Subscription{ExternalID: *sub.ExternalSubscriptionID, Status: payment.SubscriptionPastDue}
	e.inTx(func(tx pgx.Tx) error { return e.svc.SyncSubscription(e.ctx, tx, e.ws, sub.ID, ps) })
}

// A renewal paid while past due makes the subscription active with its new
// period, so the grace sweep never suspends a customer who paid.
func TestPaidRenewalClearsPastDueAndSavesThePeriod(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	e.clock = sub.CurrentPeriodEnd.Add(time.Hour)
	e.pastDue(sub)

	e.tick(24 * time.Hour)
	start, end := *sub.CurrentPeriodEnd, sub.CurrentPeriodEnd.AddDate(0, 1, 0)
	e.renew(sub, start, end)
	got, err := e.svc.Subscription(e.ctx, e.ws)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.SubscriptionActive || got.GraceEndsAt != nil {
		t.Errorf("after a paid renewal: status %s, grace ends %v; want active with no grace", got.Status, got.GraceEndsAt)
	}
	if got.CurrentPeriodStart == nil || !got.CurrentPeriodStart.Equal(start) || got.CurrentPeriodEnd == nil ||
		!got.CurrentPeriodEnd.Equal(end) {
		t.Errorf("period after renewal = %v to %v, want %v to %v", got.CurrentPeriodStart, got.CurrentPeriodEnd, start, end)
	}

	e.tick(pastDueGrace + time.Hour)
	if _, err := e.svc.SuspendPastDue(e.ctx); err != nil { // the sweep covers every workspace in the shared database
		t.Fatal(err)
	}
	e.wantCache(e.ws, p.Code)
}

// A renewal paid after grace ran out restores the suspended plan and the status.
func TestPaidRenewalAfterGraceRestoresThePlan(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	e.clock = sub.CurrentPeriodEnd.Add(time.Hour)
	e.pastDue(sub)
	e.tick(pastDueGrace + time.Hour)
	if _, err := e.svc.SuspendPastDue(e.ctx); err != nil {
		t.Fatal(err)
	}
	e.wantCache(e.ws, model.FreePlan)

	e.renew(sub, *sub.CurrentPeriodEnd, sub.CurrentPeriodEnd.AddDate(0, 1, 0))
	e.wantCache(e.ws, p.Code)
	if got, err := e.svc.Subscription(e.ctx, e.ws); err != nil || got.Status != model.SubscriptionActive {
		t.Fatalf("after paying: %s, %v; want active", got.Status, err)
	}
}
