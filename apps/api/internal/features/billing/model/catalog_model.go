package model

import (
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Catalog is the public pricing page: every active plan and the providers that can sell them.
type Catalog struct {
	Plans     []Plan
	Providers []ProviderInfo
}

// ProviderInfo is an enabled payment provider and what it can do.
type ProviderInfo struct {
	Provider     payment.Provider
	Capabilities payment.Capabilities
}

// Plan is one version of a plan with its active prices.
type Plan struct {
	ID                uuid.UUID
	Code              string
	Version           int
	Name              string
	TrialDays         int
	TrialRequiresCard bool
	Limits            Limits
	SortOrder         int
	Prices            []Price
}

// Price is what the pricing page shows for a plan, the same for every provider.
type Price struct {
	ID          uuid.UUID
	PlanID      uuid.UUID
	PlanCode    string
	BillingType string // BillingRecurring or BillingOneTime
	Interval    string // IntervalMonth or IntervalYear; empty for one-time prices
	Currency    string
	AmountMinor int64
	// Providers can sell this price (they have an active provider_prices row).
	Providers []payment.Provider
	// InstallmentOffers split a one-time price into payments.
	InstallmentOffers []InstallmentOffer
}

// InstallmentOffer splits a one-time price into Count monthly payments.
type InstallmentOffer struct {
	ID          uuid.UUID
	PlanPriceID uuid.UUID
	Count       int
	Interval    string
	// TotalMinor may include a markup over the one-time price (Q4).
	TotalMinor int64
	GraceDays  int
	MaxMissed  int
	// Providers can collect this offer, each in its own way.
	Providers []OfferProvider
}

// OfferProvider is a provider that can collect an installment offer, and how.
type OfferProvider struct {
	Provider   payment.Provider
	Collection string // CollectionProviderSchedule, CollectionAppLink or CollectionProviderFinanced
}
