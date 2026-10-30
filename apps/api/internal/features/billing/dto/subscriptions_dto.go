package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

type SubscriptionDTO struct {
	ID                 uuid.UUID  `json:"id"`
	PlanPriceID        uuid.UUID  `json:"plan_price_id"`
	Price              *PriceDTO  `json:"price,omitempty"`
	Provider           string     `json:"provider" enum:"stripe,creem,paypal,sslcommerz"`
	Status             string     `json:"status" enum:"incomplete,trialing,active,past_due,canceled"`
	CurrentPeriodStart *time.Time `json:"current_period_start"`
	CurrentPeriodEnd   *time.Time `json:"current_period_end"`
	GraceEndsAt        *time.Time `json:"grace_ends_at" doc:"Past due: access continues until then"`
	CancelAtPeriodEnd  bool       `json:"cancel_at_period_end"`
	CanceledAt         *time.Time `json:"canceled_at"`
	PendingPlanPriceID *uuid.UUID `json:"pending_plan_price_id" doc:"A downgrade waiting for pending_change_at"`
	PendingChangeAt    *time.Time `json:"pending_change_at"`
	DiscountEndsAt     *time.Time `json:"discount_ends_at" doc:"When a repeating discount stops"`
	CreatedAt          time.Time  `json:"created_at"`
}

func ToSubscriptionDTO(s model.Subscription) SubscriptionDTO {
	out := SubscriptionDTO{
		ID: s.ID, PlanPriceID: s.PlanPriceID, Provider: string(s.Provider), Status: s.Status,
		CurrentPeriodStart: s.CurrentPeriodStart, CurrentPeriodEnd: s.CurrentPeriodEnd, GraceEndsAt: s.GraceEndsAt,
		CancelAtPeriodEnd: s.CancelAtPeriodEnd, CanceledAt: s.CanceledAt, PendingPlanPriceID: s.PendingPlanPriceID,
		PendingChangeAt: s.PendingChangeAt, DiscountEndsAt: s.DiscountEndsAt, CreatedAt: s.CreatedAt,
	}
	if s.Price != nil {
		p := ToPriceDTO(*s.Price)
		out.Price = &p
	}
	return out
}

type SubscriptionOutput struct {
	Body SubscriptionDTO
}

type ChangePlanInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Body        struct {
		PlanPriceID uuid.UUID `json:"plan_price_id" doc:"A recurring price of another plan or interval"`
	}
}

type ChangeQuoteOutput struct {
	Body struct {
		Kind           string    `json:"kind" enum:"upgrade,downgrade,interval"`
		FromPriceID    uuid.UUID `json:"from_price_id"`
		ToPriceID      uuid.UUID `json:"to_price_id"`
		EffectiveAt    time.Time `json:"effective_at" doc:"Now for an upgrade; the period end for a downgrade"`
		ProrationMinor int64     `json:"proration_minor" doc:"Charged now (positive) or credited (negative)"`
		Currency       string    `json:"currency"`
	}
}

func ToChangeQuoteOutput(q model.ChangeQuote) *ChangeQuoteOutput {
	out := &ChangeQuoteOutput{}
	out.Body.Kind, out.Body.FromPriceID, out.Body.ToPriceID = q.Kind, q.FromPriceID, q.ToPriceID
	out.Body.EffectiveAt, out.Body.ProrationMinor, out.Body.Currency = q.EffectiveAt, q.ProrationMinor, q.Currency
	return out
}

type CancelSubscriptionInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Body        struct {
		AtPeriodEnd bool `json:"at_period_end" doc:"True keeps access until the period ends; false cancels now"`
	}
}
