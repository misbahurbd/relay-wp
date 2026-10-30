package dto

import (
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

type PreviewDiscountInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Body        struct {
		Code               string     `json:"code" minLength:"3" maxLength:"40" doc:"Case doesn't matter"`
		PlanPriceID        uuid.UUID  `json:"plan_price_id"`
		InstallmentOfferID *uuid.UUID `json:"installment_offer_id,omitempty"`
	}
}

func (in *PreviewDiscountInput) ToModel() model.DiscountPreviewInput {
	return model.DiscountPreviewInput{
		Code: in.Body.Code, PlanPriceID: in.Body.PlanPriceID, InstallmentOfferID: in.Body.InstallmentOfferID,
	}
}

type DiscountPreviewOutput struct {
	Body struct {
		Code              string  `json:"code" example:"LAUNCH20"`
		SubtotalMinor     int64   `json:"subtotal_minor"`
		DiscountMinor     int64   `json:"discount_minor"`
		TotalMinor        int64   `json:"total_minor"`
		Currency          string  `json:"currency"`
		Duration          string  `json:"duration" enum:"once,repeating,forever"`
		DurationMonths    *int    `json:"duration_months" doc:"For repeating discounts"`
		InstallmentsMinor []int64 `json:"installments_minor" doc:"The reduced installments, for an installment offer"`
	}
}

func ToDiscountPreviewOutput(p model.DiscountPreview) *DiscountPreviewOutput {
	out := &DiscountPreviewOutput{}
	b := &out.Body
	b.Code, b.SubtotalMinor, b.DiscountMinor, b.TotalMinor = p.Code, p.SubtotalMinor, p.DiscountMinor, p.TotalMinor
	b.Currency, b.Duration, b.DurationMonths = p.Currency, p.Duration, p.DurationMonths
	b.InstallmentsMinor = p.Installments
	if b.InstallmentsMinor == nil {
		b.InstallmentsMinor = []int64{}
	}
	return out
}
