package payment

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

// stripeGateway is Stripe with embedded checkout through Elements.
// Not built yet (track T1): every call returns ErrNotImplemented.
type stripeGateway struct {
	secretKey      string
	publishableKey string
	webhookSecret  string
	client         *http.Client
}

func newStripe(cfg config.Payment, client *http.Client) (*stripeGateway, error) {
	if cfg.StripeSecretKey == "" || cfg.StripePublishableKey == "" || cfg.StripeWebhookSecret == "" {
		return nil, errors.New("STRIPE_SECRET_KEY, STRIPE_PUBLISHABLE_KEY and STRIPE_WEBHOOK_SECRET are required " +
			"when PAYMENT_PROVIDERS includes stripe")
	}
	return &stripeGateway{
		secretKey: cfg.StripeSecretKey, publishableKey: cfg.StripePublishableKey,
		webhookSecret: cfg.StripeWebhookSecret, client: client,
	}, nil
}

func (g *stripeGateway) Provider() Provider { return Stripe }

func (g *stripeGateway) Capabilities() Capabilities {
	return Capabilities{
		EmbeddedCheckout: true, RenewsSubscriptions: true, Installments: true, Coupons: true,
		PartialRefunds: true, Currencies: []string{"USD"}, PublishableKey: g.publishableKey,
	}
}

func (g *stripeGateway) CreateCheckout(context.Context, CheckoutRequest) (Checkout, error) {
	return Checkout{}, fmt.Errorf("stripe: create checkout: %w", ErrNotImplemented)
}

func (g *stripeGateway) ChangeSubscription(context.Context, ChangeRequest) (Subscription, error) {
	return Subscription{}, fmt.Errorf("stripe: change subscription: %w", ErrNotImplemented)
}

func (g *stripeGateway) CancelSubscription(context.Context, CancelRequest) (Subscription, error) {
	return Subscription{}, fmt.Errorf("stripe: cancel subscription: %w", ErrNotImplemented)
}

func (g *stripeGateway) Refund(context.Context, RefundRequest) (Refund, error) {
	return Refund{}, fmt.Errorf("stripe: refund: %w", ErrNotImplemented)
}

func (g *stripeGateway) CreateCoupon(context.Context, CouponRequest) (Coupon, error) {
	return Coupon{}, fmt.Errorf("stripe: create coupon: %w", ErrNotImplemented)
}

func (g *stripeGateway) ParseWebhook(context.Context, http.Header, []byte) (Event, error) {
	return Event{}, fmt.Errorf("stripe: parse webhook: %w", ErrNotImplemented)
}
