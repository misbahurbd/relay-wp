package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

type InstallmentDTO struct {
	ID          uuid.UUID  `json:"id"`
	Seq         int        `json:"seq" doc:"1 is the first payment"`
	DueAt       time.Time  `json:"due_at"`
	AmountMinor int64      `json:"amount_minor"`
	Status      string     `json:"status" enum:"scheduled,due,paid,overdue,failed,waived,canceled"`
	PaidAt      *time.Time `json:"paid_at"`
}

type OrderDTO struct {
	ID                     uuid.UUID        `json:"id"`
	PlanPriceID            uuid.UUID        `json:"plan_price_id"`
	Price                  *PriceDTO        `json:"price,omitempty"`
	InstallmentOfferID     *uuid.UUID       `json:"installment_offer_id"`
	Provider               string           `json:"provider" enum:"stripe,creem,paypal,sslcommerz"`
	Status                 string           `json:"status" enum:"pending,in_installments,paid,defaulted,upgraded,refunded,canceled"`
	TotalMinor             int64            `json:"total_minor"`
	DiscountMinor          int64            `json:"discount_minor"`
	CreditMinor            int64            `json:"credit_minor" doc:"Carried in from a replaced subscription or upgraded order"`
	PaidMinor              int64            `json:"paid_minor"`
	BalanceMinor           int64            `json:"balance_minor" doc:"Still owed"`
	Currency               string           `json:"currency"`
	PaidInFullAt           *time.Time       `json:"paid_in_full_at"`
	DefaultedAt            *time.Time       `json:"defaulted_at"`
	UpgradedFromOrderID    *uuid.UUID       `json:"upgraded_from_order_id"`
	ReplacedSubscriptionID *uuid.UUID       `json:"replaced_subscription_id"`
	CreatedAt              time.Time        `json:"created_at"`
	Installments           []InstallmentDTO `json:"installments"`
}

func ToOrderDTO(o model.Order) OrderDTO {
	out := OrderDTO{
		ID: o.ID, PlanPriceID: o.PlanPriceID, InstallmentOfferID: o.InstallmentOfferID, Provider: string(o.Provider),
		Status: o.Status, TotalMinor: o.TotalMinor, DiscountMinor: o.DiscountMinor, CreditMinor: o.CreditMinor,
		PaidMinor: o.PaidMinor, BalanceMinor: o.BalanceMinor(), Currency: o.Currency, PaidInFullAt: o.PaidInFull,
		DefaultedAt: o.DefaultedAt, UpgradedFromOrderID: o.UpgradedFromOrderID,
		ReplacedSubscriptionID: o.ReplacedSubscriptionID, CreatedAt: o.CreatedAt,
		Installments: make([]InstallmentDTO, 0, len(o.Installments)),
	}
	if o.Price != nil {
		p := ToPriceDTO(*o.Price)
		out.Price = &p
	}
	for _, i := range o.Installments {
		out.Installments = append(out.Installments, InstallmentDTO{
			ID: i.ID, Seq: i.Seq, DueAt: i.DueAt, AmountMinor: i.AmountMinor, Status: i.Status, PaidAt: i.PaidAt,
		})
	}
	return out
}

type OrderOutput struct {
	Body OrderDTO
}

type OrderInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	OrderID     uuid.UUID `path:"orderId"`
}

type QuoteOrderInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Body        struct {
		PlanPriceID        uuid.UUID  `json:"plan_price_id" doc:"A one-time (lifetime) price"`
		InstallmentOfferID *uuid.UUID `json:"installment_offer_id,omitempty"`
		DiscountCode       string     `json:"discount_code,omitempty" maxLength:"40"`
	}
}

func (in *QuoteOrderInput) ToModel() model.QuoteInput {
	return model.QuoteInput{
		PlanPriceID: in.Body.PlanPriceID, InstallmentOfferID: in.Body.InstallmentOfferID, DiscountCode: in.Body.DiscountCode,
	}
}

type ScheduledPaymentDTO struct {
	Seq         int       `json:"seq"`
	DueAt       time.Time `json:"due_at"`
	AmountMinor int64     `json:"amount_minor"`
}

type OrderQuoteOutput struct {
	Body struct {
		PlanPriceID            uuid.UUID             `json:"plan_price_id"`
		InstallmentOfferID     *uuid.UUID            `json:"installment_offer_id"`
		Currency               string                `json:"currency"`
		TotalMinor             int64                 `json:"total_minor"`
		DiscountMinor          int64                 `json:"discount_minor"`
		CreditMinor            int64                 `json:"credit_minor"`
		DueNowMinor            int64                 `json:"due_now_minor" doc:"The first payment"`
		Schedule               []ScheduledPaymentDTO `json:"schedule"`
		ReplacesSubscriptionID *uuid.UUID            `json:"replaces_subscription_id" doc:"Canceled at checkout; its unused time is credit_minor"`
		UpgradesOrderID        *uuid.UUID            `json:"upgrades_order_id" doc:"Upgraded at checkout; what was paid is credit_minor"`
	}
}

func ToOrderQuoteOutput(q model.OrderQuote) *OrderQuoteOutput {
	out := &OrderQuoteOutput{}
	b := &out.Body
	b.PlanPriceID, b.InstallmentOfferID, b.Currency = q.PlanPriceID, q.InstallmentOfferID, q.Currency
	b.TotalMinor, b.DiscountMinor, b.CreditMinor, b.DueNowMinor = q.TotalMinor, q.DiscountMinor, q.CreditMinor, q.DueNowMinor
	b.ReplacesSubscriptionID, b.UpgradesOrderID = q.ReplacesSubscriptionID, q.UpgradesOrderID
	b.Schedule = make([]ScheduledPaymentDTO, 0, len(q.Schedule))
	for _, s := range q.Schedule {
		b.Schedule = append(b.Schedule, ScheduledPaymentDTO(s))
	}
	return out
}

type PayInstallmentInput struct {
	WorkspaceID    uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	IdempotencyKey string    `header:"Idempotency-Key" required:"true" minLength:"8" maxLength:"255"`
	OrderID        uuid.UUID `path:"orderId"`
	InstallmentID  uuid.UUID `path:"installmentId"`
}

type ResumeOrderInput struct {
	WorkspaceID    uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	IdempotencyKey string    `header:"Idempotency-Key" required:"true" minLength:"8" maxLength:"255"`
	OrderID        uuid.UUID `path:"orderId"`
}
