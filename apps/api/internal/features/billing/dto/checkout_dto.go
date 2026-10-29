package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

type CreateCheckoutInput struct {
	WorkspaceID    uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	IdempotencyKey string    `header:"Idempotency-Key" required:"true" minLength:"8" maxLength:"255" doc:"A retry with the same key returns the same checkout"`
	Body           struct {
		PlanPriceID        uuid.UUID  `json:"plan_price_id"`
		InstallmentOfferID *uuid.UUID `json:"installment_offer_id,omitempty" doc:"Pay a one-time price in installments"`
		Provider           string     `json:"provider,omitempty" enum:"stripe,creem,paypal,sslcommerz" doc:"Omit to use the first provider that sells this price"`
		DiscountCode       string     `json:"discount_code,omitempty" maxLength:"40"`
		ReturnURL          string     `json:"return_url,omitempty" format:"uri" maxLength:"2048" doc:"Where a hosted checkout or 3-D Secure returns to"`
	}
}

func (in *CreateCheckoutInput) ToModel() model.CheckoutInput {
	return model.CheckoutInput{
		PlanPriceID: in.Body.PlanPriceID, InstallmentOfferID: in.Body.InstallmentOfferID,
		Provider: payment.Provider(in.Body.Provider), DiscountCode: in.Body.DiscountCode,
		IdempotencyKey: in.IdempotencyKey, ReturnURL: in.Body.ReturnURL,
	}
}

type CheckoutSessionInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	SessionID   uuid.UUID `path:"sessionId"`
}

type CheckoutSessionDTO struct {
	ID                 uuid.UUID  `json:"id"`
	PlanPriceID        uuid.UUID  `json:"plan_price_id"`
	InstallmentOfferID *uuid.UUID `json:"installment_offer_id"`
	Purpose            string     `json:"purpose" enum:"new,renewal,change,ltd,installment"`
	SubscriptionID     *uuid.UUID `json:"subscription_id"`
	OrderID            *uuid.UUID `json:"order_id"`
	InstallmentID      *uuid.UUID `json:"installment_id"`
	Provider           string     `json:"provider" enum:"stripe,creem,paypal,sslcommerz"`
	Status             string     `json:"status" enum:"pending,completed,failed,expired"`
	AmountMinor        int64      `json:"amount_minor" doc:"Charged now, after discount and credit"`
	DiscountMinor      int64      `json:"discount_minor"`
	Currency           string     `json:"currency"`
	ExpiresAt          time.Time  `json:"expires_at"`
	CreatedAt          time.Time  `json:"created_at"`
}

func ToCheckoutSessionDTO(c model.CheckoutSession) CheckoutSessionDTO {
	return CheckoutSessionDTO{
		ID: c.ID, PlanPriceID: c.PlanPriceID, InstallmentOfferID: c.InstallmentOfferID, Purpose: c.Purpose,
		SubscriptionID: c.SubscriptionID, OrderID: c.OrderID, InstallmentID: c.InstallmentID,
		Provider: string(c.Provider), Status: c.Status, AmountMinor: c.AmountMinor, DiscountMinor: c.DiscountMinor,
		Currency: c.Currency, ExpiresAt: c.ExpiresAt, CreatedAt: c.CreatedAt,
	}
}

type CheckoutSessionOutput struct {
	Body CheckoutSessionDTO
}

type StartedCheckoutOutput struct {
	Body struct {
		Session        CheckoutSessionDTO `json:"session"`
		ClientSecret   *string            `json:"client_secret" doc:"For the provider's embedded form; shown once, never stored"`
		URL            *string            `json:"url" doc:"The provider's hosted checkout page, when it has no embedded form"`
		PublishableKey *string            `json:"publishable_key" doc:"Browser key for the embedded form (Stripe Elements)"`
	}
}

func ToStartedCheckoutOutput(c model.StartedCheckout) *StartedCheckoutOutput {
	out := &StartedCheckoutOutput{}
	out.Body.Session = ToCheckoutSessionDTO(c.Session)
	out.Body.ClientSecret, out.Body.URL = nonEmpty(c.ClientSecret), nonEmpty(c.URL)
	out.Body.PublishableKey = nonEmpty(c.PublishableKey)
	return out
}
