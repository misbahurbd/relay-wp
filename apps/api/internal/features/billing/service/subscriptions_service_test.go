package service

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
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
	if q.Kind != model.ChangeInterval || !q.EffectiveAt.Equal(mid) || q.ProrationMinor != 10000-500 {
		t.Fatalf("month to year: %+v", q)
	}

	yend := start.AddDate(1, 0, 0)
	ysub := model.Subscription{Status: model.SubscriptionActive, CurrentPeriodStart: &start, CurrentPeriodEnd: &yend, Price: &yearly}
	q, err = quoteChange(ysub, loadedPrice{Price: monthly, IsActive: true}, mid)
	if err != nil || q.Kind != model.ChangeInterval || !q.EffectiveAt.Equal(yend) || q.ProrationMinor != 0 {
		t.Fatalf("year to month: %+v, %v", q, err)
	}

	sub.Status = model.SubscriptionTrialing
	other := model.Price{ID: uuid.New(), PlanID: uuid.New(), BillingType: model.BillingRecurring, Interval: model.IntervalMonth, Currency: "USD", AmountMinor: 5000}
	if q, _ := quoteChange(sub, loadedPrice{Price: other, IsActive: true}, mid); q.Kind != model.ChangeUpgrade || q.ProrationMinor != 0 {
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
