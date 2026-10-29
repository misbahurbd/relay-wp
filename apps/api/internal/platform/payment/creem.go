package payment

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

// creemGateway is Creem, the merchant of record for its sales.
// Not built yet (track T2): every call returns ErrNotImplemented, and its
// capabilities wait for the "Creem findings" in docs/PHASE-6-BILLING.md.
type creemGateway struct {
	apiKey        string
	webhookSecret string
	client        *http.Client
}

func newCreem(cfg config.Payment, client *http.Client) (*creemGateway, error) {
	if cfg.CreemAPIKey == "" || cfg.CreemWebhookSecret == "" {
		return nil, errors.New("CREEM_API_KEY and CREEM_WEBHOOK_SECRET are required when PAYMENT_PROVIDERS includes creem")
	}
	return &creemGateway{apiKey: cfg.CreemAPIKey, webhookSecret: cfg.CreemWebhookSecret, client: client}, nil
}

func (g *creemGateway) Provider() Provider { return Creem }

func (g *creemGateway) Capabilities() Capabilities {
	return Capabilities{MerchantOfRecord: true, Currencies: []string{"USD"}}
}

func (g *creemGateway) CreateCheckout(context.Context, CheckoutRequest) (Checkout, error) {
	return Checkout{}, fmt.Errorf("creem: create checkout: %w", ErrNotImplemented)
}

func (g *creemGateway) ChangeSubscription(context.Context, ChangeRequest) (Subscription, error) {
	return Subscription{}, fmt.Errorf("creem: change subscription: %w", ErrNotImplemented)
}

func (g *creemGateway) CancelSubscription(context.Context, CancelRequest) (Subscription, error) {
	return Subscription{}, fmt.Errorf("creem: cancel subscription: %w", ErrNotImplemented)
}

func (g *creemGateway) Refund(context.Context, RefundRequest) (Refund, error) {
	return Refund{}, fmt.Errorf("creem: refund: %w", ErrNotImplemented)
}

func (g *creemGateway) CreateCoupon(context.Context, CouponRequest) (Coupon, error) {
	return Coupon{}, fmt.Errorf("creem: create coupon: %w", ErrNotImplemented)
}

func (g *creemGateway) ParseWebhook(context.Context, http.Header, []byte) (Event, error) {
	return Event{}, fmt.Errorf("creem: parse webhook: %w", ErrNotImplemented)
}
