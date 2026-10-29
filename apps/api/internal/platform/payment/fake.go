package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FakeSignatureHeader carries the fake gateway's webhook signature:
// "t=<unix seconds>,v1=<hex HMAC-SHA256 of "<t>.<body>">".
const FakeSignatureHeader = "Fake-Signature"

// DevFakeWebhookSecret signs fake webhooks in local development ("stripe:fake").
// It protects nothing; the fake is refused outside development.
const DevFakeWebhookSecret = "fake_whsec_dev" //nolint:gosec // not a credential: the fake runs only in development

// fakeTolerance is how old a signed fake webhook may be.
const fakeTolerance = 5 * time.Minute

// Fake is an in-memory Gateway for tests and local development. It answers
// under any provider's name, needs no network, and signs and verifies its
// webhooks like a real provider does.
type Fake struct {
	provider Provider
	secret   []byte

	mu            sync.Mutex
	caps          Capabilities
	now           func() time.Time
	seq           int
	checkouts     map[string]FakeCheckout // by external session ID
	byIdempotency map[string]string       // idempotency key → external session ID
	subscriptions map[string]Subscription // by external ID
	refunds       []Refund
	coupons       []CouponRequest
	failNext      error
}

// FakeCheckout is a checkout the fake started, with the request that started it.
type FakeCheckout struct {
	Request  CheckoutRequest
	Checkout Checkout
}

// NewFake builds a fake gateway named provider, which signs webhooks with secret.
func NewFake(provider Provider, secret string) *Fake {
	return &Fake{
		provider: provider,
		secret:   []byte(secret),
		caps: Capabilities{
			EmbeddedCheckout: true, RenewsSubscriptions: true, Installments: true, Coupons: true,
			PartialRefunds: true, Currencies: []string{"USD"}, PublishableKey: "pk_fake",
		},
		now:           time.Now,
		checkouts:     map[string]FakeCheckout{},
		byIdempotency: map[string]string{},
		subscriptions: map[string]Subscription{},
	}
}

// SetCapabilities replaces what the fake claims to support.
func (f *Fake) SetCapabilities(c Capabilities) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.caps = c
}

// SetClock replaces the fake's clock.
func (f *Fake) SetClock(now func() time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = now
}

// FailNext makes the next call that talks to the "provider" return err.
func (f *Fake) FailNext(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failNext = err
}

// Provider implements Gateway.
func (f *Fake) Provider() Provider { return f.provider }

// Capabilities implements Gateway.
func (f *Fake) Capabilities() Capabilities {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.caps
	c.Currencies = slices.Clone(c.Currencies)
	return c
}

// takeFailure returns and clears the error set by FailNext. Call with f.mu held.
func (f *Fake) takeFailure() error {
	err := f.failNext
	f.failNext = nil
	return err
}

// nextID returns a new fake provider ID such as "fake_cs_3". Call with f.mu held.
func (f *Fake) nextID(kind string) string {
	f.seq++
	return "fake_" + kind + "_" + strconv.Itoa(f.seq)
}

// CreateCheckout implements Gateway. A repeated idempotency key returns the first checkout.
func (f *Fake) CreateCheckout(_ context.Context, req CheckoutRequest) (Checkout, error) {
	if err := validateCheckout(req); err != nil {
		return Checkout{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.takeFailure(); err != nil {
		return Checkout{}, err
	}
	if req.IdempotencyKey != "" {
		if id, ok := f.byIdempotency[req.IdempotencyKey]; ok {
			return f.checkouts[id].Checkout, nil
		}
	}
	if req.Mode == ModeInstallments && !f.caps.Installments {
		return Checkout{}, fmt.Errorf("fake: installments: %w", ErrUnsupported)
	}

	now := f.now().UTC()
	c := Checkout{
		ExternalSessionID:  f.nextID("cs"),
		ExternalCustomerID: req.Customer.ExternalID,
		ExpiresAt:          now.Add(time.Hour),
	}
	if c.ExternalCustomerID == "" {
		c.ExternalCustomerID = f.nextID("cus")
	}
	if f.caps.EmbeddedCheckout {
		c.ClientSecret = f.nextID("secret")
	} else {
		c.URL = "https://fake.invalid/checkout/" + c.ExternalSessionID
	}
	switch req.Mode {
	case ModeSubscription:
		c.ExternalSubscriptionID = f.nextID("sub")
		status := SubscriptionIncomplete
		f.subscriptions[c.ExternalSubscriptionID] = Subscription{
			ExternalID: c.ExternalSubscriptionID, ExternalCustomerID: c.ExternalCustomerID,
			ExternalPriceID: req.ExternalPriceID, Status: status, ProviderStatus: string(status),
		}
	case ModeInstallments:
		c.ExternalScheduleID = f.nextID("sched")
	case ModePayment:
	}
	f.checkouts[c.ExternalSessionID] = FakeCheckout{Request: req, Checkout: c}
	if req.IdempotencyKey != "" {
		f.byIdempotency[req.IdempotencyKey] = c.ExternalSessionID
	}
	return c, nil
}

func validateCheckout(req CheckoutRequest) error {
	var missing []string
	if req.Reference == uuid.Nil {
		missing = append(missing, "reference")
	}
	if req.Currency == "" {
		missing = append(missing, "currency")
	}
	if req.AmountMinor < 0 {
		missing = append(missing, "amount")
	}
	switch req.Mode {
	case ModePayment:
	case ModeSubscription:
		if req.ExternalPriceID == "" {
			missing = append(missing, "external price")
		}
	case ModeInstallments:
		if req.ExternalPriceID == "" {
			missing = append(missing, "external price")
		}
		if len(req.Schedule) < 2 {
			missing = append(missing, "schedule")
		}
	default:
		missing = append(missing, "mode")
	}
	if len(missing) > 0 {
		return fmt.Errorf("fake: checkout needs %s: %w", strings.Join(missing, ", "), ErrInvalidRequest)
	}
	return nil
}

// ChangeSubscription implements Gateway. The fake never prorates.
func (f *Fake) ChangeSubscription(_ context.Context, req ChangeRequest) (Subscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.takeFailure(); err != nil {
		return Subscription{}, err
	}
	s, ok := f.subscriptions[req.ExternalSubscriptionID]
	if !ok || s.Status == SubscriptionCanceled {
		return Subscription{}, fmt.Errorf("fake: subscription %q: %w", req.ExternalSubscriptionID, ErrNotFound)
	}
	s.ExternalPriceID = req.ExternalPriceID
	f.subscriptions[s.ExternalID] = s
	return s, nil
}

// CancelSubscription implements Gateway.
func (f *Fake) CancelSubscription(_ context.Context, req CancelRequest) (Subscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.takeFailure(); err != nil {
		return Subscription{}, err
	}
	s, ok := f.subscriptions[req.ExternalSubscriptionID]
	if !ok || s.Status == SubscriptionCanceled {
		return Subscription{}, fmt.Errorf("fake: subscription %q: %w", req.ExternalSubscriptionID, ErrNotFound)
	}
	switch req.Mode {
	case CancelNow:
		now := f.now().UTC()
		s.Status, s.ProviderStatus, s.CanceledAt, s.CancelAtPeriodEnd = SubscriptionCanceled, "canceled", &now, false
	case CancelAtPeriodEnd:
		s.CancelAtPeriodEnd = true
	case CancelUndo:
		s.CancelAtPeriodEnd = false
	default:
		return Subscription{}, fmt.Errorf("fake: cancel mode %q: %w", req.Mode, ErrInvalidRequest)
	}
	f.subscriptions[s.ExternalID] = s
	return s, nil
}

// Refund implements Gateway. Refunds succeed at once.
func (f *Fake) Refund(_ context.Context, req RefundRequest) (Refund, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.takeFailure(); err != nil {
		return Refund{}, err
	}
	if req.ExternalPaymentID == "" || req.AmountMinor <= 0 {
		return Refund{}, fmt.Errorf("fake: refund needs a payment and an amount: %w", ErrInvalidRequest)
	}
	r := Refund{
		ExternalID: f.nextID("re"), ExternalPaymentID: req.ExternalPaymentID,
		AmountMinor: req.AmountMinor, Currency: req.Currency, Status: RefundSucceeded,
	}
	f.refunds = append(f.refunds, r)
	return r, nil
}

// CreateCoupon implements Gateway.
func (f *Fake) CreateCoupon(_ context.Context, req CouponRequest) (Coupon, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.takeFailure(); err != nil {
		return Coupon{}, err
	}
	if !f.caps.Coupons {
		return Coupon{}, fmt.Errorf("fake: coupons: %w", ErrUnsupported)
	}
	if (req.PercentBps > 0) == (req.AmountOffMinor > 0) {
		return Coupon{}, fmt.Errorf("fake: coupon needs a percent or an amount: %w", ErrInvalidRequest)
	}
	f.coupons = append(f.coupons, req)
	return Coupon{ExternalCouponID: f.nextID("coupon")}, nil
}

// ParseWebhook implements Gateway. The body is an Event as JSON, signed by Webhook.
func (f *Fake) ParseWebhook(_ context.Context, header http.Header, body []byte) (Event, error) {
	f.mu.Lock()
	now := f.now()
	f.mu.Unlock()
	if !f.verify(header.Get(FakeSignatureHeader), body, now) {
		return Event{}, fmt.Errorf("fake: %w", ErrInvalidSignature)
	}
	var ev Event
	if err := json.Unmarshal(body, &ev); err != nil {
		return Event{}, fmt.Errorf("fake: decode event: %w", ErrInvalidRequest)
	}
	if ev.ExternalID == "" || ev.Type == "" {
		return Event{}, fmt.Errorf("fake: event needs an id and a type: %w", ErrInvalidRequest)
	}
	ev.Provider = f.provider
	ev.Payload = json.RawMessage(slices.Clone(body))
	return ev, nil
}

func (f *Fake) verify(sig string, body []byte, now time.Time) bool {
	var ts, mac string
	for part := range strings.SplitSeq(sig, ",") {
		k, v, _ := strings.Cut(part, "=")
		switch k {
		case "t":
			ts = v
		case "v1":
			mac = v
		}
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false
	}
	if age := now.Sub(time.Unix(sec, 0)); age > fakeTolerance || age < -fakeTolerance {
		return false
	}
	want, err := hex.DecodeString(mac)
	return err == nil && hmac.Equal(want, f.sign(ts, body))
}

func (f *Fake) sign(ts string, body []byte) []byte {
	h := hmac.New(sha256.New, f.secret)
	h.Write([]byte(ts + "."))
	h.Write(body)
	return h.Sum(nil)
}

// Webhook signs ev as the fake provider would deliver it. Provider and
// Payload are ignored; ParseWebhook fills them in.
func (f *Fake) Webhook(ev Event) (http.Header, []byte, error) {
	f.mu.Lock()
	if ev.ExternalID == "" {
		ev.ExternalID = f.nextID("evt")
	}
	now := f.now()
	f.mu.Unlock()
	if ev.OccurredAt.IsZero() {
		ev.OccurredAt = now.UTC()
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return nil, nil, fmt.Errorf("fake: encode event: %w", err)
	}
	ts := strconv.FormatInt(now.Unix(), 10)
	h := http.Header{}
	h.Set(FakeSignatureHeader, "t="+ts+",v1="+hex.EncodeToString(f.sign(ts, body)))
	h.Set("Content-Type", "application/json")
	return h, body, nil
}

// CompleteCheckout pays a checkout the fake started and returns the event the
// provider would send: checkout.completed with the payment. A subscription
// becomes active for one month (or trialing, with TrialDays).
func (f *Fake) CompleteCheckout(externalSessionID string) (Event, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fc, ok := f.checkouts[externalSessionID]
	if !ok {
		return Event{}, fmt.Errorf("fake: checkout %q: %w", externalSessionID, ErrNotFound)
	}
	now := f.now().UTC()
	ref := fc.Request.Reference
	ev := Event{
		ExternalID: f.nextID("evt"), Type: EventCheckoutCompleted, ProviderType: "checkout.completed",
		OccurredAt: now, Reference: &ref,
		Payment: &Payment{
			ExternalID: f.nextID("pay"), InvoiceNumber: f.nextID("inv"),
			ExternalCustomerID:     fc.Checkout.ExternalCustomerID,
			ExternalSubscriptionID: fc.Checkout.ExternalSubscriptionID,
			ExternalScheduleID:     fc.Checkout.ExternalScheduleID,
			SubtotalMinor:          fc.Request.AmountMinor, TotalMinor: fc.Request.AmountMinor,
			Currency: fc.Request.Currency, PaidAt: &now,
		},
	}
	if id := fc.Checkout.ExternalSubscriptionID; id != "" {
		s := f.subscriptions[id]
		end := now.AddDate(0, 1, 0)
		s.Status, s.CurrentPeriodStart, s.CurrentPeriodEnd = SubscriptionActive, &now, &end
		if d := fc.Request.TrialDays; d > 0 {
			end = now.AddDate(0, 0, d)
			s.Status, s.TrialEnd, s.CurrentPeriodEnd = SubscriptionTrialing, &end, &end
		}
		s.ProviderStatus = string(s.Status)
		f.subscriptions[id] = s
		ev.Subscription = &s
		ev.Payment.PeriodStart, ev.Payment.PeriodEnd = s.CurrentPeriodStart, s.CurrentPeriodEnd
	}
	return ev, nil
}

// Checkout returns a checkout the fake started.
func (f *Fake) Checkout(externalSessionID string) (FakeCheckout, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.checkouts[externalSessionID]
	return c, ok
}

// Subscription returns the fake's view of a subscription.
func (f *Fake) Subscription(externalID string) (Subscription, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.subscriptions[externalID]
	return s, ok
}

// Refunds returns every refund made, oldest first.
func (f *Fake) Refunds() []Refund {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.refunds)
}

// Coupons returns every coupon created, oldest first.
func (f *Fake) Coupons() []CouponRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.coupons)
}
