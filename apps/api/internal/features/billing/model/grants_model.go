package model

import (
	"time"

	"github.com/google/uuid"
)

// Grant sources (plan_grants.source).
const (
	GrantFromSubscription = "subscription"
	GrantFromOrder        = "order"
	GrantFromTrial        = "trial"
	GrantManual           = "manual"
)

// Grant statuses (plan_grants.status). A suspended grant keeps its row but
// the workspace falls back to free limits until it is restored.
const (
	GrantActive    = "active"
	GrantSuspended = "suspended"
	GrantEnded     = "ended"
)

// Why a grant is suspended (plan_grants.suspend_reason). Each is lifted only by
// its own cure: a renewal, a paid installment, or support.
const (
	SuspendPastDue      = "past_due"
	SuspendInstallments = "installments"
	SuspendDispute      = "dispute"
)

// Grant is what a workspace can use: one active or suspended grant at most.
type Grant struct {
	ID             uuid.UUID
	PlanID         uuid.UUID
	PlanCode       string
	PlanName       string
	Source         string // GrantFrom… or GrantManual
	SubscriptionID *uuid.UUID
	OrderID        *uuid.UUID
	Status         string // GrantActive or GrantSuspended (GrantEnded once replaced)
	// PlanLimits are the plan's limits as they are now; Overrides are this grant's own (ADR 0008).
	PlanLimits    Limits
	Overrides     Limits
	StartsAt      time.Time
	EndsAt        *time.Time // nil: lifetime
	SuspendedAt   *time.Time
	SuspendReason string // Suspend…; empty unless suspended
}

// GrantInput gives a workspace a new grant. The current one ends in the same transaction.
type GrantInput struct {
	PlanID         uuid.UUID
	Source         string
	SubscriptionID *uuid.UUID
	OrderID        *uuid.UUID
	StartsAt       time.Time
	EndsAt         *time.Time // required for trials
	CreatedBy      *uuid.UUID
	Reason         *string // required for manual grants
}

// CurrentPlan is the plan a workspace is on right now.
type CurrentPlan struct {
	// PlanCode is FreePlan with no grant or while the grant is suspended.
	PlanCode string
	PlanName string
	// Grant is nil on the free plan.
	Grant *Grant
	// Limits apply now: the free plan's while suspended, every resource filled in.
	Limits Limits
}

// Overview is the billing page: plan, subscription and lifetime deal.
type Overview struct {
	Plan CurrentPlan
	// Subscription is the live one, if any.
	Subscription *Subscription
	// Order is the live lifetime deal, if any, with its installments.
	Order *Order
}

// Why a workspace can't simply be deleted (Q17).
const (
	DeleteBlockedSubscription = "subscription" // cancel the subscription first
	DeleteNeedsLTDConfirm     = "lifetime"     // the owner must confirm giving up the deal
)

// DeletionCheck says whether a workspace on a paid plan can be deleted.
type DeletionCheck struct {
	Allowed bool
	// Reason is DeleteBlockedSubscription or DeleteNeedsLTDConfirm when not allowed.
	Reason string
}
