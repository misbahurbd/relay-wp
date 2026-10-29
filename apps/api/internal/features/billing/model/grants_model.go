package model

import (
	"encoding/json"
	"fmt"
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

// Unlimited is a limit with no cap.
const Unlimited = -1

// Limits are what a plan allows, from plans.limits (copied onto each grant).
// A key missing from the JSON means Unlimited.
type Limits struct {
	Members         int `json:"members"`
	WhatsAppNumbers int `json:"whatsapp_numbers"`
}

// FreeLimits apply with no current grant, or while it is suspended: the five
// seats a workspace had before billing, and one WhatsApp number.
var FreeLimits = Limits{Members: 5, WhatsAppNumbers: 1}

// ParseLimits decodes plans.limits or plan_grants.limits.
func ParseLimits(raw []byte) (Limits, error) {
	var in struct {
		Members         *int `json:"members"`
		WhatsAppNumbers *int `json:"whatsapp_numbers"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			return Limits{}, fmt.Errorf("parse plan limits: %w", err)
		}
	}
	or := func(p *int) int {
		if p == nil {
			return Unlimited
		}
		return *p
	}
	return Limits{Members: or(in.Members), WhatsAppNumbers: or(in.WhatsAppNumbers)}, nil
}

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
	Limits         Limits
	StartsAt       time.Time
	EndsAt         *time.Time // nil: lifetime
	SuspendedAt    *time.Time
}

// GrantInput gives a workspace a new grant. The current one ends in the same transaction.
type GrantInput struct {
	PlanID         uuid.UUID
	Source         string
	SubscriptionID *uuid.UUID
	OrderID        *uuid.UUID
	// Limits nil copies the plan's.
	Limits    *Limits
	StartsAt  time.Time
	EndsAt    *time.Time // required for trials
	CreatedBy *uuid.UUID
	Reason    *string // required for manual grants
}

// CurrentPlan is the plan a workspace is on right now.
type CurrentPlan struct {
	// PlanCode is FreePlan with no grant or while the grant is suspended.
	PlanCode string
	PlanName string
	// Grant is nil on the free plan.
	Grant *Grant
	// Limits apply now: FreeLimits while suspended.
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
