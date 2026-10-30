package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v87"
	"github.com/stripe/stripe-go/v87/webhook"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

// Metadata keys Relay sets on Stripe objects. relay_reference is our
// checkout_sessions.id; webhooks map back to a workspace only through it.
const (
	stripeMetaReference = "relay_reference"
	stripeMetaWorkspace = "relay_workspace_id"
	stripeMetaSchedule  = "relay_schedule"
	stripeMetaDiscount  = "relay_discount_id"
	stripeMetaReason    = "relay_reason"
)

// stripeSignatureHeader carries Stripe's webhook signature.
const stripeSignatureHeader = "Stripe-Signature"

// stripeCheckoutTTL is how long a checkout stays payable: Stripe expires an
// incomplete subscription after 23 hours, and a PaymentIntent gets the same.
const stripeCheckoutTTL = 23 * time.Hour

// stripeGateway is Stripe with embedded checkout through Elements: every
// checkout returns a client secret for a PaymentIntent, an invoice's
// confirmation secret, or a SetupIntent (a trial with a card).
type stripeGateway struct {
	secretKey      string
	publishableKey string
	webhookSecret  string
	client         *http.Client
	sc             *stripe.Client
	now            func() time.Time
}

func newStripe(cfg config.Payment, client *http.Client) (*stripeGateway, error) {
	if cfg.StripeSecretKey == "" || cfg.StripePublishableKey == "" || cfg.StripeWebhookSecret == "" {
		return nil, errors.New("STRIPE_SECRET_KEY, STRIPE_PUBLISHABLE_KEY and STRIPE_WEBHOOK_SECRET are required " +
			"when PAYMENT_PROVIDERS includes stripe")
	}
	return &stripeGateway{
		secretKey: cfg.StripeSecretKey, publishableKey: cfg.StripePublishableKey,
		webhookSecret: cfg.StripeWebhookSecret, client: client,
		sc:  newStripeClient(cfg.StripeSecretKey, client, nil, nil),
		now: time.Now,
	}, nil
}

// newStripeClient builds a stripe-go client on the shared http.Client. baseURL
// and retries are nil in production; tests point them at a fake API.
func newStripeClient(key string, client *http.Client, baseURL *string, retries *int64) *stripe.Client {
	backends := stripe.NewBackendsWithConfig(&stripe.BackendConfig{
		HTTPClient: client,
		// stripe-go logs failed requests itself; keep its output out of ours.
		LeveledLogger:     &stripe.LeveledLogger{Level: stripe.LevelNull},
		URL:               baseURL,
		MaxNetworkRetries: retries,
	})
	return stripe.NewClient(key, stripe.WithBackends(backends))
}

func (g *stripeGateway) Provider() Provider { return Stripe }

func (g *stripeGateway) Capabilities() Capabilities {
	return Capabilities{
		EmbeddedCheckout: true, RenewsSubscriptions: true, Installments: true, Coupons: true,
		PartialRefunds: true, Currencies: []string{"USD"}, PublishableKey: g.publishableKey,
	}
}

func (g *stripeGateway) CreateCheckout(ctx context.Context, req CheckoutRequest) (Checkout, error) {
	if err := validateStripeCheckout(req); err != nil {
		return Checkout{}, err
	}
	customerID, err := g.ensureCustomer(ctx, req)
	if err != nil {
		return Checkout{}, err
	}
	var c Checkout
	switch req.Mode {
	case ModePayment:
		c, err = g.createPaymentIntent(ctx, req, customerID)
	case ModeSubscription:
		c, err = g.createSubscription(ctx, req, customerID)
	case ModeInstallments:
		c, err = g.createInstallments(ctx, req, customerID)
	}
	if err != nil {
		return Checkout{}, err
	}
	c.ExternalCustomerID = customerID
	c.ExpiresAt = g.now().UTC().Add(stripeCheckoutTTL)
	return c, nil
}

func validateStripeCheckout(req CheckoutRequest) error {
	var missing []string
	if req.Reference == uuid.Nil {
		missing = append(missing, "reference")
	}
	if req.Currency == "" {
		missing = append(missing, "currency")
	}
	switch req.Mode {
	case ModePayment:
		if req.AmountMinor <= 0 {
			missing = append(missing, "amount")
		}
	case ModeSubscription:
		if req.ExternalPriceID == "" {
			missing = append(missing, "external price")
		}
	case ModeInstallments:
		if req.ExternalPriceID == "" {
			missing = append(missing, "external price")
		}
		if len(req.Schedule) < 2 || slices.ContainsFunc(req.Schedule, func(a int64) bool { return a <= 0 }) {
			missing = append(missing, "schedule")
		}
	default:
		missing = append(missing, "mode")
	}
	if len(missing) > 0 {
		return fmt.Errorf("stripe: checkout needs %s: %w", strings.Join(missing, ", "), ErrInvalidRequest)
	}
	return nil
}

// ensureCustomer returns req.Customer.ExternalID, or creates a Stripe customer from it.
func (g *stripeGateway) ensureCustomer(ctx context.Context, req CheckoutRequest) (string, error) {
	cu := req.Customer
	if cu.ExternalID != "" {
		return cu.ExternalID, nil
	}
	p := &stripe.CustomerCreateParams{
		Name: optString(cu.Name), Email: optString(cu.Email), Phone: optString(cu.Phone),
		Metadata: map[string]string{stripeMetaWorkspace: cu.WorkspaceID.String()},
	}
	if a := cu.Address; a != (Address{}) {
		p.Address = &stripe.AddressParams{
			Line1: optString(a.Line1), Line2: optString(a.Line2), City: optString(a.City),
			State: optString(a.State), PostalCode: optString(a.PostalCode), Country: optString(a.Country),
		}
	}
	if cu.TaxIDType != "" && cu.TaxID != "" {
		p.TaxIDData = []*stripe.CustomerCreateTaxIDDataParams{{Type: stripe.String(cu.TaxIDType), Value: stripe.String(cu.TaxID)}}
	}
	setIdempotency(&p.Params, req.IdempotencyKey, "customer")
	created, err := g.sc.V1Customers.Create(ctx, p)
	if err != nil {
		return "", stripeError("create customer", err)
	}
	return created.ID, nil
}

// createPaymentIntent charges req.AmountMinor once. Relay has already taken
// any discount off, so no coupon applies.
func (g *stripeGateway) createPaymentIntent(ctx context.Context, req CheckoutRequest, customerID string) (Checkout, error) {
	p := &stripe.PaymentIntentCreateParams{
		Amount: stripe.Int64(req.AmountMinor), Currency: stripe.String(strings.ToLower(req.Currency)),
		Customer: stripe.String(customerID), Description: optString(req.Description),
		AutomaticPaymentMethods: &stripe.PaymentIntentCreateAutomaticPaymentMethodsParams{Enabled: stripe.Bool(true)},
		Metadata:                map[string]string{stripeMetaReference: req.Reference.String()},
	}
	setIdempotency(&p.Params, req.IdempotencyKey, "payment_intent")
	pi, err := g.sc.V1PaymentIntents.Create(ctx, p)
	if err != nil {
		return Checkout{}, stripeError("create payment intent", err)
	}
	return Checkout{ExternalSessionID: pi.ID, ClientSecret: pi.ClientSecret}, nil
}

// createSubscription starts an incomplete subscription that Elements confirms.
func (g *stripeGateway) createSubscription(ctx context.Context, req CheckoutRequest, customerID string) (Checkout, error) {
	item := &stripe.SubscriptionCreateItemParams{Price: stripe.String(req.ExternalPriceID)}
	sub, err := g.startSubscription(ctx, req, customerID, item, "subscription")
	if err != nil {
		return Checkout{}, err
	}
	secret, err := subscriptionClientSecret(sub)
	if err != nil {
		return Checkout{}, err
	}
	return Checkout{ExternalSessionID: sub.ID, ExternalSubscriptionID: sub.ID, ClientSecret: secret}, nil
}

// createInstallments starts a subscription for the first installment, then
// hands it to a schedule that bills len(req.Schedule) periods and cancels.
// Runs of equal amounts share a phase; an amount that isn't the price's own
// (the last installment absorbs rounding) uses inline price data.
func (g *stripeGateway) createInstallments(ctx context.Context, req CheckoutRequest, customerID string) (Checkout, error) {
	price, err := g.sc.V1Prices.Retrieve(ctx, req.ExternalPriceID, nil)
	if err != nil {
		return Checkout{}, stripeError("retrieve price", err)
	}
	if price.Recurring == nil || price.Product == nil {
		return Checkout{}, fmt.Errorf("stripe: price %q is not recurring: %w", req.ExternalPriceID, ErrInvalidRequest)
	}
	currency := strings.ToLower(req.Currency)
	interval, count := string(price.Recurring.Interval), max(price.Recurring.IntervalCount, 1)

	first := &stripe.SubscriptionCreateItemParams{Price: stripe.String(price.ID)}
	if req.Schedule[0] != price.UnitAmount {
		first = &stripe.SubscriptionCreateItemParams{PriceData: &stripe.SubscriptionCreateItemPriceDataParams{
			Currency: stripe.String(currency), Product: stripe.String(price.Product.ID),
			UnitAmount: stripe.Int64(req.Schedule[0]),
			Recurring: &stripe.SubscriptionCreateItemPriceDataRecurringParams{
				Interval: stripe.String(interval), IntervalCount: stripe.Int64(count),
			},
		}}
	}
	reqNoCoupon := req
	reqNoCoupon.ExternalCouponID = "" // Relay takes the discount off the schedule.
	reqNoCoupon.TrialDays = 0
	sub, err := g.startSubscription(ctx, reqNoCoupon, customerID, first, "installments")
	if err != nil {
		return Checkout{}, err
	}
	secret, err := subscriptionClientSecret(sub)
	if err != nil {
		return Checkout{}, err
	}

	sp := &stripe.SubscriptionScheduleCreateParams{FromSubscription: stripe.String(sub.ID)}
	setIdempotency(&sp.Params, req.IdempotencyKey, "schedule")
	sched, err := g.sc.V1SubscriptionSchedules.Create(ctx, sp)
	if err != nil {
		return Checkout{}, stripeError("create subscription schedule", err)
	}
	if len(sched.Phases) == 0 {
		return Checkout{}, fmt.Errorf("stripe: schedule %s has no phases: %w", sched.ID, ErrUnavailable)
	}

	meta := map[string]string{stripeMetaReference: req.Reference.String(), stripeMetaSchedule: sched.ID}
	up := &stripe.SubscriptionScheduleUpdateParams{
		EndBehavior: stripe.String("cancel"), ProrationBehavior: stripe.String("none"), Metadata: meta,
	}
	for i, run := range installmentRuns(req.Schedule) {
		item := &stripe.SubscriptionScheduleUpdatePhaseItemParams{Price: stripe.String(price.ID)}
		if run.amount != price.UnitAmount {
			item = &stripe.SubscriptionScheduleUpdatePhaseItemParams{PriceData: &stripe.SubscriptionItemPriceDataParams{
				Currency: stripe.String(currency), Product: stripe.String(price.Product.ID),
				UnitAmount: stripe.Int64(run.amount),
				Recurring: &stripe.SubscriptionItemPriceDataRecurringParams{
					Interval: stripe.String(interval), IntervalCount: stripe.Int64(count),
				},
			}}
		}
		phase := &stripe.SubscriptionScheduleUpdatePhaseParams{
			Items:    []*stripe.SubscriptionScheduleUpdatePhaseItemParams{item},
			Metadata: meta,
			Duration: &stripe.SubscriptionScheduleUpdatePhaseDurationParams{
				Interval: stripe.String(interval), IntervalCount: stripe.Int64(count * int64(run.n)),
			},
		}
		if i == 0 {
			phase.StartDate = stripe.Int64(sched.Phases[0].StartDate)
		}
		up.Phases = append(up.Phases, phase)
	}
	setIdempotency(&up.Params, req.IdempotencyKey, "schedule_phases")
	if _, err := g.sc.V1SubscriptionSchedules.Update(ctx, sched.ID, up); err != nil {
		return Checkout{}, stripeError("set schedule phases", err)
	}
	return Checkout{
		ExternalSessionID: sub.ID, ExternalSubscriptionID: sub.ID, ExternalScheduleID: sched.ID, ClientSecret: secret,
	}, nil
}

// installmentRun is n consecutive installments of the same amount.
type installmentRun struct {
	amount int64
	n      int
}

func installmentRuns(schedule []int64) []installmentRun {
	var runs []installmentRun
	for _, a := range schedule {
		if len(runs) > 0 && runs[len(runs)-1].amount == a {
			runs[len(runs)-1].n++
			continue
		}
		runs = append(runs, installmentRun{amount: a, n: 1})
	}
	return runs
}

// startSubscription creates a default_incomplete subscription with the first
// invoice's confirmation secret (or the trial's SetupIntent) expanded.
func (g *stripeGateway) startSubscription(
	ctx context.Context, req CheckoutRequest, customerID string, item *stripe.SubscriptionCreateItemParams, step string,
) (*stripe.Subscription, error) {
	p := &stripe.SubscriptionCreateParams{
		Customer:        stripe.String(customerID),
		Items:           []*stripe.SubscriptionCreateItemParams{item},
		PaymentBehavior: stripe.String("default_incomplete"),
		PaymentSettings: &stripe.SubscriptionCreatePaymentSettingsParams{
			SaveDefaultPaymentMethod: stripe.String("on_subscription"),
		},
		Description: optString(req.Description),
		Metadata:    map[string]string{stripeMetaReference: req.Reference.String()},
		Expand:      []*string{stripe.String("latest_invoice.confirmation_secret"), stripe.String("pending_setup_intent")},
	}
	if req.ExternalCouponID != "" {
		p.Discounts = []*stripe.SubscriptionCreateDiscountParams{{Coupon: stripe.String(req.ExternalCouponID)}}
	}
	if req.TrialDays > 0 {
		p.TrialPeriodDays = stripe.Int64(int64(req.TrialDays))
	}
	setIdempotency(&p.Params, req.IdempotencyKey, step)
	sub, err := g.sc.V1Subscriptions.Create(ctx, p)
	if err != nil {
		return nil, stripeError("create subscription", err)
	}
	return sub, nil
}

// subscriptionClientSecret is what Elements confirms: the first invoice's
// payment, or for a trial (nothing due now) the SetupIntent that saves the card.
func subscriptionClientSecret(sub *stripe.Subscription) (string, error) {
	if inv := sub.LatestInvoice; inv != nil && inv.ConfirmationSecret != nil && inv.ConfirmationSecret.ClientSecret != "" {
		return inv.ConfirmationSecret.ClientSecret, nil
	}
	if si := sub.PendingSetupIntent; si != nil && si.ClientSecret != "" {
		return si.ClientSecret, nil
	}
	return "", fmt.Errorf("stripe: subscription %s has no client secret: %w", sub.ID, ErrUnavailable)
}

func (g *stripeGateway) ChangeSubscription(ctx context.Context, req ChangeRequest) (Subscription, error) {
	if req.ExternalSubscriptionID == "" || req.ExternalPriceID == "" {
		return Subscription{}, fmt.Errorf("stripe: change needs a subscription and a price: %w", ErrInvalidRequest)
	}
	cur, err := g.sc.V1Subscriptions.Retrieve(ctx, req.ExternalSubscriptionID, nil)
	if err != nil {
		return Subscription{}, stripeError("retrieve subscription", err)
	}
	if cur.Items == nil || len(cur.Items.Data) != 1 {
		return Subscription{}, fmt.Errorf("stripe: subscription %s needs exactly one item: %w", cur.ID, ErrInvalidRequest)
	}
	p := &stripe.SubscriptionUpdateParams{
		Items: []*stripe.SubscriptionUpdateItemParams{{
			ID: stripe.String(cur.Items.Data[0].ID), Price: stripe.String(req.ExternalPriceID),
		}},
		ProrationBehavior: stripe.String("none"),
		Expand:            []*string{stripe.String("latest_invoice")},
	}
	if req.ProrateNow {
		// Invoice the difference now; a declined card leaves the subscription unchanged.
		p.ProrationBehavior = stripe.String("always_invoice")
		p.PaymentBehavior = stripe.String("error_if_incomplete")
	}
	setIdempotency(&p.Params, req.IdempotencyKey, "change")
	sub, err := g.sc.V1Subscriptions.Update(ctx, cur.ID, p)
	if err != nil {
		return Subscription{}, stripeError("change subscription", err)
	}
	out := stripeSubscription(sub)
	if inv := sub.LatestInvoice; req.ProrateNow && inv != nil && inv.BillingReason == stripe.InvoiceBillingReasonSubscriptionUpdate {
		out.ProrationMinor = inv.Total
	}
	return out, nil
}

func (g *stripeGateway) CancelSubscription(ctx context.Context, req CancelRequest) (Subscription, error) {
	if req.ExternalSubscriptionID == "" {
		return Subscription{}, fmt.Errorf("stripe: cancel needs a subscription: %w", ErrInvalidRequest)
	}
	var (
		sub *stripe.Subscription
		err error
	)
	switch req.Mode {
	case CancelNow:
		sub, err = g.sc.V1Subscriptions.Cancel(ctx, req.ExternalSubscriptionID, nil)
	case CancelAtPeriodEnd, CancelUndo:
		sub, err = g.sc.V1Subscriptions.Update(ctx, req.ExternalSubscriptionID, &stripe.SubscriptionUpdateParams{
			CancelAtPeriodEnd: stripe.Bool(req.Mode == CancelAtPeriodEnd),
		})
	default:
		return Subscription{}, fmt.Errorf("stripe: cancel mode %q: %w", req.Mode, ErrInvalidRequest)
	}
	if err != nil {
		return Subscription{}, stripeError("cancel subscription", err)
	}
	return stripeSubscription(sub), nil
}

// CancelSchedule cancels the subscription schedule, which cancels the
// subscription under it at once and without proration.
func (g *stripeGateway) CancelSchedule(ctx context.Context, externalScheduleID string) error {
	if externalScheduleID == "" {
		return fmt.Errorf("stripe: cancel needs a schedule: %w", ErrInvalidRequest)
	}
	if _, err := g.sc.V1SubscriptionSchedules.Cancel(ctx, externalScheduleID, &stripe.SubscriptionScheduleCancelParams{
		Prorate: stripe.Bool(false),
	}); err != nil {
		return stripeError("cancel subscription schedule", err)
	}
	return nil
}

func (g *stripeGateway) Refund(ctx context.Context, req RefundRequest) (Refund, error) {
	if req.ExternalPaymentID == "" || req.AmountMinor <= 0 {
		return Refund{}, fmt.Errorf("stripe: refund needs a payment and an amount: %w", ErrInvalidRequest)
	}
	p := &stripe.RefundCreateParams{Amount: stripe.Int64(req.AmountMinor)}
	switch id := req.ExternalPaymentID; {
	case strings.HasPrefix(id, "ch_") || strings.HasPrefix(id, "py_"):
		p.Charge = stripe.String(id)
	case strings.HasPrefix(id, "in_"):
		pi, err := g.invoicePaymentIntent(ctx, id)
		if err != nil {
			return Refund{}, err
		}
		if pi == "" {
			return Refund{}, fmt.Errorf("stripe: invoice %s has no payment: %w", id, ErrNotFound)
		}
		p.PaymentIntent = stripe.String(pi)
	default:
		p.PaymentIntent = stripe.String(id)
	}
	switch req.Reason {
	case "duplicate", "fraudulent", "requested_by_customer":
		p.Reason = stripe.String(req.Reason)
	case "":
	default:
		p.Metadata = map[string]string{stripeMetaReason: req.Reason}
	}
	setIdempotency(&p.Params, req.IdempotencyKey, "refund")
	r, err := g.sc.V1Refunds.Create(ctx, p)
	if err != nil {
		return Refund{}, stripeError("refund", err)
	}
	out := stripeRefund(r)
	if out.ExternalPaymentID == "" {
		out.ExternalPaymentID = req.ExternalPaymentID
	}
	return out, nil
}

// invoicePaymentIntent returns the PaymentIntent that paid an invoice, or "".
func (g *stripeGateway) invoicePaymentIntent(ctx context.Context, invoiceID string) (string, error) {
	p := &stripe.InvoicePaymentListParams{Invoice: stripe.String(invoiceID), Status: stripe.String("paid")}
	p.Limit = stripe.Int64(10)
	for ip, err := range g.sc.V1InvoicePayments.List(ctx, p).All(ctx) {
		if err != nil {
			return "", stripeError("list invoice payments", err)
		}
		if ip.Payment != nil && ip.Payment.PaymentIntent != nil && ip.Payment.PaymentIntent.ID != "" {
			return ip.Payment.PaymentIntent.ID, nil
		}
	}
	return "", nil
}

// CreateCoupon creates a Stripe coupon. No promotion code: Relay owns the
// customer-facing code and attaches the coupon itself.
func (g *stripeGateway) CreateCoupon(ctx context.Context, req CouponRequest) (Coupon, error) {
	if (req.PercentBps > 0) == (req.AmountOffMinor > 0) {
		return Coupon{}, fmt.Errorf("stripe: coupon needs a percent or an amount: %w", ErrInvalidRequest)
	}
	p := &stripe.CouponCreateParams{
		Name:     optString(req.Name),
		Duration: stripe.String(req.Duration),
		Metadata: map[string]string{stripeMetaDiscount: req.DiscountID.String()},
	}
	switch req.Duration {
	case "once", "forever":
	case "repeating":
		if req.DurationMonths <= 0 {
			return Coupon{}, fmt.Errorf("stripe: repeating coupon needs months: %w", ErrInvalidRequest)
		}
		p.DurationInMonths = stripe.Int64(int64(req.DurationMonths))
	default:
		return Coupon{}, fmt.Errorf("stripe: coupon duration %q: %w", req.Duration, ErrInvalidRequest)
	}
	if req.PercentBps > 0 {
		p.PercentOff = stripe.Float64(float64(req.PercentBps) / 100)
	} else {
		if req.Currency == "" {
			return Coupon{}, fmt.Errorf("stripe: amount coupon needs a currency: %w", ErrInvalidRequest)
		}
		p.AmountOff = stripe.Int64(req.AmountOffMinor)
		p.Currency = stripe.String(strings.ToLower(req.Currency))
	}
	if req.DiscountID != uuid.Nil {
		setIdempotency(&p.Params, "coupon:"+req.DiscountID.String(), "")
	}
	c, err := g.sc.V1Coupons.Create(ctx, p)
	if err != nil {
		return Coupon{}, stripeError("create coupon", err)
	}
	return Coupon{ExternalCouponID: c.ID}, nil
}

// Card reads the subscription's default card (subscriptions save the
// card they were paid with), else the customer's invoice default.
func (g *stripeGateway) Card(ctx context.Context, req CardRequest) (*Card, error) {
	if req.ExternalCustomerID == "" {
		return nil, fmt.Errorf("stripe: payment method needs a customer: %w", ErrInvalidRequest)
	}
	if req.ExternalSubscriptionID != "" {
		sub, err := g.sc.V1Subscriptions.Retrieve(ctx, req.ExternalSubscriptionID, &stripe.SubscriptionRetrieveParams{
			Expand: []*string{stripe.String("default_payment_method")},
		})
		if err != nil {
			return nil, stripeError("retrieve subscription", err)
		}
		if pm := stripeCard(sub.DefaultPaymentMethod); pm != nil {
			return pm, nil
		}
	}
	c, err := g.sc.V1Customers.Retrieve(ctx, req.ExternalCustomerID, &stripe.CustomerRetrieveParams{
		Expand: []*string{stripe.String("invoice_settings.default_payment_method")},
	})
	if err != nil {
		return nil, stripeError("retrieve customer", err)
	}
	if c.InvoiceSettings == nil {
		return nil, nil
	}
	return stripeCard(c.InvoiceSettings.DefaultPaymentMethod), nil
}

// PortalURL opens a Stripe Billing Portal session. Which actions the portal
// offers is set in the Stripe dashboard (payment methods only; see
// docs/PHASE-7-BILLING.md section 4).
func (g *stripeGateway) PortalURL(ctx context.Context, req PortalRequest) (string, error) {
	if req.ExternalCustomerID == "" {
		return "", fmt.Errorf("stripe: portal needs a customer: %w", ErrInvalidRequest)
	}
	s, err := g.sc.V1BillingPortalSessions.Create(ctx, &stripe.BillingPortalSessionCreateParams{
		Customer: stripe.String(req.ExternalCustomerID), ReturnURL: optString(req.ReturnURL),
	})
	if err != nil {
		return "", stripeError("create portal session", err)
	}
	return s.URL, nil
}

// stripeCard returns pm's card, or nil when pm is unset, unexpanded or not a card.
func stripeCard(pm *stripe.PaymentMethod) *Card {
	if pm == nil || pm.Card == nil || pm.Card.Last4 == "" {
		return nil
	}
	return &Card{
		Brand: string(pm.Card.Brand), Last4: pm.Card.Last4, ExpMonth: int(pm.Card.ExpMonth), ExpYear: int(pm.Card.ExpYear),
	}
}

func (g *stripeGateway) ParseWebhook(ctx context.Context, header http.Header, body []byte) (Event, error) {
	if err := webhook.ValidatePayloadWithTolerance(body, header.Get(stripeSignatureHeader), g.webhookSecret,
		webhook.DefaultTolerance); err != nil {
		return Event{}, fmt.Errorf("stripe: %w: %w", ErrInvalidSignature, err)
	}
	var se stripe.Event
	if err := json.Unmarshal(body, &se); err != nil || se.ID == "" || se.Type == "" || se.Data == nil {
		return Event{}, fmt.Errorf("stripe: decode event: %w", ErrInvalidRequest)
	}
	ev := Event{
		Provider: Stripe, ExternalID: se.ID, Type: EventIgnored, ProviderType: string(se.Type),
		OccurredAt: time.Unix(se.Created, 0).UTC(), Payload: json.RawMessage(slices.Clone(body)),
	}
	if err := g.normalise(ctx, &ev, se); err != nil {
		return Event{}, err
	}
	return ev, nil
}

// normalise fills ev's type and objects from the Stripe event. Events Relay
// doesn't act on stay EventIgnored.
func (g *stripeGateway) normalise(ctx context.Context, ev *Event, se stripe.Event) error {
	decode := func(v any) error {
		if err := json.Unmarshal(se.Data.Raw, v); err != nil {
			return fmt.Errorf("stripe: decode %s: %w", se.Type, ErrInvalidRequest)
		}
		return nil
	}
	switch se.Type {
	case "payment_intent.succeeded", "payment_intent.payment_failed", "payment_intent.canceled":
		var pi stripe.PaymentIntent
		if err := decode(&pi); err != nil {
			return err
		}
		// Only Relay's one-time checkouts carry a reference; an invoice's own
		// PaymentIntent is reported through the invoice events instead.
		ev.Reference = stripeReference(pi.Metadata)
		if ev.Reference == nil {
			return nil
		}
		ev.Payment = paymentIntentPayment(&pi)
		switch se.Type {
		case "payment_intent.succeeded":
			ev.Type = EventCheckoutCompleted
			ev.Payment.PaidAt = &ev.OccurredAt
		case "payment_intent.payment_failed":
			// One declined attempt: the customer can retry with the same form,
			// so the checkout stays open until the intent is canceled.
			ev.Type = EventIgnored
		default:
			ev.Type = EventCheckoutExpired
		}

	case "invoice.paid", "invoice.payment_failed":
		var inv stripe.Invoice
		if err := decode(&inv); err != nil {
			return err
		}
		meta := invoiceSubscriptionMeta(&inv)
		if meta == nil {
			return nil // not a subscription invoice
		}
		ev.Reference = stripeReference(meta.Metadata)
		p, err := g.invoicePayment(ctx, &inv, meta)
		if err != nil {
			return err
		}
		ev.Payment = p
		first := inv.BillingReason == stripe.InvoiceBillingReasonSubscriptionCreate
		if se.Type == "invoice.paid" && first && inv.Total == 0 {
			// Nothing due now (a trial, or a 100% code): the $0 invoice is paid
			// before any card is saved. The checkout completes once the card is
			// in, so the subscription's own status decides.
			sub, err := g.sc.V1Subscriptions.Retrieve(ctx, meta.Subscription.ID, nil)
			if err != nil {
				return stripeError("retrieve subscription", err)
			}
			if sub.PendingSetupIntent != nil {
				return nil // EventIgnored: completed by customer.subscription.updated below
			}
			s := stripeSubscription(sub)
			ev.Subscription = &s
		}
		switch {
		case se.Type == "invoice.paid" && first:
			ev.Type = EventCheckoutCompleted
		case se.Type == "invoice.paid":
			ev.Type = EventPaymentSucceeded
		case first:
			// A declined first attempt; the customer can retry. The subscription
			// ending incomplete_expired is what closes the checkout.
			ev.Type = EventIgnored
		default:
			ev.Type = EventPaymentFailed
		}

	case "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := decode(&sub); err != nil {
			return err
		}
		s := stripeSubscription(&sub)
		ev.Subscription = &s
		ev.Reference = stripeReference(sub.Metadata)
		_, setupDone := se.Data.PreviousAttributes["pending_setup_intent"]
		switch {
		case sub.Status == stripe.SubscriptionStatusIncompleteExpired:
			ev.Type = EventCheckoutExpired // the first payment never went through
		case se.Type == "customer.subscription.updated" && setupDone && sub.PendingSetupIntent == nil &&
			sub.Status == stripe.SubscriptionStatusTrialing:
			// The trial's card is saved: that, not the $0 invoice, completes the checkout.
			ev.Type = EventCheckoutCompleted
			ev.Payment = &Payment{
				ExternalID: stripeInvoiceID(sub.LatestInvoice), ExternalCustomerID: s.ExternalCustomerID,
				ExternalSubscriptionID: sub.ID, Currency: strings.ToUpper(string(sub.Currency)),
				PeriodStart: s.CurrentPeriodStart, PeriodEnd: s.CurrentPeriodEnd, PaidAt: &ev.OccurredAt,
			}
		case se.Type == "customer.subscription.deleted":
			ev.Type = EventSubscriptionCanceled
		default:
			ev.Type = EventSubscriptionUpdated
		}

	case "refund.created", "refund.updated", "refund.failed", "charge.refund.updated":
		var r stripe.Refund
		if err := decode(&r); err != nil {
			return err
		}
		out := stripeRefund(&r)
		ev.Refund = &out
		ev.Type = EventRefundUpdated

	case "charge.dispute.created", "charge.dispute.closed":
		var d stripe.Dispute
		if err := decode(&d); err != nil {
			return err
		}
		p := &Payment{
			ExternalID: stripeID(d.PaymentIntent), SubtotalMinor: d.Amount, TotalMinor: d.Amount,
			Currency: strings.ToUpper(string(d.Currency)), Status: string(d.Status),
		}
		if p.ExternalID == "" {
			p.ExternalID = stripeID(d.Charge)
		}
		ev.Payment = p
		ev.Type = EventDisputeOpened
		if se.Type == "charge.dispute.closed" {
			ev.Type = EventDisputeClosed
			if d.Status != stripe.DisputeStatusLost {
				p.Status = "won" // won, or an inquiry closed without a chargeback
			}
		}
	}
	return nil
}

// stripeInvoiceID is an invoice's ID, expanded or not.
func stripeInvoiceID(inv *stripe.Invoice) string {
	if inv == nil {
		return ""
	}
	return inv.ID
}

func invoiceSubscriptionMeta(inv *stripe.Invoice) *stripe.InvoiceParentSubscriptionDetails {
	if inv.Parent == nil || inv.Parent.SubscriptionDetails == nil || inv.Parent.SubscriptionDetails.Subscription == nil {
		return nil
	}
	return inv.Parent.SubscriptionDetails
}

// invoicePayment builds the Payment for a subscription invoice. Its ExternalID
// is the PaymentIntent that paid it, so refunds and disputes (which name the
// PaymentIntent) match the same invoice; the invoice ID stands in when nothing
// was charged (a $0 or failed invoice).
func (g *stripeGateway) invoicePayment(
	ctx context.Context, inv *stripe.Invoice, meta *stripe.InvoiceParentSubscriptionDetails,
) (*Payment, error) {
	p := &Payment{
		ExternalID: inv.ID, InvoiceNumber: inv.Number, ExternalCustomerID: stripeID(inv.Customer),
		ExternalSubscriptionID: meta.Subscription.ID, ExternalScheduleID: meta.Metadata[stripeMetaSchedule],
		SubtotalMinor: inv.Subtotal, TotalMinor: inv.Total, Currency: strings.ToUpper(string(inv.Currency)),
		ReceiptURL: inv.HostedInvoiceURL,
	}
	for _, d := range inv.TotalDiscountAmounts {
		p.DiscountMinor += d.Amount
	}
	for _, t := range inv.TotalTaxes {
		p.TaxMinor += t.Amount
	}
	if inv.Lines != nil && len(inv.Lines.Data) > 0 && inv.Lines.Data[0].Period != nil {
		start, end := unixTime(inv.Lines.Data[0].Period.Start), unixTime(inv.Lines.Data[0].Period.End)
		p.PeriodStart, p.PeriodEnd = start, end
	}
	if st := inv.StatusTransitions; st != nil {
		p.PaidAt = unixTime(st.PaidAt)
	}
	if inv.AmountPaid > 0 {
		pi, err := g.invoicePaymentIntent(ctx, inv.ID)
		if err != nil {
			return nil, err
		}
		if pi != "" {
			p.ExternalID = pi
		}
	}
	return p, nil
}

func paymentIntentPayment(pi *stripe.PaymentIntent) *Payment {
	amount := pi.AmountReceived
	if amount == 0 {
		amount = pi.Amount
	}
	p := &Payment{
		ExternalID: pi.ID, ExternalCustomerID: stripeID(pi.Customer),
		SubtotalMinor: amount, TotalMinor: amount, Currency: strings.ToUpper(string(pi.Currency)),
	}
	if e := pi.LastPaymentError; e != nil {
		p.FailureCode = string(e.Code)
		if e.DeclineCode != "" {
			p.FailureCode = string(e.DeclineCode)
		}
	}
	return p
}

func stripeSubscription(s *stripe.Subscription) Subscription {
	out := Subscription{
		ExternalID: s.ID, ExternalCustomerID: stripeID(s.Customer),
		Status: stripeSubscriptionStatus(s.Status), ProviderStatus: string(s.Status),
		TrialEnd: unixTime(s.TrialEnd), CancelAtPeriodEnd: s.CancelAtPeriodEnd, CanceledAt: unixTime(s.CanceledAt),
	}
	if s.Items != nil && len(s.Items.Data) > 0 {
		it := s.Items.Data[0]
		if it.Price != nil {
			out.ExternalPriceID = it.Price.ID
		}
		out.CurrentPeriodStart, out.CurrentPeriodEnd = unixTime(it.CurrentPeriodStart), unixTime(it.CurrentPeriodEnd)
	}
	return out
}

func stripeSubscriptionStatus(s stripe.SubscriptionStatus) SubscriptionStatus {
	switch s {
	case stripe.SubscriptionStatusTrialing:
		return SubscriptionTrialing
	case stripe.SubscriptionStatusActive:
		return SubscriptionActive
	case stripe.SubscriptionStatusPastDue, stripe.SubscriptionStatusUnpaid, stripe.SubscriptionStatusPaused:
		return SubscriptionPastDue
	case stripe.SubscriptionStatusCanceled, stripe.SubscriptionStatusIncompleteExpired:
		return SubscriptionCanceled
	default:
		return SubscriptionIncomplete
	}
}

func stripeRefund(r *stripe.Refund) Refund {
	out := Refund{
		ExternalID: r.ID, ExternalPaymentID: stripeID(r.PaymentIntent), AmountMinor: r.Amount,
		Currency: strings.ToUpper(string(r.Currency)), Status: RefundPending,
	}
	if out.ExternalPaymentID == "" {
		out.ExternalPaymentID = stripeID(r.Charge)
	}
	switch r.Status {
	case stripe.RefundStatusSucceeded:
		out.Status = RefundSucceeded
	case stripe.RefundStatusFailed, stripe.RefundStatusCanceled:
		out.Status = RefundFailed
	}
	return out
}

// stripeReference reads our checkout_sessions.id back from Stripe metadata.
func stripeReference(meta map[string]string) *uuid.UUID {
	id, err := uuid.Parse(meta[stripeMetaReference])
	if err != nil {
		return nil
	}
	return &id
}

// stripeError maps a stripe-go error onto the gateway errors. Only the
// error's type, code and message are kept: never the whole object.
func stripeError(op string, err error) error {
	var se *stripe.Error
	if !errors.As(err, &se) {
		// No answer: a network error, a timeout or a canceled context.
		return fmt.Errorf("stripe: %s: %w: %w", op, ErrUnavailable, err)
	}
	kind := ErrInvalidRequest
	switch {
	case se.Type == stripe.ErrorTypeCard:
		kind = ErrDeclined
	case se.HTTPStatusCode == http.StatusNotFound || se.Code == stripe.ErrorCodeResourceMissing:
		kind = ErrNotFound
	case se.HTTPStatusCode == http.StatusTooManyRequests || se.HTTPStatusCode >= http.StatusInternalServerError ||
		se.HTTPStatusCode == http.StatusUnauthorized || se.HTTPStatusCode == http.StatusForbidden ||
		se.Type == stripe.ErrorTypeAPI:
		kind = ErrUnavailable
	}
	detail := string(se.Type)
	if se.Code != "" {
		detail += " " + string(se.Code)
	}
	if se.DeclineCode != "" {
		detail += " " + string(se.DeclineCode)
	}
	return fmt.Errorf("stripe: %s: %w (%s: %s)", op, kind, detail, se.Msg)
}

// setIdempotency sets "<key>:<step>" as the request's idempotency key, so
// each call of a multi-step checkout is retried safely on its own.
func setIdempotency(p *stripe.Params, key, step string) {
	if key == "" {
		return
	}
	if step != "" {
		key += ":" + step
	}
	p.IdempotencyKey = stripe.String(key)
}

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return stripe.String(s)
}

func unixTime(sec int64) *time.Time {
	if sec == 0 {
		return nil
	}
	t := time.Unix(sec, 0).UTC()
	return &t
}

// stripeID returns the ID of an expandable Stripe object, or "".
func stripeID[T stripe.Customer | stripe.PaymentIntent | stripe.Charge](obj *T) string {
	if obj == nil {
		return ""
	}
	switch o := any(obj).(type) {
	case *stripe.Customer:
		return o.ID
	case *stripe.PaymentIntent:
		return o.ID
	case *stripe.Charge:
		return o.ID
	}
	return ""
}
