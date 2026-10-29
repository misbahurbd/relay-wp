// Package payment talks to payment providers (Stripe, Creem; PayPal and
// SSLCommerz later). PAYMENT_PROVIDERS chooses which are enabled; New builds
// them once at startup into a Registry (ADR 0006). This package is
// infrastructure only: plans, grants and workspaces belong to features/billing.
//
// Provider secrets stay inside the gateways. Nothing here is stored, and a
// webhook's raw payload and a checkout's client secret are never logged.
package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

// Provider names a payment provider. The values match the provider column of every billing table.
type Provider string

// Providers the schema knows. Only Stripe and Creem have gateways today.
const (
	Stripe     Provider = "stripe"
	Creem      Provider = "creem"
	PayPal     Provider = "paypal"
	SSLCommerz Provider = "sslcommerz"
)

// ParseProvider returns the provider named s.
func ParseProvider(s string) (Provider, bool) {
	switch p := Provider(s); p {
	case Stripe, Creem, PayPal, SSLCommerz:
		return p, true
	default:
		return "", false
	}
}

// Errors a gateway returns. Wrap them; callers match with errors.Is.
var (
	// ErrNotImplemented: the gateway exists but this call isn't built yet.
	ErrNotImplemented = errors.New("payment: not implemented")
	// ErrUnsupported: the provider can't do this (see Capabilities).
	ErrUnsupported = errors.New("payment: not supported by this provider")
	// ErrInvalidSignature: a webhook failed verification. Store nothing.
	ErrInvalidSignature = errors.New("payment: invalid webhook signature")
	// ErrInvalidRequest: the request is missing something the provider needs.
	ErrInvalidRequest = errors.New("payment: invalid request")
	// ErrNotFound: the provider has no such object.
	ErrNotFound = errors.New("payment: not found")
	// ErrDeclined: the provider refused the payment, refund or change.
	ErrDeclined = errors.New("payment: declined")
	// ErrUnavailable: the provider didn't answer or answered 5xx. Retry later.
	ErrUnavailable = errors.New("payment: provider unavailable")
)

// Gateway is one payment provider. Implementations are safe for concurrent use.
type Gateway interface {
	// Provider is the name rows are stored under.
	Provider() Provider
	// Capabilities says what this provider can do, so callers never switch on its name.
	Capabilities() Capabilities

	// CreateCheckout starts a payment for one of our checkout sessions.
	// req.Reference (our checkout_sessions.id) is sent to the provider and
	// comes back on every event about it.
	CreateCheckout(ctx context.Context, req CheckoutRequest) (Checkout, error)
	// ChangeSubscription moves a provider-renewed subscription to another price.
	ChangeSubscription(ctx context.Context, req ChangeRequest) (Subscription, error)
	// CancelSubscription cancels now, at the period end, or undoes a cancel at the period end.
	CancelSubscription(ctx context.Context, req CancelRequest) (Subscription, error)
	// CancelSchedule stops a provider-run installment schedule
	// (Checkout.ExternalScheduleID) so it charges nothing more: the order was
	// upgraded or defaulted. ErrUnsupported without Capabilities.Installments.
	CancelSchedule(ctx context.Context, externalScheduleID string) error
	// Refund refunds all or part of a payment.
	Refund(ctx context.Context, req RefundRequest) (Refund, error)
	// CreateCoupon mirrors a Relay discount as a provider coupon, for
	// subscriptions the provider renews. One-time payments never need one:
	// Relay charges the reduced amount.
	CreateCoupon(ctx context.Context, req CouponRequest) (Coupon, error)

	// ParseWebhook verifies a webhook delivery and normalises it. A bad or
	// missing signature returns ErrInvalidSignature. A verified event Relay
	// doesn't act on has Type EventIgnored.
	ParseWebhook(ctx context.Context, header http.Header, body []byte) (Event, error)
}

// Capabilities describes what a provider supports.
type Capabilities struct {
	// EmbeddedCheckout: CreateCheckout returns a ClientSecret for an in-app
	// form. Otherwise it returns a URL to the provider's hosted page.
	EmbeddedCheckout bool `json:"embedded_checkout"`
	// EmbeddableURL: the hosted page at Checkout.URL may be shown in an
	// iframe inside Relay (Creem's embed.js) instead of a full redirect.
	EmbeddableURL bool `json:"embeddable_url"`
	// RenewsSubscriptions: the provider charges each period itself
	// (renewal_mode provider). Otherwise Relay renews with a River job (app).
	RenewsSubscriptions bool `json:"renews_subscriptions"`
	// Installments: the provider runs an N-payment schedule itself
	// (collection provider_schedule). Otherwise installments use app_link.
	Installments bool `json:"installments"`
	// Coupons: CreateCoupon works, so discounts carry over to renewals.
	Coupons bool `json:"coupons"`
	// PartialRefunds: Refund accepts less than the payment's total.
	PartialRefunds bool `json:"partial_refunds"`
	// MerchantOfRecord: the provider charges and reports tax (Creem).
	MerchantOfRecord bool `json:"merchant_of_record"`
	// Currencies the provider settles in, ISO 4217 upper case.
	Currencies []string `json:"currencies"`
	// PublishableKey is safe for the browser (Stripe's publishable key), or empty.
	PublishableKey string `json:"publishable_key,omitempty"`
}

// CheckoutMode says what a checkout pays for.
type CheckoutMode string

// Checkout modes.
const (
	// ModePayment charges AmountMinor once: a lifetime deal in full, one
	// app_link installment, or a defaulted order's remaining balance.
	ModePayment CheckoutMode = "payment"
	// ModeSubscription starts a subscription on ExternalPriceID that the provider renews.
	ModeSubscription CheckoutMode = "subscription"
	// ModeInstallments starts a provider-run schedule of len(Schedule)
	// payments on ExternalPriceID (collection provider_schedule).
	ModeInstallments CheckoutMode = "installments"
)

// CheckoutRequest starts a checkout. Amounts are integer minor units.
type CheckoutRequest struct {
	// Reference is our checkout_sessions.id. Webhooks map back to a workspace only through it.
	Reference      uuid.UUID
	IdempotencyKey string
	Mode           CheckoutMode
	// ExternalPriceID is provider_prices.external_price_id. Subscription and
	// installment modes always need it; payment mode needs it on a provider
	// that charges a one-time product (Creem), and ignores it otherwise.
	ExternalPriceID string
	// AmountMinor is what the customer pays now, after any discount and credit.
	AmountMinor int64
	Currency    string
	// Schedule is every installment's amount, in order, the first one included
	// (installment mode). It adds up to the order's balance.
	Schedule []int64
	// ExternalCouponID is provider_discounts.external_coupon_id, for a discounted subscription.
	ExternalCouponID string
	// TrialDays starts a subscription in trialing (a trial that needs a card).
	TrialDays int
	Customer  Customer
	// Description is shown to the customer, e.g. "Growth, monthly".
	Description string
	// ReturnURL is where a hosted checkout or a 3-D Secure step sends the customer back.
	ReturnURL string
}

// Customer is who pays, prefilled from billing_profiles. Personal data: never log it.
type Customer struct {
	// ExternalID is billing_customers.external_customer_id; empty asks the provider to create one.
	ExternalID  string
	WorkspaceID uuid.UUID
	Name        string
	Email       string
	Phone       string
	Address     Address
	TaxIDType   string
	TaxID       string
}

// Address is a postal address; Country is ISO 3166-1 alpha-2.
type Address struct {
	Line1      string
	Line2      string
	City       string
	State      string
	PostalCode string
	Country    string
}

// LogValue keeps personal data out of logs.
func (c Customer) LogValue() slog.Value {
	return slog.GroupValue(slog.String("external_id", c.ExternalID), slog.String("workspace_id", c.WorkspaceID.String()))
}

// Checkout is a started checkout.
type Checkout struct {
	// ExternalSessionID is stored in checkout_sessions.external_session_id.
	ExternalSessionID string
	// ExternalCustomerID is stored in billing_customers.external_customer_id.
	ExternalCustomerID string
	// ExternalSubscriptionID is set when the provider creates the subscription at once (Stripe).
	ExternalSubscriptionID string
	// ExternalScheduleID is set for installment mode; stored in orders.external_ref.
	ExternalScheduleID string
	// ClientSecret goes straight to the browser for an embedded form. Never store or log it.
	ClientSecret string
	// URL is the provider's hosted checkout page, when the provider has no embedded form.
	URL       string
	ExpiresAt time.Time
}

// LogValue keeps the client secret out of logs.
func (c Checkout) LogValue() slog.Value {
	return slog.GroupValue(slog.String("external_session_id", c.ExternalSessionID),
		slog.String("external_subscription_id", c.ExternalSubscriptionID))
}

// ChangeRequest moves a subscription to another price.
type ChangeRequest struct {
	ExternalSubscriptionID string
	ExternalPriceID        string
	// ProrateNow charges or credits the difference now (an upgrade). False
	// starts the new price at the next period (a downgrade).
	ProrateNow     bool
	IdempotencyKey string
}

// CancelMode says how CancelSubscription cancels.
type CancelMode string

// Cancel modes.
const (
	CancelNow         CancelMode = "now"
	CancelAtPeriodEnd CancelMode = "at_period_end"
	// CancelUndo keeps a subscription that was set to cancel at the period end.
	CancelUndo CancelMode = "undo"
)

// CancelRequest cancels a subscription.
type CancelRequest struct {
	ExternalSubscriptionID string
	Mode                   CancelMode
}

// SubscriptionStatus is the normalised status, the same values as subscriptions.status.
type SubscriptionStatus string

// Subscription statuses.
const (
	SubscriptionIncomplete SubscriptionStatus = "incomplete"
	SubscriptionTrialing   SubscriptionStatus = "trialing"
	SubscriptionActive     SubscriptionStatus = "active"
	SubscriptionPastDue    SubscriptionStatus = "past_due"
	SubscriptionCanceled   SubscriptionStatus = "canceled"
)

// Subscription is the provider's view of a subscription.
type Subscription struct {
	ExternalID         string             `json:"external_id"`
	ExternalCustomerID string             `json:"external_customer_id,omitempty"`
	ExternalPriceID    string             `json:"external_price_id,omitempty"`
	Status             SubscriptionStatus `json:"status"`
	// ProviderStatus is the provider's own word for it, kept for support.
	ProviderStatus     string     `json:"provider_status,omitempty"`
	CurrentPeriodStart *time.Time `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   *time.Time `json:"current_period_end,omitempty"`
	TrialEnd           *time.Time `json:"trial_end,omitempty"`
	CancelAtPeriodEnd  bool       `json:"cancel_at_period_end"`
	CanceledAt         *time.Time `json:"canceled_at,omitempty"`
	// ProrationMinor is what ChangeSubscription charged (positive) or credited (negative) now.
	ProrationMinor int64 `json:"proration_minor,omitempty"`
}

// RefundRequest refunds a payment.
type RefundRequest struct {
	// ExternalPaymentID is invoices.external_payment_id.
	ExternalPaymentID string
	AmountMinor       int64
	Currency          string
	Reason            string
	IdempotencyKey    string
}

// RefundStatus is the normalised status, the same values as refunds.status.
type RefundStatus string

// Refund statuses.
const (
	RefundPending   RefundStatus = "pending"
	RefundSucceeded RefundStatus = "succeeded"
	RefundFailed    RefundStatus = "failed"
)

// Refund is a refund as the provider reports it.
type Refund struct {
	ExternalID        string       `json:"external_id"`
	ExternalPaymentID string       `json:"external_payment_id"`
	AmountMinor       int64        `json:"amount_minor"`
	Currency          string       `json:"currency"`
	Status            RefundStatus `json:"status"`
}

// CouponRequest mirrors a Relay discount. Exactly one of PercentBps and AmountOffMinor is set.
type CouponRequest struct {
	DiscountID     uuid.UUID
	Code           string
	Name           string
	PercentBps     int
	AmountOffMinor int64
	Currency       string
	// Duration is once, repeating (for DurationMonths) or forever.
	Duration       string
	DurationMonths int
	// ExternalProductIDs are the provider prices the coupon applies to, for a
	// provider that scopes coupons to products (Creem). Empty means every
	// active recurring product.
	ExternalProductIDs []string
}

// Coupon is a provider coupon; stored in provider_discounts.
type Coupon struct {
	ExternalCouponID        string
	ExternalPromotionCodeID string
}

// EventType is a normalised webhook event. Billing dispatches on it.
type EventType string

// Event types.
const (
	// EventCheckoutCompleted: a checkout's first payment went through. Payment is set.
	EventCheckoutCompleted EventType = "checkout.completed"
	// EventCheckoutFailed: the first payment failed for good, not one declined
	// attempt the customer can retry. Payment may be set.
	EventCheckoutFailed EventType = "checkout.failed"
	// EventCheckoutExpired: the customer never paid.
	EventCheckoutExpired EventType = "checkout.expired"
	// EventPaymentSucceeded: a later payment went through (a renewal or an installment). Payment is set.
	EventPaymentSucceeded EventType = "payment.succeeded"
	// EventPaymentFailed: a later payment failed. Payment is set.
	EventPaymentFailed EventType = "payment.failed"
	// EventSubscriptionUpdated: status, period or cancel-at-period-end changed. Subscription is set.
	EventSubscriptionUpdated EventType = "subscription.updated"
	// EventSubscriptionCanceled: the subscription ended. Subscription is set.
	EventSubscriptionCanceled EventType = "subscription.canceled"
	// EventRefundUpdated: a refund was created or changed status. Refund is set.
	EventRefundUpdated EventType = "refund.updated"
	// EventDisputeOpened: the customer disputed a payment. Payment is set.
	EventDisputeOpened EventType = "dispute.opened"
	// EventDisputeClosed: a dispute was resolved. Payment is set; Payment.Status says how.
	EventDisputeClosed EventType = "dispute.closed"
	// EventIgnored: verified, but nothing Relay acts on. Stored, then marked processed.
	EventIgnored EventType = "ignored"
)

// Event is one verified webhook, normalised. Everything but Payload is safe to
// put in job arguments.
type Event struct {
	Provider Provider `json:"provider"`
	// ExternalID is the provider's event ID; payment_events dedupes on it.
	ExternalID string    `json:"external_id"`
	Type       EventType `json:"type"`
	// ProviderType is the provider's own event name, e.g. "invoice.paid".
	ProviderType string    `json:"provider_type"`
	OccurredAt   time.Time `json:"occurred_at"`
	// Reference is our checkout_sessions.id, when the event carries it.
	Reference    *uuid.UUID    `json:"reference,omitempty"`
	Payment      *Payment      `json:"payment,omitempty"`
	Subscription *Subscription `json:"subscription,omitempty"`
	Refund       *Refund       `json:"refund,omitempty"`
	// Payload is the raw verified body, for payment_events.payload only.
	// Never log it or return it.
	Payload json.RawMessage `json:"-"`
}

// LogValue keeps the payload out of logs.
func (e Event) LogValue() slog.Value {
	return slog.GroupValue(slog.String("provider", string(e.Provider)), slog.String("external_id", e.ExternalID),
		slog.String("type", string(e.Type)), slog.String("provider_type", e.ProviderType))
}

// Payment is a charge as the provider reports it. Amounts are integer minor
// units and TotalMinor = SubtotalMinor - DiscountMinor + TaxMinor.
type Payment struct {
	// ExternalID is stored in invoices.external_payment_id.
	ExternalID string `json:"external_id"`
	// InvoiceNumber is the provider's invoice number (Q19), if it issued one.
	InvoiceNumber          string     `json:"invoice_number,omitempty"`
	ExternalCustomerID     string     `json:"external_customer_id,omitempty"`
	ExternalSubscriptionID string     `json:"external_subscription_id,omitempty"`
	ExternalScheduleID     string     `json:"external_schedule_id,omitempty"`
	SubtotalMinor          int64      `json:"subtotal_minor"`
	DiscountMinor          int64      `json:"discount_minor"`
	TaxMinor               int64      `json:"tax_minor"`
	TotalMinor             int64      `json:"total_minor"`
	Currency               string     `json:"currency"`
	PeriodStart            *time.Time `json:"period_start,omitempty"`
	PeriodEnd              *time.Time `json:"period_end,omitempty"`
	ReceiptURL             string     `json:"receipt_url,omitempty"`
	PaidAt                 *time.Time `json:"paid_at,omitempty"`
	// Status is set for disputes: "won" or "lost" when closed.
	Status string `json:"status,omitempty"`
	// FailureCode is the provider's reason for a failed payment.
	FailureCode string `json:"failure_code,omitempty"`
}

// Registry holds the enabled gateways, keyed by provider. A nil Registry has none.
type Registry struct {
	order    []Provider
	gateways map[Provider]Gateway
}

// NewRegistry builds a registry from gateways that are already built. Tests use it with fakes.
func NewRegistry(gateways ...Gateway) (*Registry, error) {
	r := &Registry{gateways: make(map[Provider]Gateway, len(gateways))}
	for _, g := range gateways {
		p := g.Provider()
		if _, dup := r.gateways[p]; dup {
			return nil, fmt.Errorf("payment: %s is enabled twice", p)
		}
		r.order = append(r.order, p)
		r.gateways[p] = g
	}
	return r, nil
}

// Get returns the gateway for p, or false when p isn't enabled.
func (r *Registry) Get(p Provider) (Gateway, bool) {
	if r == nil {
		return nil, false
	}
	g, ok := r.gateways[p]
	return g, ok
}

// Providers lists the enabled providers in PAYMENT_PROVIDERS order.
func (r *Registry) Providers() []Provider {
	if r == nil {
		return nil
	}
	return append([]Provider(nil), r.order...)
}

// fakeSuffix registers the fake gateway under a provider's name: "stripe:fake".
const fakeSuffix = ":fake"

// New builds a gateway for every provider in cfg.Providers, once at startup.
// An unknown or duplicate provider, or one with missing keys, is an error.
// "<provider>:fake" registers the fake gateway and is allowed only when dev is true.
func New(cfg config.Payment, dev bool) (*Registry, error) {
	// One client, so every gateway shares one connection pool.
	client := &http.Client{Timeout: 20 * time.Second}
	gateways := make([]Gateway, 0, len(cfg.Providers))
	for _, raw := range cfg.Providers {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		name, fake := strings.CutSuffix(entry, fakeSuffix)
		p, ok := ParseProvider(name)
		if !ok {
			return nil, fmt.Errorf("unknown payment provider %q in PAYMENT_PROVIDERS (want stripe or creem)", entry)
		}
		if fake {
			if !dev {
				return nil, fmt.Errorf("PAYMENT_PROVIDERS: %q is only allowed in development", entry)
			}
			gateways = append(gateways, NewFake(p, DevFakeWebhookSecret))
			continue
		}
		var (
			g   Gateway
			err error
		)
		switch p {
		case Stripe:
			g, err = newStripe(cfg, client)
		case Creem:
			g, err = newCreem(cfg, client)
		default:
			err = fmt.Errorf("payment provider %q has no gateway yet", p)
		}
		if err != nil {
			return nil, err
		}
		gateways = append(gateways, g)
	}
	return NewRegistry(gateways...)
}
