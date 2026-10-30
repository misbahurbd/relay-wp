package dto

import (
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

type LimitsDTO struct {
	Members         int `json:"members" doc:"Seat-using members; -1 is unlimited"`
	WhatsAppNumbers int `json:"whatsapp_numbers" doc:"Connected WhatsApp numbers; -1 is unlimited"`
}

func ToLimitsDTO(l model.Limits) LimitsDTO { return LimitsDTO(l) }

type OfferProviderDTO struct {
	Provider   string `json:"provider" enum:"stripe,creem,paypal,sslcommerz"`
	Collection string `json:"collection" enum:"provider_schedule,app_link,provider_financed" doc:"provider_schedule: the provider charges each installment; app_link: Relay emails a pay link for each one"`
}

type InstallmentOfferDTO struct {
	ID           uuid.UUID          `json:"id"`
	Count        int                `json:"count" example:"3"`
	Interval     string             `json:"interval" enum:"month"`
	TotalMinor   int64              `json:"total_minor" doc:"What all installments add up to; may be more than the one-time price"`
	AmountsMinor []int64            `json:"amounts_minor" doc:"Each installment, in order; the last absorbs rounding"`
	GraceDays    int                `json:"grace_days" doc:"Days a missed installment has before access is suspended"`
	Providers    []OfferProviderDTO `json:"providers"`
}

type PriceDTO struct {
	ID                uuid.UUID             `json:"id"`
	PlanID            uuid.UUID             `json:"plan_id"`
	PlanCode          string                `json:"plan_code" example:"growth"`
	BillingType       string                `json:"billing_type" enum:"recurring,one_time"`
	Interval          *string               `json:"interval" nullable:"true" enum:"month,year" doc:"Null for one-time prices"`
	Currency          string                `json:"currency" example:"USD"`
	AmountMinor       int64                 `json:"amount_minor" example:"2900"`
	Providers         []string              `json:"providers" doc:"Providers that can sell this price"`
	InstallmentOffers []InstallmentOfferDTO `json:"installment_offers" doc:"Ways to pay a one-time price in installments"`
}

func ToPriceDTO(p model.Price) PriceDTO {
	out := PriceDTO{
		ID: p.ID, PlanID: p.PlanID, PlanCode: p.PlanCode, BillingType: p.BillingType, Interval: nonEmpty(p.Interval),
		Currency: p.Currency, AmountMinor: p.AmountMinor, Providers: providerNames(p.Providers),
		InstallmentOffers: make([]InstallmentOfferDTO, 0, len(p.InstallmentOffers)),
	}
	for _, o := range p.InstallmentOffers {
		offer := InstallmentOfferDTO{
			ID: o.ID, Count: o.Count, Interval: o.Interval, TotalMinor: o.TotalMinor,
			AmountsMinor: model.SplitInstallments(o.TotalMinor, o.Count), GraceDays: o.GraceDays,
			Providers: make([]OfferProviderDTO, 0, len(o.Providers)),
		}
		for _, op := range o.Providers {
			offer.Providers = append(offer.Providers, OfferProviderDTO{Provider: string(op.Provider), Collection: op.Collection})
		}
		out.InstallmentOffers = append(out.InstallmentOffers, offer)
	}
	return out
}

type PlanDTO struct {
	ID                uuid.UUID  `json:"id"`
	Code              string     `json:"code" example:"growth"`
	Version           int        `json:"version"`
	Name              string     `json:"name" example:"Growth"`
	Description       string     `json:"description" example:"For growing teams running regular campaigns."`
	Features          []string   `json:"features" doc:"Bullet points for the pricing table, in order"`
	Recommended       bool       `json:"recommended" doc:"The plan the pricing table points people to"`
	TrialDays         int        `json:"trial_days" doc:"0: no trial"`
	TrialRequiresCard bool       `json:"trial_requires_card"`
	Limits            LimitsDTO  `json:"limits"`
	Prices            []PriceDTO `json:"prices"`
}

type ProviderDTO struct {
	Name                string   `json:"name" enum:"stripe,creem,paypal,sslcommerz"`
	EmbeddedCheckout    bool     `json:"embedded_checkout" doc:"True: checkout returns a client_secret for an in-app form; false: a url to open"`
	EmbeddableURL       bool     `json:"embeddable_url" doc:"The checkout url may be shown in an iframe (Creem's embed.js) instead of a redirect"`
	RenewsSubscriptions bool     `json:"renews_subscriptions"`
	Installments        bool     `json:"installments" doc:"The provider can run installment schedules itself"`
	MerchantOfRecord    bool     `json:"merchant_of_record" doc:"The provider charges and reports tax"`
	Currencies          []string `json:"currencies"`
	PublishableKey      *string  `json:"publishable_key" doc:"Browser key for the provider's embedded form (Stripe Elements)"`
}

type CatalogOutput struct {
	Body struct {
		Plans     []PlanDTO     `json:"plans"`
		Providers []ProviderDTO `json:"providers" doc:"Payment providers this server has enabled"`
	}
}

func ToCatalogOutput(c model.Catalog) *CatalogOutput {
	out := &CatalogOutput{}
	out.Body.Plans = make([]PlanDTO, 0, len(c.Plans))
	for _, p := range c.Plans {
		plan := PlanDTO{
			ID: p.ID, Code: p.Code, Version: p.Version, Name: p.Name, Description: p.Description,
			Features: nonNil(p.Features), Recommended: p.Recommended, TrialDays: p.TrialDays,
			TrialRequiresCard: p.TrialRequiresCard, Limits: ToLimitsDTO(p.Limits),
			Prices: make([]PriceDTO, 0, len(p.Prices)),
		}
		for _, pr := range p.Prices {
			plan.Prices = append(plan.Prices, ToPriceDTO(pr))
		}
		out.Body.Plans = append(out.Body.Plans, plan)
	}
	out.Body.Providers = make([]ProviderDTO, 0, len(c.Providers))
	for _, p := range c.Providers {
		caps := p.Capabilities
		currencies := caps.Currencies
		if currencies == nil {
			currencies = []string{}
		}
		out.Body.Providers = append(out.Body.Providers, ProviderDTO{
			Name: string(p.Provider), EmbeddedCheckout: caps.EmbeddedCheckout, EmbeddableURL: caps.EmbeddableURL,
			RenewsSubscriptions: caps.RenewsSubscriptions,
			Installments:        caps.Installments, MerchantOfRecord: caps.MerchantOfRecord, Currencies: currencies,
			PublishableKey: nonEmpty(caps.PublishableKey),
		})
	}
	return out
}
