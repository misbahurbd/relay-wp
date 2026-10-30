package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

func t3IsInvalidInput(err error) bool {
	e, ok := apperr.As(err)
	return ok && e.Code == "invalid_input"
}

// discount creates a code; edit tweaks the defaults (20% off, once, anything).
func (e *t3Env) discount(edit func(*model.NewDiscount)) model.Discount {
	e.t.Helper()
	in := model.NewDiscount{
		Code: "T3" + uuid.NewString()[:8], Kind: model.DiscountPercent, PercentBps: ptrTo(2000),
		AllowInstallments: true, StartsAt: e.clock.Add(-time.Hour),
	}
	if edit != nil {
		edit(&in)
	}
	d, err := e.svc.CreateDiscount(e.ctx, in)
	if err != nil {
		e.t.Fatalf("create discount: %v", err)
	}
	return d
}

func (e *t3Env) redemption(sessionID uuid.UUID) (status string, count int) {
	e.t.Helper()
	if err := e.pool.QueryRow(e.ctx, `SELECT r.status, d.redeemed_count FROM discount_redemptions r
		JOIN discounts d ON d.id = r.discount_id WHERE r.checkout_session_id = $1`, sessionID).Scan(&status, &count); err != nil {
		e.t.Fatal(err)
	}
	return status, count
}

func TestApplyDiscountRejections(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	other := e.plan(t3PlanOpts{Monthly: 1000})
	monthly, err := e.svc.Price(e.ctx, p.Monthly)
	if err != nil {
		t.Fatal(err)
	}
	lifetime := monthly
	lifetime.BillingType, lifetime.Interval = model.BillingOneTime, ""
	offer := &model.InstallmentOffer{ID: uuid.New(), PlanPriceID: lifetime.ID, Count: 3, TotalMinor: 30000}

	inactive := e.discount(nil)
	if err := e.svc.DeactivateDiscount(e.ctx, inactive.Code); err != nil {
		t.Fatal(err)
	}
	usedUp := e.discount(func(d *model.NewDiscount) { d.MaxRedemptions = ptrTo(1) })
	e.exec(`UPDATE discounts SET redeemed_count = 1 WHERE id = $1`, usedUp.ID)
	// This workspace already used it once (per_workspace_limit 1).
	perWorkspace := e.discount(nil)
	c := e.checkout(p.Monthly, perWorkspace.Code)
	e.inTx(func(tx pgx.Tx) error { return e.svc.FailCheckout(e.ctx, tx, e.ws, c.Session.ID, model.CheckoutFailed) })
	e.exec(`UPDATE discount_redemptions SET status = 'redeemed', redeemed_at = now() WHERE checkout_session_id = $1`, c.Session.ID)

	cases := []struct {
		name  string
		code  string
		price model.Price
		offer *model.InstallmentOffer
		want  error
	}{
		{"unknown", "NO-SUCH-CODE", monthly, nil, model.ErrDiscountNotFound},
		{"inactive", inactive.Code, monthly, nil, model.ErrDiscountInactive},
		{"not started", e.discount(func(d *model.NewDiscount) { d.StartsAt = e.clock.Add(time.Hour) }).Code, monthly, nil, model.ErrDiscountNotStarted},
		{"expired", e.discount(func(d *model.NewDiscount) { d.ExpiresAt = ptrTo(e.clock.Add(-time.Minute)) }).Code, monthly, nil, model.ErrDiscountExpired},
		{"used up", usedUp.Code, monthly, nil, model.ErrDiscountUsedUp},
		{"workspace limit", perWorkspace.Code, monthly, nil, model.ErrDiscountWorkspaceLimit},
		{"wrong plan", e.discount(func(d *model.NewDiscount) { d.PlanID = &other.ID }).Code, monthly, nil, model.ErrDiscountWrongPlan},
		{"one-time code on a subscription", e.discount(func(d *model.NewDiscount) { d.AppliesTo = model.AppliesToOneTime }).Code, monthly, nil, model.ErrDiscountWrongPrice},
		{"recurring code on a lifetime deal", e.discount(func(d *model.NewDiscount) { d.AppliesTo = model.AppliesToRecurring }).Code, lifetime, nil, model.ErrDiscountWrongPrice},
		{"no installments", e.discount(func(d *model.NewDiscount) { d.AllowInstallments = false }).Code, lifetime, offer, model.ErrDiscountNoInstallments},
		{"other currency", e.discount(func(d *model.NewDiscount) {
			d.Kind, d.PercentBps, d.AmountOffMinor, d.Currency = model.DiscountAmount, nil, ptrTo(int64(500)), ptrTo("EUR")
		}).Code, monthly, nil, model.ErrDiscountCurrency},
	}
	for _, tc := range cases {
		if _, err := e.svc.ApplyDiscount(e.ctx, e.ws, tc.code, tc.price, tc.offer); !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestApplyDiscountAmounts(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	price, _ := e.svc.Price(e.ctx, p.Monthly)

	pct := e.discount(func(d *model.NewDiscount) {
		d.Code = "  t3lower" + uuid.NewString()[:6] // codes are case-insensitive
		d.Duration, d.DurationMonths = model.DurationRepeating, ptrTo(3)
	})
	got, err := e.svc.ApplyDiscount(e.ctx, e.ws, strings.ToLower(pct.Code), price, nil)
	if err != nil || got.DiscountMinor != 200 || got.EndsAt == nil || !got.EndsAt.Equal(e.clock.AddDate(0, 3, 0)) {
		t.Fatalf("20%% repeating: %+v, %v", got, err)
	}
	big := e.discount(func(d *model.NewDiscount) {
		d.Kind, d.PercentBps, d.AmountOffMinor, d.Currency = model.DiscountAmount, nil, ptrTo(int64(5000)), ptrTo("usd")
	})
	if got, err := e.svc.ApplyDiscount(e.ctx, e.ws, big.Code, price, nil); err != nil || got.DiscountMinor != 1000 {
		t.Fatalf("amount over the price: %+v, %v", got, err)
	}

	preview, err := e.svc.PreviewDiscount(e.ctx, e.ws, model.DiscountPreviewInput{Code: pct.Code, PlanPriceID: p.Monthly})
	if err != nil || preview.SubtotalMinor != 1000 || preview.TotalMinor != 800 || preview.Duration != model.DurationRepeating {
		t.Fatalf("preview = %+v, %v", preview, err)
	}
}

func TestCheckoutDiscountIsRedeemedOnPayment(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	d := e.discount(func(d *model.NewDiscount) { d.Duration, d.DurationMonths = model.DurationRepeating, ptrTo(3) })

	c := e.checkout(p.Monthly, d.Code)
	if c.Session.AmountMinor != 800 || c.Session.DiscountMinor != 200 {
		t.Fatalf("session = %+v", c.Session)
	}
	if status, count := e.redemption(c.Session.ID); status != model.RedemptionReserved || count != 1 {
		t.Fatalf("after checkout: %s, %d", status, count)
	}
	if fc, _ := e.fake.Checkout(*c.Session.ExternalSessionID); fc.Request.ExternalCouponID == "" {
		t.Fatal("subscription checkout carries no provider coupon")
	}

	e.pay(c)
	if status, count := e.redemption(c.Session.ID); status != model.RedemptionRedeemed || count != 1 {
		t.Fatalf("after payment: %s, %d", status, count)
	}
	sub, err := e.svc.Subscription(e.ctx, e.ws)
	if err != nil {
		t.Fatal(err)
	}
	if !sameID(sub.DiscountID, d.ID) || sub.DiscountEndsAt == nil || !sub.DiscountEndsAt.Equal(sub.CurrentPeriodStart.AddDate(0, 3, 0)) {
		t.Fatalf("subscription discount = %v until %v", sub.DiscountID, sub.DiscountEndsAt)
	}

	// The coupon is mirrored once per provider, then reused by the next workspace.
	coupons := len(e.fake.Coupons())
	e.ws = e.workspace()
	if _, err := e.svc.UpdateProfile(e.ctx, e.ws, model.ProfileInput{LegalName: "Second", Email: "second@acme.test"}); err != nil {
		t.Fatal(err)
	}
	e.checkout(p.Monthly, d.Code)
	if n := len(e.fake.Coupons()); n != coupons {
		t.Fatalf("provider coupons went from %d to %d, want reuse", coupons, n)
	}
}

func TestExpiredCheckoutReleasesDiscount(t *testing.T) {
	e := newT3Env(t)
	e.svc.cfg.CheckoutTTL = time.Second
	p := e.plan(t3PlanOpts{Monthly: 1000})
	d := e.discount(nil)
	c := e.checkout(p.Monthly, d.Code)

	// The session stays open as long as the provider's checkout (the fake's: an hour).
	if n, _ := e.svc.ExpireCheckouts(e.ctx); n != 0 {
		t.Fatalf("expired %d before the provider's checkout did", n)
	}
	e.tick(2 * time.Hour)
	if n, err := e.svc.ExpireCheckouts(e.ctx); err != nil || n < 1 {
		t.Fatalf("ExpireCheckouts = %d, %v", n, err)
	}
	if status, count := e.redemption(c.Session.ID); status != model.RedemptionReleased || count != 0 {
		t.Fatalf("after expiry: %s, %d", status, count)
	}
	s, err := e.svc.CheckoutSession(e.ctx, e.ws, c.Session.ID)
	if err != nil || s.Status != model.CheckoutExpired {
		t.Fatalf("session = %+v, %v", s, err)
	}
	// The released use counts no more: the workspace can use the code again.
	price, _ := e.svc.Price(e.ctx, p.Monthly)
	if _, err := e.svc.ApplyDiscount(e.ctx, e.ws, d.Code, price, nil); err != nil {
		t.Fatalf("code after release: %v", err)
	}
}

func TestLatePaymentCompletesAnExpiredCheckout(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	d := e.discount(nil)
	c := e.checkout(p.Monthly, d.Code)
	e.tick(2 * time.Hour)
	if n, err := e.svc.ExpireCheckouts(e.ctx); err != nil || n < 1 {
		t.Fatalf("ExpireCheckouts = %d, %v", n, err)
	}

	// The customer pays from the tab they left open: the money is real.
	e.pay(c)
	s, err := e.svc.CheckoutSession(e.ctx, e.ws, c.Session.ID)
	if err != nil || s.Status != model.CheckoutCompleted {
		t.Fatalf("session = %+v, %v", s, err)
	}
	if status, count := e.redemption(c.Session.ID); status != model.RedemptionRedeemed || count != 1 {
		t.Fatalf("discount after a late payment: %s, %d", status, count)
	}
	e.wantCache(e.ws, p.Code)
}

func TestCreateDiscountValidates(t *testing.T) {
	e := newT3Env(t)
	for name, in := range map[string]model.NewDiscount{
		"short code":       {Code: "AB", Kind: model.DiscountPercent, PercentBps: ptrTo(1000)},
		"no percent":       {Code: "T3NOPCT", Kind: model.DiscountPercent},
		"no currency":      {Code: "T3NOCUR", Kind: model.DiscountAmount, AmountOffMinor: ptrTo(int64(100))},
		"repeating no end": {Code: "T3NOMONTHS", Kind: model.DiscountPercent, PercentBps: ptrTo(1000), Duration: model.DurationRepeating},
		"bad kind":         {Code: "T3BADKIND", Kind: "free"},
	} {
		if _, err := e.svc.CreateDiscount(e.ctx, in); !t3IsInvalidInput(err) {
			t.Errorf("%s: %v", name, err)
		}
	}
	d := e.discount(nil)
	if _, err := e.svc.CreateDiscount(e.ctx, model.NewDiscount{Code: d.Code, Kind: model.DiscountPercent, PercentBps: ptrTo(100)}); !errors.Is(err, model.ErrDiscountCodeTaken) {
		t.Fatalf("duplicate code: %v", err)
	}
	if err := e.svc.DeactivateDiscount(e.ctx, "T3-NO-SUCH"); !errors.Is(err, model.ErrDiscountNotFound) {
		t.Fatalf("deactivate unknown: %v", err)
	}
	active, err := e.svc.ListDiscounts(e.ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, x := range active {
		found = found || x.ID == d.ID
	}
	if !found {
		t.Fatal("new code not listed")
	}
}
