package model

import (
	"time"

	"github.com/google/uuid"
)

// Discount kinds (discounts.kind).
const (
	DiscountPercent = "percent" // PercentBps basis points: 2000 = 20%
	DiscountAmount  = "amount"  // AmountOffMinor in Currency
)

// Discount durations (discounts.duration). One-time payments always apply once.
const (
	DurationOnce      = "once"
	DurationRepeating = "repeating" // for DurationMonths
	DurationForever   = "forever"
)

// What a discount applies to (discounts.applies_to).
const (
	AppliesToAll       = "all"
	AppliesToRecurring = "recurring"
	AppliesToOneTime   = "one_time"
)

// Redemption statuses (discount_redemptions.status).
const (
	RedemptionReserved = "reserved" // held by a pending checkout
	RedemptionRedeemed = "redeemed" // the payment went through
	RedemptionReleased = "released" // the checkout failed or expired
)

// Discount is a code Relay owns.
type Discount struct {
	ID                uuid.UUID
	Code              string
	Name              string
	Kind              string
	PercentBps        *int
	AmountOffMinor    *int64
	Currency          *string
	Duration          string
	DurationMonths    *int
	AppliesTo         string
	PlanID            *uuid.UUID
	AllowInstallments bool
	MaxRedemptions    *int
	PerWorkspaceLimit int
	RedeemedCount     int
	StartsAt          time.Time
	ExpiresAt         *time.Time
	IsActive          bool
}

// NewDiscount creates a discount code (from the CLI).
type NewDiscount struct {
	Code              string
	Name              string
	Kind              string
	PercentBps        *int
	AmountOffMinor    *int64
	Currency          *string
	Duration          string
	DurationMonths    *int
	AppliesTo         string
	PlanID            *uuid.UUID
	AllowInstallments bool
	MaxRedemptions    *int
	PerWorkspaceLimit int
	StartsAt          time.Time
	ExpiresAt         *time.Time
	CreatedBy         *uuid.UUID
}

// DiscountPreviewInput asks what a code would take off a price.
type DiscountPreviewInput struct {
	Code               string
	PlanPriceID        uuid.UUID
	InstallmentOfferID *uuid.UUID
}

// AppliedDiscount is a valid code worked out for one price.
type AppliedDiscount struct {
	Discount      Discount
	DiscountMinor int64
	// EndsAt is when a repeating subscription discount stops (subscriptions.discount_ends_at).
	EndsAt *time.Time
}

// DiscountPreview is a code checked against a price, before checkout.
type DiscountPreview struct {
	Code          string
	SubtotalMinor int64
	DiscountMinor int64
	TotalMinor    int64
	Currency      string
	Duration      string
	// DurationMonths is set for repeating discounts.
	DurationMonths *int
	// Installments are the reduced installment amounts, for an installment offer.
	Installments []int64
}
