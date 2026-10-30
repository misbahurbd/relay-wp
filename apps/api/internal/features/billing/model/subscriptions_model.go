package model

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Subscription statuses (subscriptions.status). incomplete is an embedded
// checkout before payment and never counts as live.
const (
	SubscriptionIncomplete = "incomplete"
	SubscriptionTrialing   = "trialing"
	SubscriptionActive     = "active"
	SubscriptionPastDue    = "past_due"
	SubscriptionCanceled   = "canceled"
)

// LiveSubscriptionStatuses are the statuses a workspace has at most one subscription in.
var LiveSubscriptionStatuses = []string{SubscriptionTrialing, SubscriptionActive, SubscriptionPastDue}

// Plan change kinds (subscription_changes.kind).
const (
	ChangeUpgrade   = "upgrade"   // applies now, prorated
	ChangeDowngrade = "downgrade" // waits for the period end
	ChangeInterval  = "interval"  // same plan, monthly ↔ yearly
)

// Subscription is a recurring plan, renewed by its provider or by Relay.
type Subscription struct {
	ID                     uuid.UUID
	WorkspaceID            uuid.UUID
	PlanPriceID            uuid.UUID
	BillingCustomerID      uuid.UUID
	Provider               payment.Provider
	ExternalSubscriptionID *string
	RenewalMode            string // RenewalProvider or RenewalApp
	Status                 string
	ProviderStatus         *string
	CurrentPeriodStart     *time.Time
	CurrentPeriodEnd       *time.Time
	GraceEndsAt            *time.Time
	CancelAtPeriodEnd      bool
	CanceledAt             *time.Time
	// PendingPlanPriceID is a downgrade waiting for PendingChangeAt.
	PendingPlanPriceID *uuid.UUID
	PendingChangeAt    *time.Time
	DiscountID         *uuid.UUID
	DiscountEndsAt     *time.Time
	CreatedAt          time.Time
	// Price is the price it is on, filled in for responses.
	Price *Price
}

// ChangeQuote is what moving a subscription to another price would do.
type ChangeQuote struct {
	Kind        string // ChangeUpgrade, ChangeDowngrade or ChangeInterval
	FromPriceID uuid.UUID
	ToPriceID   uuid.UUID
	// EffectiveAt is now for an upgrade, the period end for a downgrade.
	EffectiveAt time.Time
	// ProrationMinor is charged now (positive) or credited (negative).
	ProrationMinor int64
	Currency       string
}

// SubscriptionCredit is the unused time on a subscription a lifetime deal replaces (Q13).
type SubscriptionCredit struct {
	// SubscriptionID is nil when the workspace has no live subscription.
	SubscriptionID *uuid.UUID
	CreditMinor    int64
	Currency       string
}
