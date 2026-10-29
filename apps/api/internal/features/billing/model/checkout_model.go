package model

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Checkout purposes (checkout_sessions.purpose).
const (
	PurposeNew         = "new"         // a new subscription
	PurposeRenewal     = "renewal"     // an app-renewed subscription's next period
	PurposeChange      = "change"      // a plan change that needs a payment
	PurposeLTD         = "ltd"         // a lifetime deal, in full or installment 1
	PurposeInstallment = "installment" // one app_link installment, or a defaulted order's balance
)

// Checkout session statuses (checkout_sessions.status).
const (
	CheckoutPending   = "pending"
	CheckoutCompleted = "completed"
	CheckoutFailed    = "failed"
	CheckoutExpired   = "expired"
)

// CheckoutInput starts a checkout for a price.
type CheckoutInput struct {
	PlanPriceID        uuid.UUID
	InstallmentOfferID *uuid.UUID
	// Provider empty picks the first enabled provider that sells the price (Q10 deferred).
	Provider       payment.Provider
	DiscountCode   string
	IdempotencyKey string
	// ReturnURL is where a hosted checkout or 3-D Secure sends the customer back.
	ReturnURL string
}

// CheckoutSession is one checkout attempt. Its ID is the provider's reference.
type CheckoutSession struct {
	ID                 uuid.UUID
	WorkspaceID        uuid.UUID
	PlanPriceID        uuid.UUID
	InstallmentOfferID *uuid.UUID
	Purpose            string
	SubscriptionID     *uuid.UUID
	OrderID            *uuid.UUID
	InstallmentID      *uuid.UUID
	Provider           payment.Provider
	ExternalSessionID  *string
	Status             string
	AmountMinor        int64
	Currency           string
	DiscountID         *uuid.UUID
	DiscountMinor      int64
	CreatedBy          *uuid.UUID
	ExpiresAt          time.Time
	CreatedAt          time.Time
}

// StartedCheckout is a checkout the browser can complete.
type StartedCheckout struct {
	Session CheckoutSession
	// ClientSecret is for an embedded form (Stripe Elements). Returned once, never stored.
	ClientSecret string
	// URL is the provider's hosted page, when it has no embedded form.
	URL string
	// PublishableKey is the provider's browser key, if it has one.
	PublishableKey string
}

// PaymentCheckoutInput starts a one-off payment checkout on an existing order:
// an app_link installment, or the balance of a defaulted order (purpose installment).
type PaymentCheckoutInput struct {
	OrderID uuid.UUID
	// InstallmentID is the installment being paid; nil pays the order's balance.
	InstallmentID  *uuid.UUID
	AmountMinor    int64
	Currency       string
	Provider       payment.Provider
	IdempotencyKey string
	Description    string
	ReturnURL      string
}
