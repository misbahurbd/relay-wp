package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

type AdminProviderPriceDTO struct {
	ID              uuid.UUID `json:"id"`
	Provider        string    `json:"provider" enum:"stripe,creem,paypal,sslcommerz"`
	ExternalPriceID *string   `nullable:"true" json:"external_price_id" doc:"The provider's price or product ID"`
	Collection      *string   `nullable:"true" json:"collection" enum:"provider_schedule,app_link,provider_financed" doc:"Installment offers only"`
	IsActive        bool      `json:"is_active"`
}

type AdminOfferDTO struct {
	ID             uuid.UUID               `json:"id"`
	Count          int                     `json:"count"`
	TotalMinor     int64                   `json:"total_minor"`
	AmountsMinor   []int64                 `json:"amounts_minor"`
	GraceDays      int                     `json:"grace_days"`
	MaxMissed      int                     `json:"max_missed"`
	IsActive       bool                    `json:"is_active"`
	ProviderPrices []AdminProviderPriceDTO `json:"provider_prices"`
}

type AdminPriceDTO struct {
	ID             uuid.UUID               `json:"id"`
	BillingType    string                  `json:"billing_type" enum:"recurring,one_time"`
	Interval       *string                 `nullable:"true" json:"interval" enum:"month,year"`
	Currency       string                  `json:"currency"`
	AmountMinor    int64                   `json:"amount_minor"`
	IsActive       bool                    `json:"is_active"`
	ProviderPrices []AdminProviderPriceDTO `json:"provider_prices"`
	Offers         []AdminOfferDTO         `json:"installment_offers"`
}

type AdminPlanDTO struct {
	ID                uuid.UUID       `json:"id"`
	Code              string          `json:"code"`
	Version           int             `json:"version"`
	Name              string          `json:"name"`
	Description       string          `json:"description" doc:"One line about the plan; may be empty"`
	Features          []string        `json:"features" doc:"Bullet points for the pricing table, in order"`
	Recommended       bool            `json:"recommended" doc:"The plan the pricing table points people to"`
	TrialDays         int             `json:"trial_days"`
	TrialRequiresCard bool            `json:"trial_requires_card"`
	Limits            LimitsDTO       `json:"limits"`
	IsActive          bool            `json:"is_active"`
	SortOrder         int             `json:"sort_order"`
	Prices            []AdminPriceDTO `json:"prices"`
	Workspaces        int             `json:"workspaces" doc:"Workspaces whose limits come from this plan now; editing its limits reaches them all"`
}

type AdminCatalogOutput struct {
	Body struct {
		Plans     []AdminPlanDTO `json:"plans"`
		Providers []string       `json:"providers" doc:"Payment providers this server has enabled"`
	}
}

func toAdminProviderPrices(in []model.AdminProviderPrice) []AdminProviderPriceDTO {
	out := make([]AdminProviderPriceDTO, 0, len(in))
	for _, p := range in {
		out = append(out, AdminProviderPriceDTO(p))
	}
	return out
}

func ToAdminCatalogOutput(c model.AdminCatalog) *AdminCatalogOutput {
	out := &AdminCatalogOutput{}
	out.Body.Providers = providerNames(c.Providers)
	out.Body.Plans = make([]AdminPlanDTO, 0, len(c.Plans))
	for _, p := range c.Plans {
		plan := AdminPlanDTO{
			ID: p.ID, Code: p.Code, Version: p.Version, Name: p.Name, Description: p.Description,
			Features: nonNil(p.Features), Recommended: p.Recommended, TrialDays: p.TrialDays,
			TrialRequiresCard: p.TrialRequiresCard, Limits: ToLimitsDTO(p.Limits), IsActive: p.IsActive,
			SortOrder: p.SortOrder, Prices: make([]AdminPriceDTO, 0, len(p.Prices)), Workspaces: p.Workspaces,
		}
		for _, pr := range p.Prices {
			price := AdminPriceDTO{
				ID: pr.ID, BillingType: pr.BillingType, Interval: nonEmpty(pr.Interval), Currency: pr.Currency,
				AmountMinor: pr.AmountMinor, IsActive: pr.IsActive, ProviderPrices: toAdminProviderPrices(pr.ProviderPrices),
				Offers: make([]AdminOfferDTO, 0, len(pr.Offers)),
			}
			for _, o := range pr.Offers {
				price.Offers = append(price.Offers, AdminOfferDTO{
					ID: o.ID, Count: o.Count, TotalMinor: o.TotalMinor,
					AmountsMinor: model.SplitInstallments(o.TotalMinor, o.Count), GraceDays: o.GraceDays,
					MaxMissed: o.MaxMissed, IsActive: o.IsActive, ProviderPrices: toAdminProviderPrices(o.ProviderPrices),
				})
			}
			plan.Prices = append(plan.Prices, price)
		}
		out.Body.Plans = append(out.Body.Plans, plan)
	}
	return out
}

type CreatedOutput struct {
	Body struct {
		ID uuid.UUID `json:"id"`
	}
}

func ToCreatedOutput(id uuid.UUID) *CreatedOutput {
	out := &CreatedOutput{}
	out.Body.ID = id
	return out
}

// PlanDetailsBody is what staff set on a plan version, on create and on edit.
type PlanDetailsBody struct {
	Name              string    `json:"name" minLength:"1" maxLength:"80" example:"Growth"`
	Description       string    `json:"description,omitempty" maxLength:"200" example:"For growing teams running regular campaigns."`
	Features          []string  `json:"features,omitempty" maxItems:"20" doc:"Bullet points for the pricing table, in order; each at most 80 characters"`
	Recommended       bool      `json:"recommended,omitempty" doc:"Mark this as the plan the pricing table points people to"`
	TrialDays         int       `json:"trial_days" minimum:"0" maximum:"90"`
	TrialRequiresCard bool      `json:"trial_requires_card"`
	Limits            LimitsDTO `json:"limits" doc:"What the plan includes: a cap (-1 unlimited) per count or monthly resource, 1 per feature. A resource left out is not included."`
	SortOrder         int       `json:"sort_order" minimum:"0" maximum:"10000"`
}

func (b PlanDetailsBody) toModel() model.PlanDetails {
	return model.PlanDetails{
		Name: b.Name, Description: b.Description, Features: b.Features, Recommended: b.Recommended,
		TrialDays: b.TrialDays, TrialRequiresCard: b.TrialRequiresCard, Limits: model.Limits(b.Limits), SortOrder: b.SortOrder,
	}
}

type CreatePlanInput struct {
	Body struct {
		Code string `json:"code" minLength:"2" maxLength:"41" example:"growth" doc:"A code already in use publishes its next version"`
		PlanDetailsBody
	}
}

func (in *CreatePlanInput) ToModel() model.NewPlan {
	return model.NewPlan{Code: in.Body.Code, PlanDetails: in.Body.toModel()}
}

type UpdatePlanInput struct {
	ID   uuid.UUID `path:"id"`
	Body struct {
		PlanDetailsBody
		Reason string `json:"reason,omitempty" maxLength:"300" doc:"Required when the limits change; kept with the change"`
	}
}

func (in *UpdatePlanInput) ToModel(changedBy uuid.UUID) model.PlanEdit {
	return model.PlanEdit{PlanID: in.ID, PlanDetails: in.Body.toModel(), Reason: in.Body.Reason, ChangedBy: changedBy}
}

type CreatePriceInput struct {
	Body struct {
		PlanID      uuid.UUID `json:"plan_id"`
		BillingType string    `json:"billing_type" enum:"recurring,one_time"`
		Interval    string    `json:"interval,omitempty" enum:"month,year" doc:"Recurring prices only"`
		Currency    string    `json:"currency" minLength:"3" maxLength:"3" example:"USD"`
		AmountMinor int64     `json:"amount_minor" minimum:"0" example:"2900"`
	}
}

func (in *CreatePriceInput) ToModel() model.NewPrice {
	b := in.Body
	return model.NewPrice{
		PlanID: b.PlanID, BillingType: b.BillingType, Interval: b.Interval, Currency: b.Currency, AmountMinor: b.AmountMinor,
	}
}

type CreateInstallmentOfferInput struct {
	Body struct {
		PlanPriceID uuid.UUID `json:"plan_price_id" doc:"A one-time price"`
		Count       int       `json:"count" minimum:"2" maximum:"24"`
		TotalMinor  int64     `json:"total_minor" minimum:"1" doc:"May be more than the one-time price"`
		GraceDays   int       `json:"grace_days" minimum:"0" maximum:"60" default:"7"`
		MaxMissed   int       `json:"max_missed" minimum:"1" maximum:"24" default:"2"`
	}
}

func (in *CreateInstallmentOfferInput) ToModel() model.NewInstallmentOffer {
	b := in.Body
	return model.NewInstallmentOffer{
		PlanPriceID: b.PlanPriceID, Count: b.Count, TotalMinor: b.TotalMinor, GraceDays: b.GraceDays, MaxMissed: b.MaxMissed,
	}
}

type CreateProviderPriceInput struct {
	Body struct {
		PlanPriceID        uuid.UUID  `json:"plan_price_id"`
		InstallmentOfferID *uuid.UUID `json:"installment_offer_id,omitempty" doc:"Link one of the price's installment offers instead of the price"`
		Provider           string     `json:"provider" enum:"stripe,creem,paypal,sslcommerz"`
		ExternalPriceID    *string    `json:"external_price_id,omitempty" maxLength:"200" doc:"The provider's price or product ID"`
		Collection         *string    `json:"collection,omitempty" enum:"provider_schedule,app_link,provider_financed" doc:"Required for an installment offer"`
	}
}

func (in *CreateProviderPriceInput) ToModel() model.NewProviderPrice {
	b := in.Body
	return model.NewProviderPrice{
		PlanPriceID: b.PlanPriceID, InstallmentOfferID: b.InstallmentOfferID, Provider: b.Provider,
		ExternalPriceID: b.ExternalPriceID, Collection: b.Collection,
	}
}

type DeactivateCatalogRowInput struct {
	ID uuid.UUID `path:"id"`
}

type AdminDiscountDTO struct {
	ID                uuid.UUID  `json:"id"`
	Code              string     `json:"code"`
	Name              string     `json:"name"`
	Kind              string     `json:"kind" enum:"percent,amount"`
	PercentBps        *int       `nullable:"true" json:"percent_bps"`
	AmountOffMinor    *int64     `nullable:"true" json:"amount_off_minor"`
	Currency          *string    `nullable:"true" json:"currency"`
	Duration          string     `json:"duration" enum:"once,repeating,forever"`
	DurationMonths    *int       `nullable:"true" json:"duration_months"`
	AppliesTo         string     `json:"applies_to" enum:"all,recurring,one_time"`
	PlanID            *uuid.UUID `nullable:"true" json:"plan_id"`
	AllowInstallments bool       `json:"allow_installments"`
	MaxRedemptions    *int       `nullable:"true" json:"max_redemptions"`
	PerWorkspaceLimit int        `json:"per_workspace_limit"`
	RedeemedCount     int        `json:"redeemed_count"`
	StartsAt          time.Time  `json:"starts_at"`
	ExpiresAt         *time.Time `nullable:"true" json:"expires_at"`
	IsActive          bool       `json:"is_active"`
}

type AdminDiscountsOutput struct {
	Body struct {
		Discounts []AdminDiscountDTO `json:"discounts"`
	}
}

func ToAdminDiscountsOutput(ds []model.Discount) *AdminDiscountsOutput {
	out := &AdminDiscountsOutput{}
	out.Body.Discounts = make([]AdminDiscountDTO, 0, len(ds))
	for _, d := range ds {
		out.Body.Discounts = append(out.Body.Discounts, AdminDiscountDTO(d))
	}
	return out
}

type AdminDiscountOutput struct {
	Body AdminDiscountDTO
}

func ToAdminDiscountOutput(d model.Discount) *AdminDiscountOutput {
	return &AdminDiscountOutput{Body: AdminDiscountDTO(d)}
}

type CreateDiscountInput struct {
	Body struct {
		Code              string     `json:"code" minLength:"3" maxLength:"40" example:"LAUNCH20"`
		Name              string     `json:"name,omitempty" maxLength:"80"`
		Kind              string     `json:"kind" enum:"percent,amount"`
		PercentBps        *int       `json:"percent_bps,omitempty" minimum:"1" maximum:"10000" doc:"Basis points: 2000 is 20%"`
		AmountOffMinor    *int64     `json:"amount_off_minor,omitempty" minimum:"1"`
		Currency          *string    `json:"currency,omitempty" minLength:"3" maxLength:"3"`
		Duration          string     `json:"duration,omitempty" enum:"once,repeating,forever" default:"once"`
		DurationMonths    *int       `json:"duration_months,omitempty" minimum:"1" maximum:"36"`
		AppliesTo         string     `json:"applies_to,omitempty" enum:"all,recurring,one_time" default:"all"`
		PlanID            *uuid.UUID `json:"plan_id,omitempty"`
		AllowInstallments bool       `json:"allow_installments"`
		MaxRedemptions    *int       `json:"max_redemptions,omitempty" minimum:"1"`
		PerWorkspaceLimit int        `json:"per_workspace_limit,omitempty" minimum:"1" default:"1"`
		StartsAt          *time.Time `json:"starts_at,omitempty" doc:"Default: now"`
		ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	}
}

func (in *CreateDiscountInput) ToModel(createdBy uuid.UUID) model.NewDiscount {
	b := in.Body
	d := model.NewDiscount{
		Code: b.Code, Name: b.Name, Kind: b.Kind, PercentBps: b.PercentBps, AmountOffMinor: b.AmountOffMinor,
		Currency: b.Currency, Duration: b.Duration, DurationMonths: b.DurationMonths, AppliesTo: b.AppliesTo,
		PlanID: b.PlanID, AllowInstallments: b.AllowInstallments, MaxRedemptions: b.MaxRedemptions,
		PerWorkspaceLimit: b.PerWorkspaceLimit, ExpiresAt: b.ExpiresAt, CreatedBy: &createdBy,
	}
	if b.StartsAt != nil {
		d.StartsAt = *b.StartsAt
	}
	return d
}

type DeactivateDiscountInput struct {
	Code string `path:"code" minLength:"3" maxLength:"40"`
}

type AdminWorkspacesInput struct {
	Search string `query:"search" maxLength:"100" doc:"Matches the workspace name or its creator's email"`
}

type AdminGrantDTO struct {
	ID        uuid.UUID  `json:"id"`
	PlanCode  string     `json:"plan_code"`
	PlanName  string     `json:"plan_name"`
	Source    string     `json:"source" enum:"subscription,order,trial,manual"`
	Status    string     `json:"status" enum:"active,suspended"`
	EndsAt    *time.Time `nullable:"true" json:"ends_at"`
	Overrides LimitsDTO  `json:"overrides" doc:"This workspace's own limits on top of the plan's; only the keys that differ"`
}

type AdminWorkspaceDTO struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	OwnerEmail  string         `json:"owner_email" doc:"Empty when the creator's account is gone"`
	Plan        string         `json:"plan" doc:"The cached plan code; free with no grant"`
	TrialEndsAt *time.Time     `nullable:"true" json:"trial_ends_at"`
	CreatedAt   time.Time      `json:"created_at"`
	Grant       *AdminGrantDTO `json:"grant" doc:"Null on the free plan"`
	Limits      LimitsDTO      `json:"limits" doc:"What applies now, overrides included"`
}

type AdminWorkspacesOutput struct {
	Body struct {
		Workspaces []AdminWorkspaceDTO `json:"workspaces" doc:"Newest first, at most 200"`
	}
}

func ToAdminWorkspacesOutput(ws []model.AdminWorkspace) *AdminWorkspacesOutput {
	out := &AdminWorkspacesOutput{}
	out.Body.Workspaces = make([]AdminWorkspaceDTO, 0, len(ws))
	for _, w := range ws {
		d := AdminWorkspaceDTO{
			ID: w.ID, Name: w.Name, OwnerEmail: w.OwnerEmail, Plan: w.Plan, TrialEndsAt: w.TrialEndsAt, CreatedAt: w.CreatedAt,
			Limits: ToLimitsDTO(w.Limits),
		}
		if g := w.Grant; g != nil {
			d.Grant = &AdminGrantDTO{
				ID: g.ID, PlanCode: g.PlanCode, PlanName: g.PlanName, Source: g.Source, Status: g.Status, EndsAt: g.EndsAt,
				Overrides: LimitsDTO(g.Overrides),
			}
		}
		out.Body.Workspaces = append(out.Body.Workspaces, d)
	}
	return out
}

type ManualGrantInput struct {
	Body struct {
		WorkspaceID uuid.UUID  `json:"workspace_id"`
		PlanID      uuid.UUID  `json:"plan_id"`
		EndsAt      *time.Time `json:"ends_at,omitempty" doc:"Omit for no end"`
		Reason      string     `json:"reason" minLength:"1" maxLength:"300"`
	}
}

func (in *ManualGrantInput) ToModel(createdBy uuid.UUID) model.ManualGrantInput {
	b := in.Body
	return model.ManualGrantInput{
		WorkspaceID: b.WorkspaceID, PlanID: b.PlanID, EndsAt: b.EndsAt, Reason: b.Reason, CreatedBy: createdBy,
	}
}

type AdminWorkspaceInput struct {
	Body struct {
		WorkspaceID uuid.UUID `json:"workspace_id"`
	}
}

type GrantOverridesInput struct {
	Body struct {
		WorkspaceID uuid.UUID `json:"workspace_id"`
		Overrides   LimitsDTO `json:"overrides" doc:"Only the keys that differ from the plan (0 takes a resource away); {} clears them"`
		Reason      string    `json:"reason" minLength:"1" maxLength:"300" doc:"Kept with the change, for the record"`
	}
}

func (in *GrantOverridesInput) ToModel(changedBy uuid.UUID) model.GrantOverridesEdit {
	b := in.Body
	return model.GrantOverridesEdit{WorkspaceID: b.WorkspaceID, Overrides: model.Limits(b.Overrides), Reason: b.Reason, ChangedBy: changedBy}
}
