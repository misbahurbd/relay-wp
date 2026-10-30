package payment_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// The fake must satisfy the interface the billing service holds.
var _ payment.Gateway = (*payment.Fake)(nil)

func newFake(t *testing.T) *payment.Fake {
	t.Helper()
	return payment.NewFake(payment.Stripe, "whsec_test")
}

func subscriptionCheckout() payment.CheckoutRequest {
	return payment.CheckoutRequest{
		Reference: uuid.New(), IdempotencyKey: "k1", Mode: payment.ModeSubscription,
		ExternalPriceID: "price_growth_month", AmountMinor: 2900, Currency: "USD",
	}
}

func TestFakeCheckoutCompletesThroughSignedWebhook(t *testing.T) {
	ctx := context.Background()
	f := newFake(t)
	req := subscriptionCheckout()

	c, err := f.CreateCheckout(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if c.ExternalSessionID == "" || c.ClientSecret == "" || c.ExternalSubscriptionID == "" || c.ExternalCustomerID == "" {
		t.Fatalf("checkout = %+v", c)
	}
	again, err := f.CreateCheckout(ctx, req)
	if err != nil || again.ExternalSessionID != c.ExternalSessionID {
		t.Fatalf("same idempotency key: got %q, %v; want %q", again.ExternalSessionID, err, c.ExternalSessionID)
	}

	ev, err := f.CompleteCheckout(c.ExternalSessionID)
	if err != nil {
		t.Fatal(err)
	}
	header, body, err := f.Webhook(ev)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.ParseWebhook(ctx, header, body)
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != payment.Stripe || got.Type != payment.EventCheckoutCompleted || got.ExternalID != ev.ExternalID {
		t.Fatalf("event = %+v", got)
	}
	if got.Reference == nil || *got.Reference != req.Reference {
		t.Fatalf("reference = %v, want %v", got.Reference, req.Reference)
	}
	if got.Payment == nil || got.Payment.TotalMinor != 2900 || got.Payment.Currency != "USD" {
		t.Fatalf("payment = %+v", got.Payment)
	}
	if got.Subscription == nil || got.Subscription.Status != payment.SubscriptionActive ||
		got.Subscription.CurrentPeriodEnd == nil {
		t.Fatalf("subscription = %+v", got.Subscription)
	}
	if !bytes.Equal(got.Payload, body) {
		t.Error("payload should be the raw verified body")
	}
}

func TestFakeRejectsTamperedOrStaleWebhooks(t *testing.T) {
	ctx := context.Background()
	f := newFake(t)
	header, body, err := f.Webhook(payment.Event{Type: payment.EventPaymentSucceeded})
	if err != nil {
		t.Fatal(err)
	}

	tampered := bytes.Replace(body, []byte("payment.succeeded"), []byte("payment.failed"), 1)
	if _, err := f.ParseWebhook(ctx, header, tampered); !errors.Is(err, payment.ErrInvalidSignature) {
		t.Errorf("tampered body: err = %v", err)
	}
	if _, err := f.ParseWebhook(ctx, nil, body); !errors.Is(err, payment.ErrInvalidSignature) {
		t.Errorf("no signature: err = %v", err)
	}
	other := payment.NewFake(payment.Stripe, "another_secret")
	if _, err := other.ParseWebhook(ctx, header, body); !errors.Is(err, payment.ErrInvalidSignature) {
		t.Errorf("wrong secret: err = %v", err)
	}
	f.SetClock(func() time.Time { return time.Now().Add(time.Hour) })
	if _, err := f.ParseWebhook(ctx, header, body); !errors.Is(err, payment.ErrInvalidSignature) {
		t.Errorf("stale signature: err = %v", err)
	}
}

func TestFakeCancelModes(t *testing.T) {
	ctx := context.Background()
	f := newFake(t)
	c, err := f.CreateCheckout(ctx, subscriptionCheckout())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.CompleteCheckout(c.ExternalSessionID); err != nil {
		t.Fatal(err)
	}
	id := c.ExternalSubscriptionID

	s, err := f.CancelSubscription(ctx, payment.CancelRequest{ExternalSubscriptionID: id, Mode: payment.CancelAtPeriodEnd})
	if err != nil || !s.CancelAtPeriodEnd || s.Status != payment.SubscriptionActive {
		t.Fatalf("at period end: %+v, %v", s, err)
	}
	s, err = f.CancelSubscription(ctx, payment.CancelRequest{ExternalSubscriptionID: id, Mode: payment.CancelUndo})
	if err != nil || s.CancelAtPeriodEnd {
		t.Fatalf("undo: %+v, %v", s, err)
	}
	s, err = f.CancelSubscription(ctx, payment.CancelRequest{ExternalSubscriptionID: id, Mode: payment.CancelNow})
	if err != nil || s.Status != payment.SubscriptionCanceled || s.CanceledAt == nil {
		t.Fatalf("now: %+v, %v", s, err)
	}
	if _, err := f.CancelSubscription(ctx, payment.CancelRequest{ExternalSubscriptionID: id, Mode: payment.CancelNow}); !errors.Is(err, payment.ErrNotFound) {
		t.Errorf("cancel twice: err = %v", err)
	}
	if _, err := f.ChangeSubscription(ctx, payment.ChangeRequest{ExternalSubscriptionID: id, ExternalPriceID: "x"}); !errors.Is(err, payment.ErrNotFound) {
		t.Errorf("change a canceled subscription: err = %v", err)
	}
}

func TestFakeRefundsCouponsAndFailures(t *testing.T) {
	ctx := context.Background()
	f := newFake(t)

	r, err := f.Refund(ctx, payment.RefundRequest{ExternalPaymentID: "pay_1", AmountMinor: 500, Currency: "USD"})
	if err != nil || r.Status != payment.RefundSucceeded || r.ExternalID == "" {
		t.Fatalf("refund = %+v, %v", r, err)
	}
	if len(f.Refunds()) != 1 {
		t.Errorf("refunds = %v", f.Refunds())
	}
	if _, err := f.CreateCoupon(ctx, payment.CouponRequest{Code: "X", PercentBps: 2000, AmountOffMinor: 100}); !errors.Is(err, payment.ErrInvalidRequest) {
		t.Errorf("coupon with percent and amount: err = %v", err)
	}
	if c, err := f.CreateCoupon(ctx, payment.CouponRequest{Code: "LAUNCH20", PercentBps: 2000, Duration: "once"}); err != nil || c.ExternalCouponID == "" {
		t.Errorf("coupon = %+v, %v", c, err)
	}

	f.FailNext(payment.ErrUnavailable)
	if _, err := f.CreateCheckout(ctx, subscriptionCheckout()); !errors.Is(err, payment.ErrUnavailable) {
		t.Errorf("FailNext: err = %v", err)
	}
	if _, err := f.CreateCheckout(ctx, payment.CheckoutRequest{Mode: payment.ModeSubscription}); !errors.Is(err, payment.ErrInvalidRequest) {
		t.Errorf("empty request: err = %v", err)
	}
	f.SetCapabilities(payment.Capabilities{EmbeddedCheckout: false})
	req := subscriptionCheckout()
	req.IdempotencyKey = "hosted"
	if c, err := f.CreateCheckout(ctx, req); err != nil || c.URL == "" || c.ClientSecret != "" {
		t.Errorf("hosted checkout = %+v, %v", c, err)
	}
}

func TestLogValuesHideSecrets(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	log.Info("x",
		"checkout", payment.Checkout{ExternalSessionID: "cs_1", ClientSecret: "secret_abc"},
		"event", payment.Event{ExternalID: "evt_1", Payload: []byte(`{"card":"4242"}`)},
		"customer", payment.Customer{ExternalID: "cus_1", Email: "pat@example.test"})
	for _, leaked := range []string{"secret_abc", "4242", "pat@example.test"} {
		if strings.Contains(buf.String(), leaked) {
			t.Errorf("log output contains %q: %s", leaked, buf.String())
		}
	}
}

func TestNewBuildsOnlyConfiguredProviders(t *testing.T) {
	stripeKeys := config.Payment{
		StripeSecretKey: "sk_test", StripePublishableKey: "pk_test", StripeWebhookSecret: "whsec",
	}
	tests := []struct {
		name    string
		cfg     config.Payment
		dev     bool
		want    []payment.Provider
		wantErr string
	}{
		{name: "none", want: nil},
		{name: "unknown", cfg: config.Payment{Providers: []string{"bitpay"}}, wantErr: "unknown payment provider"},
		{name: "half-configured stripe", cfg: config.Payment{Providers: []string{"stripe"}, StripeSecretKey: "sk"}, wantErr: "STRIPE_PUBLISHABLE_KEY"},
		{name: "creem without keys", cfg: config.Payment{Providers: []string{"creem"}}, wantErr: "CREEM_API_KEY"},
		{name: "no gateway yet", cfg: config.Payment{Providers: []string{"paypal"}}, wantErr: "no gateway yet"},
		{name: "fake outside development", cfg: config.Payment{Providers: []string{"stripe:fake"}}, wantErr: "only allowed in development"},
		{name: "fake in development", cfg: config.Payment{Providers: []string{"stripe:fake", " creem:fake"}}, dev: true, want: []payment.Provider{payment.Stripe, payment.Creem}},
		{name: "duplicate", cfg: config.Payment{Providers: []string{"stripe:fake", "stripe:fake"}}, dev: true, wantErr: "enabled twice"},
		{name: "stripe", cfg: func() config.Payment { c := stripeKeys; c.Providers = []string{"stripe"}; return c }(), want: []payment.Provider{payment.Stripe}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, err := payment.New(tt.cfg, tt.dev)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to mention %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got := reg.Providers()
			if len(got) != len(tt.want) {
				t.Fatalf("providers = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("providers = %v, want %v", got, tt.want)
				}
				if _, ok := reg.Get(got[i]); !ok {
					t.Errorf("Get(%s) missing", got[i])
				}
			}
		})
	}
	var nilReg *payment.Registry
	if _, ok := nilReg.Get(payment.Stripe); ok || nilReg.Providers() != nil {
		t.Error("a nil registry has no gateways")
	}
}
