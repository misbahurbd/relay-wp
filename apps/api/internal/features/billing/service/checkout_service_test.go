package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
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

// A deactivated plan is off sale through every price it has, even to someone
// holding a price ID. Workspaces already on it keep it.
func TestDeactivatedPlanIsOffSale(t *testing.T) {
	e := newT3Env(t)
	old := e.plan(t3PlanOpts{Monthly: 1000})
	other := e.plan(t3PlanOpts{Monthly: 3000})
	sub := e.subscribe(other.Monthly)
	if err := e.svc.DeactivateCatalogRow(e.ctx, CatalogPlan, old.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := e.svc.ChangePlan(e.ctx, e.ws, e.user, old.Monthly); !errors.Is(err, model.ErrPriceNotFound) {
		t.Errorf("plan change to a deactivated plan: %v", err)
	}
	fresh := e.workspace()
	if _, err := e.svc.UpdateProfile(e.ctx, fresh, model.ProfileInput{LegalName: "Fresh Ltd", Email: "billing@fresh.test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.CreateCheckout(e.ctx, fresh, e.user, model.CheckoutInput{
		PlanPriceID: old.Monthly, IdempotencyKey: uuid.NewString(),
	}); !errors.Is(err, model.ErrPriceNotFound) {
		t.Errorf("checkout of a deactivated plan: %v", err)
	}

	if err := e.svc.DeactivateCatalogRow(e.ctx, CatalogPlan, other.ID); err != nil {
		t.Fatal(err)
	}
	e.wantCache(e.ws, other.Code)
	if got, err := e.svc.Subscription(e.ctx, e.ws); err != nil || got.ID != sub.ID {
		t.Fatalf("subscriber on a deactivated plan: %+v, %v", got, err)
	}
}

// A subscription checkout and a lifetime-deal checkout are never open at once
// (both paid would leave two plans billing), and an abandoned one never
// blocks the other: the newest checkout expires the older one, as a second
// subscription checkout does.
func TestSubscriptionAndLifetimeCheckoutsDoNotOverlap(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	lifetime := e.price(p.ID, model.BillingOneTime, "", 30000)
	status := func(id uuid.UUID) string {
		got, err := e.svc.CheckoutSession(e.ctx, e.ws, id)
		if err != nil {
			t.Fatal(err)
		}
		return got.Status
	}

	ltd := e.checkout(lifetime, "")
	sub := e.checkout(p.Monthly, "")
	if s := status(ltd.Session.ID); s != model.CheckoutExpired {
		t.Errorf("lifetime checkout once a subscription checkout started: %s, want expired", s)
	}
	e.checkout(lifetime, "")
	if s := status(sub.Session.ID); s != model.CheckoutExpired {
		t.Errorf("subscription checkout once a lifetime checkout started: %s, want expired", s)
	}
}

// A second subscription checkout expires the first. If the first is paid
// anyway, its provider subscription is canceled and the payment refunded, so
// nothing bills twice, and the event applies instead of failing for weeks.
func TestDuplicateSubscriptionCheckoutIsRefunded(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	first := e.checkout(p.Monthly, "")
	second := e.checkout(p.Monthly, "")
	if got, err := e.svc.CheckoutSession(e.ctx, e.ws, first.Session.ID); err != nil || got.Status != model.CheckoutExpired {
		t.Errorf("first checkout %s, %v once a second started, want expired", got.Status, err)
	}

	e.pay(second)
	ev := e.pay(first)
	e.sendRefunds()

	var refunded []payment.Refund
	for _, r := range e.fake.Refunds() {
		if r.ExternalPaymentID == ev.Payment.ExternalID {
			refunded = append(refunded, r)
		}
	}
	if len(refunded) != 1 || refunded[0].AmountMinor != 1000 {
		t.Errorf("refunds of the duplicate payment = %+v, want one of 1000", refunded)
	}
	if fs, _ := e.fake.Subscription(ev.Payment.ExternalSubscriptionID); fs.Status != payment.SubscriptionCanceled {
		t.Errorf("duplicate provider subscription %s, want canceled", fs.Status)
	}
	if got, _ := e.svc.CheckoutSession(e.ctx, e.ws, first.Session.ID); got.Status != model.CheckoutFailed {
		t.Errorf("duplicate checkout %s, want failed", got.Status)
	}
	live, err := e.svc.Subscription(e.ctx, e.ws)
	if err != nil || live.ID != *second.Session.SubscriptionID || live.Status != model.SubscriptionActive {
		t.Errorf("live subscription = %+v, %v; want the second checkout's", live, err)
	}
	if g := e.grant(e.ws); g == nil || !sameID(g.SubscriptionID, live.ID) {
		t.Errorf("grant = %+v, want the live subscription's", g)
	}
}

// A duplicate subscription checkout read back after its webhook (another
// event ID, the same payment) asks the provider for one refund: Creem has no
// idempotency key, so a second request would refund twice.
func TestDuplicateSubscriptionCheckoutRefundsOnce(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	first := e.checkout(p.Monthly, "")
	second := e.checkout(p.Monthly, "")
	e.pay(second)
	ev := e.pay(first)
	readBack, err := e.fake.FetchCheckout(e.ctx, *first.Session.ExternalSessionID)
	if err != nil || readBack.ExternalID == ev.ExternalID || readBack.Payment.ExternalID != ev.Payment.ExternalID {
		t.Fatalf("read back = %+v, %v; want the same payment under another event ID", readBack, err)
	}
	e.inTx(func(tx pgx.Tx) error { return e.svc.CompleteCheckout(e.ctx, tx, e.ws, first.Session.ID, readBack) })
	e.sendRefunds()
	e.sendRefunds() // a retried job asks nothing more

	asked := 0
	for _, r := range e.fake.RefundRequests() {
		if r.ExternalPaymentID == ev.Payment.ExternalID {
			asked++
		}
	}
	if asked != 1 {
		t.Errorf("refund requests for the duplicate payment = %d, want 1", asked)
	}
}

// A duplicate subscription whose provider refuses for good to cancel it is
// canceled by the hourly sweep even when its row never got the provider's ID
// at checkout start: the ID comes from the payment, and the sweep never
// forgets a refused cancel it couldn't send.
func TestRefusedDuplicateCancelIsRetriedWithoutAStoredID(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	first := e.checkout(p.Monthly, "")
	second := e.checkout(p.Monthly, "")
	e.pay(second)
	e.exec(`UPDATE subscriptions SET external_subscription_id = NULL WHERE id = $1`, *first.Session.SubscriptionID)

	e.fake.FailNext(fmt.Errorf("fake: %w", payment.ErrInvalidRequest))
	ev := e.pay(first)
	if _, err := e.svc.RetryRefusedCancels(e.ctx); err != nil {
		t.Fatal(err)
	}
	if fs, _ := e.fake.Subscription(ev.Payment.ExternalSubscriptionID); fs.Status != payment.SubscriptionCanceled {
		t.Errorf("duplicate provider subscription after the sweep = %s, want canceled", fs.Status)
	}
}

// A refused cancel on a subscription with no provider ID stays for a person:
// the sweep can't send it, so it doesn't clear it.
func TestRefusedCancelWithoutAnIDStays(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	e.exec(`UPDATE subscriptions SET external_subscription_id = NULL, provider_cancel_refused_at = now() WHERE id = $1`, sub.ID)
	if _, err := e.svc.RetryRefusedCancels(e.ctx); err != nil {
		t.Fatal(err)
	}
	var outstanding bool
	if err := e.pool.QueryRow(e.ctx, `SELECT provider_cancel_refused_at IS NOT NULL FROM subscriptions WHERE id = $1`,
		sub.ID).Scan(&outstanding); err != nil {
		t.Fatal(err)
	}
	if !outstanding {
		t.Error("the sweep cleared a refused cancel it never sent")
	}
}

// A lifetime deal paid while a subscription is live (here a late payment for
// an expired subscription checkout) cancels that subscription, with no credit
// beyond the deal's quote.
func TestLifetimeDealCancelsTheSubscriptionLiveAtPayment(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	lifetime := e.price(p.ID, model.BillingOneTime, "", 30000)
	sub := e.checkout(p.Monthly, "")
	e.inTx(func(tx pgx.Tx) error {
		return e.svc.FailCheckout(e.ctx, tx, e.ws, sub.Session.ID, model.CheckoutExpired)
	})
	ltd := e.checkout(lifetime, "")

	e.pay(sub)
	live, err := e.svc.Subscription(e.ctx, e.ws)
	if err != nil || live.Status != model.SubscriptionActive {
		t.Fatalf("late subscription payment: %s, %v", live.Status, err)
	}
	e.pay(ltd)

	if got, err := e.svc.Subscription(e.ctx, e.ws); err != nil || got.Status != model.SubscriptionCanceled {
		t.Errorf("subscription %s, %v after the lifetime deal, want canceled", got.Status, err)
	}
	if fs, _ := e.fake.Subscription(*live.ExternalSubscriptionID); fs.Status != payment.SubscriptionCanceled {
		t.Errorf("provider subscription %s, want canceled", fs.Status)
	}
	if g := e.grant(e.ws); g == nil || g.OrderID == nil || g.Status != model.GrantActive {
		t.Errorf("grant = %+v, want the lifetime deal's", g)
	}
	var credit int64
	if err := e.pool.QueryRow(e.ctx, `SELECT credit_minor FROM orders WHERE id = $1`, *ltd.Session.OrderID).Scan(&credit); err != nil {
		t.Fatal(err)
	}
	if credit != 0 {
		t.Errorf("order credit %d, want 0 (none was quoted)", credit)
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
	// The newer checkout expired the yearly one, which gave its use back too.
	if status, count := e.redemption(again.Session.ID); status != model.RedemptionReleased || count != 0 {
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
	// Done once: a second call queues nothing (the webhook queued the invoice document).
	queued := len(queue.all())
	if _, err := e.svc.CompleteTestCheckout(e.ctx, e.ws, started.Session.ID); err != nil || len(queue.all()) != queued {
		t.Errorf("second call: %v, %d jobs, want %d", err, len(queue.all()), queued)
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

// Creem gives one customer ID to every workspace paid for from one email: a
// second workspace's checkout must still activate.
func TestWorkspacesMayShareAProviderCustomer(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	shared := e.pay(e.checkout(p.Monthly, "")).Payment.ExternalCustomerID
	if shared == "" {
		t.Fatal("the fake named no customer")
	}

	e.ws = e.workspace()
	if _, err := e.svc.UpdateProfile(e.ctx, e.ws, model.ProfileInput{LegalName: "Second", Email: "billing@acme.test"}); err != nil {
		t.Fatal(err)
	}
	c := e.checkout(p.Monthly, "")
	// Creem names the customer only once the checkout is paid.
	e.exec(`UPDATE billing_customers SET external_customer_id = NULL WHERE workspace_id = $1`, e.ws)
	ev, err := e.fake.CompleteCheckout(*c.Session.ExternalSessionID)
	if err != nil {
		t.Fatal(err)
	}
	ev.Payment.ExternalCustomerID = shared
	if err := database.WithTx(e.ctx, e.pool, func(tx pgx.Tx) error {
		return e.svc.CompleteCheckout(e.ctx, tx, e.ws, c.Session.ID, ev)
	}); err != nil {
		t.Fatalf("second workspace's checkout: %v", err)
	}
	if sub, err := e.svc.Subscription(e.ctx, e.ws); err != nil || sub.Status != model.SubscriptionActive {
		t.Fatalf("second workspace's subscription = %+v, %v", sub, err)
	}
}

// A deactivated plan is off sale, every price of it included, while its
// subscribers keep renewing.
func TestDeactivatedPlanStopsSellingButKeepsRenewing(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	sub := e.subscribe(p.Monthly)
	if err := e.svc.DeactivateCatalogRow(e.ctx, CatalogPlan, p.ID); err != nil {
		t.Fatal(err)
	}

	buyer := e.workspace()
	if _, err := e.svc.UpdateProfile(e.ctx, buyer, model.ProfileInput{LegalName: "Buyer", Email: "buyer@acme.test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.CreateCheckout(e.ctx, buyer, e.user, model.CheckoutInput{
		PlanPriceID: p.Yearly, IdempotencyKey: uuid.NewString(),
	}); !errors.Is(err, model.ErrPriceNotFound) {
		t.Fatalf("checkout for a deactivated plan's price: %v, want ErrPriceNotFound", err)
	}
	if _, err := e.svc.PreviewChange(e.ctx, e.ws, p.Yearly); !errors.Is(err, model.ErrPriceNotFound) {
		t.Fatalf("change to a deactivated plan's price: %v, want ErrPriceNotFound", err)
	}

	at := sub.CurrentPeriodEnd.Add(time.Minute)
	end := at.AddDate(0, 1, 0)
	e.inTx(func(tx pgx.Tx) error {
		return e.svc.ApplySubscriptionPayment(e.ctx, tx, e.ws, sub.ID, payment.Payment{
			ExternalID: "pay_" + uuid.NewString(), SubtotalMinor: 1000, TotalMinor: 1000, Currency: "USD",
			PeriodStart: &at, PeriodEnd: &end, PaidAt: &at,
		}, true)
	})
	if got, err := e.svc.Subscription(e.ctx, e.ws); err != nil || got.Status != model.SubscriptionActive {
		t.Fatalf("subscriber after renewal = %+v, %v", got, err)
	}
	e.wantCache(e.ws, p.Code)
}

// Prices are sold as is (case 69): the database refuses to change one.
func TestPlanPricesNeverChange(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	for _, set := range []string{"amount_minor = 2000", "currency = 'EUR'", "interval = 'year'"} {
		if _, err := e.pool.Exec(e.ctx, `UPDATE plan_prices SET `+set+` WHERE id = $1`, p.Monthly); err == nil {
			t.Errorf("SET %s was allowed", set)
		}
	}
	e.exec(`UPDATE plan_prices SET is_active = false WHERE id = $1`, p.Monthly)
}
