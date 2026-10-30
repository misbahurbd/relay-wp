package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

type GrantDTO struct {
	ID          uuid.UUID  `json:"id"`
	Source      string     `json:"source" enum:"subscription,order,trial,manual"`
	Status      string     `json:"status" enum:"active,suspended"`
	StartsAt    time.Time  `json:"starts_at"`
	EndsAt      *time.Time `json:"ends_at" doc:"Null: lifetime"`
	SuspendedAt *time.Time `json:"suspended_at" doc:"A missed installment; free limits apply until it is paid"`
}

type CurrentPlanDTO struct {
	PlanCode string    `json:"plan_code" example:"growth" doc:"free with no grant, or while it is suspended"`
	PlanName string    `json:"plan_name"`
	Limits   LimitsDTO `json:"limits" doc:"What applies right now"`
	Grant    *GrantDTO `json:"grant,omitempty"`
}

func ToCurrentPlanDTO(p model.CurrentPlan) CurrentPlanDTO {
	out := CurrentPlanDTO{PlanCode: p.PlanCode, PlanName: p.PlanName, Limits: ToLimitsDTO(p.Limits)}
	if g := p.Grant; g != nil {
		out.Grant = &GrantDTO{
			ID: g.ID, Source: g.Source, Status: g.Status, StartsAt: g.StartsAt, EndsAt: g.EndsAt, SuspendedAt: g.SuspendedAt,
		}
	}
	return out
}

type OverviewOutput struct {
	Body struct {
		Plan         CurrentPlanDTO   `json:"plan"`
		Subscription *SubscriptionDTO `json:"subscription,omitempty"`
		Order        *OrderDTO        `json:"order,omitempty" doc:"The lifetime deal, with its installments"`
	}
}

func ToOverviewOutput(o model.Overview) *OverviewOutput {
	out := &OverviewOutput{}
	out.Body.Plan = ToCurrentPlanDTO(o.Plan)
	if o.Subscription != nil {
		s := ToSubscriptionDTO(*o.Subscription)
		out.Body.Subscription = &s
	}
	if o.Order != nil {
		ord := ToOrderDTO(*o.Order)
		out.Body.Order = &ord
	}
	return out
}
