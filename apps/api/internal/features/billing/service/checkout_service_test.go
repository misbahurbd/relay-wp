package service

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

func TestCreateCheckoutIsIdempotent(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	in := model.CheckoutInput{PlanPriceID: p.Monthly, IdempotencyKey: "t3-key-" + uuid.NewString()}

	first, err := e.svc.CreateCheckout(e.ctx, e.ws, e.user, in)
	if err != nil {
		t.Fatal(err)
	}
	if first.ClientSecret == "" || first.PublishableKey == "" || first.Session.Purpose != model.PurposeNew ||
		first.Session.SubscriptionID == nil || first.Session.AmountMinor != 1000 {
		t.Fatalf("first = %+v", first)
	}
	again, err := e.svc.CreateCheckout(e.ctx, e.ws, e.user, in)
	if err != nil || again.Session.ID != first.Session.ID {
		t.Fatalf("repeat = %+v, %v", again.Session, err)
	}

	in.PlanPriceID = p.Yearly
	if _, err := e.svc.CreateCheckout(e.ctx, e.ws, e.user, in); !errors.Is(err, model.ErrIdempotencyKeyReused) {
		t.Fatalf("same key, other price: %v", err)
	}
	if _, err := e.svc.CreateCheckout(e.ctx, e.ws, e.user, model.CheckoutInput{PlanPriceID: p.Monthly}); !t3IsInvalidInput(err) {
		t.Fatalf("no key: %v", err)
	}
}

func TestCreateCheckoutRefusals(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	try := func(in model.CheckoutInput) error {
		in.IdempotencyKey = uuid.NewString()
		_, err := e.svc.CreateCheckout(e.ctx, e.ws, e.user, in)
		return err
	}
	if err := try(model.CheckoutInput{PlanPriceID: uuid.New()}); !errors.Is(err, model.ErrPriceNotFound) {
		t.Errorf("unknown price: %v", err)
	}
	if err := try(model.CheckoutInput{PlanPriceID: p.Monthly, Provider: payment.Creem}); !errors.Is(err, model.ErrProviderNotAvailable) {
		t.Errorf("disabled provider: %v", err)
	}
	unsold := e.plan(t3PlanOpts{Monthly: 1000})
	e.exec(`UPDATE provider_prices SET is_active = false WHERE plan_price_id = $1`, unsold.Monthly)
	if err := try(model.CheckoutInput{PlanPriceID: unsold.Monthly}); !errors.Is(err, model.ErrNoProviderForPrice) {
		t.Errorf("no provider price: %v", err)
	}

	noProfile := e.workspace()
	if _, err := e.svc.CreateCheckout(e.ctx, noProfile, e.user, model.CheckoutInput{
		PlanPriceID: p.Monthly, IdempotencyKey: uuid.NewString(),
	}); !errors.Is(err, model.ErrProfileNotFound) {
		t.Errorf("no billing profile: %v", err)
	}

	e.subscribe(p.Monthly)
	if err := try(model.CheckoutInput{PlanPriceID: p.Yearly}); !errors.Is(err, model.ErrAlreadySubscribed) {
		t.Errorf("second subscription: %v", err)
	}
}

func TestCardTrialStartsTrialing(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000, TrialDays: 14, RequiresCard: true})
	c := e.checkout(p.Monthly, "")
	if c.Session.AmountMinor != 0 {
		t.Fatalf("charged now = %d, want 0 during a trial", c.Session.AmountMinor)
	}
	if fc, _ := e.fake.Checkout(*c.Session.ExternalSessionID); fc.Request.TrialDays != 14 {
		t.Fatalf("trial days sent = %d", fc.Request.TrialDays)
	}
	// The provider reports the subscription trialing before any card is in:
	// that grants nothing. Only the completed checkout does.
	now, end := e.clock, e.clock.AddDate(0, 0, 14)
	e.inTx(func(tx pgx.Tx) error {
		return e.svc.SyncSubscription(e.ctx, tx, e.ws, *c.Session.SubscriptionID, payment.Subscription{
			Status: payment.SubscriptionTrialing, CurrentPeriodStart: &now, CurrentPeriodEnd: &end,
		})
	})
	e.wantCache(e.ws, model.FreePlan)
	e.pay(c)
	sub, err := e.svc.Subscription(e.ctx, e.ws)
	if err != nil || sub.Status != model.SubscriptionTrialing {
		t.Fatalf("subscription = %+v, %v", sub, err)
	}
	e.wantCache(e.ws, p.Code)
}

// A provider that keeps trials on the product (Creem) would trial every
// checkout, so a card-required trial isn't offered there: the first period is charged.
func TestCardTrialNeedsCheckoutTrials(t *testing.T) {
	e := newT3Env(t)
	caps := e.fake.Capabilities()
	t.Cleanup(func() { e.fake.SetCapabilities(caps) })
	noTrials := caps
	noTrials.CheckoutTrials = false
	e.fake.SetCapabilities(noTrials)

	p := e.plan(t3PlanOpts{Monthly: 1000, TrialDays: 14, RequiresCard: true})
	c := e.checkout(p.Monthly, "")
	if c.Session.AmountMinor != 1000 {
		t.Fatalf("charged now = %d, want the full 1000", c.Session.AmountMinor)
	}
	if fc, _ := e.fake.Checkout(*c.Session.ExternalSessionID); fc.Request.TrialDays != 0 || fc.Request.AmountMinor != 1000 {
		t.Fatalf("request = %+v, want no trial", fc.Request)
	}
}

func TestCompleteCheckoutTwiceIsANoOp(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	c := e.checkout(p.Monthly, "")
	ev := e.pay(c)
	e.inTx(func(tx pgx.Tx) error { return e.svc.CompleteCheckout(e.ctx, tx, e.ws, c.Session.ID, ev) })

	page, err := e.svc.Invoices(e.ctx, e.ws, "")
	if err != nil || len(page.Invoices) != 1 {
		t.Fatalf("invoices = %d, %v", len(page.Invoices), err)
	}
	s, _ := e.svc.CheckoutSession(e.ctx, e.ws, c.Session.ID)
	if s.Status != model.CheckoutCompleted {
		t.Fatalf("status = %s", s.Status)
	}
	// Another workspace can't see or complete it.
	other := e.workspace()
	if _, err := e.svc.CheckoutSession(e.ctx, other, c.Session.ID); !errors.Is(err, model.ErrCheckoutNotFound) {
		t.Fatalf("other workspace: %v", err)
	}
}

func TestProviderFailureFailsTheCheckout(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	d := e.discount(func(d *model.NewDiscount) { d.PerWorkspaceLimit = 2 })
	e.checkout(p.Yearly, d.Code) // mirrors the coupon, so the next provider call is the checkout
	e.fake.FailNext(payment.ErrDeclined)
	key := uuid.NewString()
	_, err := e.svc.CreateCheckout(e.ctx, e.ws, e.user, model.CheckoutInput{PlanPriceID: p.Monthly, DiscountCode: d.Code, IdempotencyKey: key})
	if !errors.Is(err, model.ErrPaymentProviderDeclined) {
		t.Fatalf("declined: %v", err)
	}
	again, err := e.svc.CreateCheckout(e.ctx, e.ws, e.user, model.CheckoutInput{PlanPriceID: p.Monthly, DiscountCode: d.Code, IdempotencyKey: key})
	if err != nil || again.Session.Status != model.CheckoutFailed {
		t.Fatalf("replay after failure: %+v, %v", again.Session, err)
	}
	// The yearly checkout above still holds the other use.
	if status, count := e.redemption(again.Session.ID); status != model.RedemptionReleased || count != 1 {
		t.Fatalf("discount after failure: %s, %d", status, count)
	}
}

func TestCompleteTestCheckoutGoesThroughTheWebhook(t *testing.T) {
	e := newT3Env(t)
	queue := &recordingQueue{}
	e.svc.queue = queue
	p := e.plan(t3PlanOpts{Monthly: 1000})
	started, err := e.svc.CreateCheckout(e.ctx, e.ws, e.user, model.CheckoutInput{
		PlanPriceID: p.Monthly, IdempotencyKey: "t3-test-complete-" + uuid.NewString(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.CompleteTestCheckout(e.ctx, e.ws, started.Session.ID); err != nil {
		t.Fatal(err)
	}
	jobs := queue.all()
	if len(jobs) != 1 {
		t.Fatalf("%d jobs, want the stored webhook's", len(jobs))
	}
	args := jobs[0].(model.ProcessPaymentEventArgs)
	if err := e.svc.ProcessPaymentEvent(e.ctx, args.PaymentEventID, args.Event); err != nil {
		t.Fatal(err)
	}
	if c, err := e.svc.CheckoutSession(e.ctx, e.ws, started.Session.ID); err != nil || c.Status != model.CheckoutCompleted {
		t.Fatalf("session after the webhook: %+v, %v", c, err)
	}
	// Done once: a second call changes nothing.
	if _, err := e.svc.CompleteTestCheckout(e.ctx, e.ws, started.Session.ID); err != nil || len(queue.all()) != 1 {
		t.Errorf("second call: %v, %d jobs", err, len(queue.all()))
	}
	if _, err := e.svc.CompleteTestCheckout(e.ctx, e.ws, uuid.New()); !errors.Is(err, model.ErrCheckoutNotFound) {
		t.Errorf("unknown session: %v", err)
	}
}

// A retry of a checkout whose first try stopped before the provider's answer
// was saved gets a payment form again, from the same provider checkout.
func TestRetryResumesAHalfOpenCheckout(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	in := model.CheckoutInput{PlanPriceID: p.Monthly, IdempotencyKey: uuid.NewString()}
	first, err := e.svc.CreateCheckout(e.ctx, e.ws, e.user, in)
	if err != nil {
		t.Fatal(err)
	}
	// What a failed save after the provider call leaves behind.
	e.exec(`UPDATE checkout_sessions SET external_session_id = NULL, subscription_id = NULL WHERE id = $1`, first.Session.ID)
	e.exec(`DELETE FROM subscriptions WHERE id = $1`, *first.Session.SubscriptionID)

	again, err := e.svc.CreateCheckout(e.ctx, e.ws, e.user, in)
	if err != nil {
		t.Fatal(err)
	}
	if again.Session.ID != first.Session.ID {
		t.Fatalf("retry opened session %s, want %s", again.Session.ID, first.Session.ID)
	}
	if again.ClientSecret == "" || again.Session.ExternalSessionID == nil ||
		*again.Session.ExternalSessionID != *first.Session.ExternalSessionID {
		t.Fatalf("retry = %+v, want the first provider checkout with its form", again)
	}
}

// A lifetime deal a discount (or credit) covers in full completes inside Relay:
// providers refuse a charge of 0, so none is asked.
func TestFreeOrderCompletesWithoutTheProvider(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	ltd := e.price(p.ID, model.BillingOneTime, "", 5000)
	d := e.discount(func(d *model.NewDiscount) { d.PercentBps = ptrTo(10000) })
	in := model.CheckoutInput{PlanPriceID: ltd, DiscountCode: d.Code, IdempotencyKey: uuid.NewString()}

	c, err := e.svc.CreateCheckout(e.ctx, e.ws, e.user, in)
	if err != nil {
		t.Fatal(err)
	}
	s := c.Session
	if s.Status != model.CheckoutCompleted || s.AmountMinor != 0 || s.ExternalSessionID != nil || c.URL != "" || c.ClientSecret != "" {
		t.Fatalf("checkout = %+v, want completed with nothing for the browser to open", c)
	}
	o, err := e.svc.CurrentOrder(e.ctx, e.ws)
	if err != nil || o.Status != model.OrderPaid || o.DiscountMinor != 5000 || o.PaidMinor != 0 {
		t.Fatalf("order = %+v, %v", o, err)
	}
	e.wantCache(e.ws, p.Code)
	if status, _ := e.redemption(s.ID); status != model.RedemptionRedeemed {
		t.Errorf("redemption = %s", status)
	}
	page, err := e.svc.Invoices(e.ctx, e.ws, "")
	if err != nil || len(page.Invoices) != 1 {
		t.Fatalf("invoices = %+v, %v", page.Invoices, err)
	}
	if inv := page.Invoices[0]; inv.TotalMinor != 0 || inv.DiscountMinor != 5000 || inv.SubtotalMinor != 5000 || inv.Status != model.InvoicePaid {
		t.Errorf("invoice = %+v", inv)
	}

	again, err := e.svc.CreateCheckout(e.ctx, e.ws, e.user, in)
	if err != nil || again.Session.ID != s.ID || again.Session.Status != model.CheckoutCompleted {
		t.Fatalf("replay = %+v, %v", again.Session, err)
	}
}

// A payment under the provider's minimum is refused before any order exists,
// whether it is the whole price or one of the installments Relay charges.
func TestOrderUnderProviderMinimumIsRefused(t *testing.T) {
	e := newT3Env(t)
	caps := e.fake.Capabilities()
	t.Cleanup(func() { e.fake.SetCapabilities(caps) })
	floored := caps
	floored.MinChargeMinor = 100
	e.fake.SetCapabilities(floored)

	p := e.plan(t3PlanOpts{Monthly: 1000})
	ltd := e.price(p.ID, model.BillingOneTime, "", 5000)
	d := e.discount(func(d *model.NewDiscount) {
		d.Kind, d.PercentBps, d.AmountOffMinor, d.Currency = model.DiscountAmount, nil, ptrTo(int64(4950)), ptrTo("USD")
	})
	_, err := e.svc.CreateCheckout(e.ctx, e.ws, e.user, model.CheckoutInput{
		PlanPriceID: ltd, DiscountCode: d.Code, IdempotencyKey: uuid.NewString(),
	})
	if !errors.Is(err, model.ErrBelowProviderMinimum) {
		t.Fatalf("50 due now: err = %v", err)
	}

	offer, err := e.svc.CreateInstallmentOffer(e.ctx, model.NewInstallmentOffer{
		PlanPriceID: ltd, Count: 3, TotalMinor: 250, GraceDays: 7, MaxMissed: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	link := model.CollectionAppLink
	if _, err := e.svc.CreateProviderPrice(e.ctx, model.NewProviderPrice{
		PlanPriceID: ltd, InstallmentOfferID: &offer, Provider: string(payment.Stripe), Collection: &link,
	}); err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.CreateCheckout(e.ctx, e.ws, e.user, model.CheckoutInput{
		PlanPriceID: ltd, InstallmentOfferID: &offer, IdempotencyKey: uuid.NewString(),
	})
	if !errors.Is(err, model.ErrBelowProviderMinimum) {
		t.Fatalf("installments of 83: err = %v", err)
	}

	var orders int
	if err := e.pool.QueryRow(e.ctx, `SELECT count(*) FROM orders WHERE workspace_id = $1`, e.ws).Scan(&orders); err != nil {
		t.Fatal(err)
	}
	if orders != 0 {
		t.Fatalf("%d orders created, want none", orders)
	}
}
