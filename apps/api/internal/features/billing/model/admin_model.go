package model

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// AdminCatalog is the whole catalog for staff, inactive rows included.
type AdminCatalog struct {
	Plans []AdminPlan
	// Providers are the ones this server has enabled.
	Providers []payment.Provider
}

// AdminPlan is one plan version with every price it has had.
type AdminPlan struct {
	ID      uuid.UUID
	Code    string
	Version int
	Name    string
	// Description is the pricing table's one line about the plan.
	Description string
	// Features are the pricing table's bullet points, in order.
	Features []string
	// Recommended marks the plan the pricing table points people to.
	Recommended       bool
	TrialDays         int
	TrialRequiresCard bool
	Limits            Limits
	IsActive          bool
	SortOrder         int
	Prices            []AdminPrice
	// Workspaces is how many workspaces an edit to Limits reaches now.
	Workspaces int
}

// AdminPrice is a plan price with how each provider sells it.
type AdminPrice struct {
	ID             uuid.UUID
	BillingType    string
	Interval       string // empty for one-time prices
	Currency       string
	AmountMinor    int64
	IsActive       bool
	ProviderPrices []AdminProviderPrice
	Offers         []AdminOffer
}

// AdminOffer is an installment offer on a one-time price.
type AdminOffer struct {
	ID             uuid.UUID
	Count          int
	TotalMinor     int64
	GraceDays      int
	MaxMissed      int
	IsActive       bool
	ProviderPrices []AdminProviderPrice
}

// AdminProviderPrice links a price, or one of its offers, to a provider's price or product.
type AdminProviderPrice struct {
	ID              uuid.UUID
	Provider        string
	ExternalPriceID *string
	Collection      *string // only for installment offers
	IsActive        bool
}

// PlanDetails are what staff set on a plan version: everything but its code, version and prices.
type PlanDetails struct {
	Name string
	// Description is the pricing table's one line about the plan.
	Description string
	// Features are the pricing table's bullet points, in order.
	Features []string
	// Recommended marks the plan the pricing table points people to.
	Recommended       bool
	TrialDays         int
	TrialRequiresCard bool
	Limits            Limits
	SortOrder         int
}

// NewPlan adds a plan. A code already in use publishes its next version.
type NewPlan struct {
	Code string
	PlanDetails
}

// PlanEdit changes a plan version in place. Its limits reach every workspace
// on it at once, so a change to them needs a reason and is audited.
type PlanEdit struct {
	PlanID uuid.UUID
	PlanDetails
	Reason    string
	ChangedBy uuid.UUID
}

// NewPrice adds a price to a plan.
type NewPrice struct {
	PlanID      uuid.UUID
	BillingType string
	Interval    string // required for recurring prices
	Currency    string
	AmountMinor int64
}

// NewInstallmentOffer splits a one-time price into Count monthly payments.
type NewInstallmentOffer struct {
	PlanPriceID uuid.UUID
	Count       int
	TotalMinor  int64
	GraceDays   int
	MaxMissed   int
}

// NewProviderPrice says how a provider sells a price, or one of its installment offers.
type NewProviderPrice struct {
	PlanPriceID        uuid.UUID
	InstallmentOfferID *uuid.UUID
	Provider           string
	ExternalPriceID    *string
	Collection         *string // required with InstallmentOfferID, never without
}

// AdminWorkspace is a workspace and the plan it is on, for staff.
type AdminWorkspace struct {
	ID          uuid.UUID
	Name        string
	OwnerEmail  string
	Plan        string
	TrialEndsAt *time.Time
	CreatedAt   time.Time
	// Grant is nil on the free plan.
	Grant *AdminGrant
	// Limits apply now, overrides included.
	Limits Limits
}

// AdminGrant is a workspace's current grant.
type AdminGrant struct {
	ID       uuid.UUID
	PlanCode string
	PlanName string
	Source   string
	Status   string
	EndsAt   *time.Time
	// Overrides are this workspace's own limits on top of the plan's.
	Overrides Limits
}

// ManualGrantInput puts a workspace on a plan by hand.
type ManualGrantInput struct {
	WorkspaceID uuid.UUID
	PlanID      uuid.UUID
	EndsAt      *time.Time // nil: until staff ends it
	Reason      string
	CreatedBy   uuid.UUID
}

// GrantOverridesEdit replaces a workspace's own limits on top of its plan's. Empty clears them.
type GrantOverridesEdit struct {
	WorkspaceID uuid.UUID
	Overrides   Limits
	Reason      string
	ChangedBy   uuid.UUID
}
