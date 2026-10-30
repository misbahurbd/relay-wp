package model

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Order statuses (orders.status).
const (
	OrderPending        = "pending"
	OrderInInstallments = "in_installments"
	OrderPaid           = "paid"
	OrderDefaulted      = "defaulted"
	OrderUpgraded       = "upgraded"
	OrderRefunded       = "refunded"
	OrderCanceled       = "canceled"
)

// LiveOrderStatuses are the statuses a workspace has at most one order in.
var LiveOrderStatuses = []string{OrderInInstallments, OrderPaid, OrderDefaulted}

// Installment statuses (installments.status).
const (
	InstallmentScheduled = "scheduled"
	InstallmentDue       = "due"
	InstallmentPaid      = "paid"
	InstallmentOverdue   = "overdue"
	InstallmentFailed    = "failed"
	InstallmentWaived    = "waived"
	InstallmentCanceled  = "canceled"
)

// Order is a one-time purchase such as a lifetime deal, paid at once or in installments.
type Order struct {
	ID                 uuid.UUID
	WorkspaceID        uuid.UUID
	PlanPriceID        uuid.UUID
	InstallmentOfferID *uuid.UUID
	BillingCustomerID  uuid.UUID
	Provider           payment.Provider
	ExternalRef        *string
	Status             string
	TotalMinor         int64
	DiscountID         *uuid.UUID
	DiscountMinor      int64
	// CreditMinor is money carried in: unused subscription time or a previous order's payments.
	CreditMinor int64
	PaidMinor   int64
	Currency    string
	PaidInFull  *time.Time
	DefaultedAt *time.Time
	// UpgradedFromOrderID is the order this one replaced (Q16).
	UpgradedFromOrderID *uuid.UUID
	// ReplacedSubscriptionID is the subscription this one replaced (Q13).
	ReplacedSubscriptionID *uuid.UUID
	CreatedAt              time.Time
	Installments           []Installment
	// Price is the price bought, filled in for responses.
	Price *Price
}

// BalanceMinor is what is still owed.
func (o Order) BalanceMinor() int64 {
	return o.TotalMinor - o.DiscountMinor - o.CreditMinor - o.PaidMinor
}

// Installment is one scheduled payment of an order.
type Installment struct {
	ID             uuid.UUID
	OrderID        uuid.UUID
	Seq            int
	DueAt          time.Time
	AmountMinor    int64
	Status         string
	PaidAt         *time.Time
	ReminderSentAt *time.Time
}

// QuoteInput asks what a lifetime deal would cost this workspace now.
type QuoteInput struct {
	PlanPriceID        uuid.UUID
	InstallmentOfferID *uuid.UUID
	DiscountCode       string
}

// OrderQuote is what a lifetime deal would cost: the discount comes off the
// total first, then credit, and the rest is split into the schedule.
type OrderQuote struct {
	PlanPriceID        uuid.UUID
	InstallmentOfferID *uuid.UUID
	Currency           string
	TotalMinor         int64
	DiscountMinor      int64
	Discount           *AppliedDiscount
	CreditMinor        int64
	// DueNowMinor is the first payment: the whole balance, or installment 1.
	DueNowMinor int64
	// Schedule has one entry per payment; a single entry for a purchase in full.
	Schedule []ScheduledPayment
	// ReplacesSubscriptionID is canceled at checkout, its unused time credited (Q13).
	ReplacesSubscriptionID *uuid.UUID
	// UpgradesOrderID becomes upgraded at checkout, its payments credited (Q16).
	UpgradesOrderID *uuid.UUID
}

// ScheduledPayment is one payment of a quote's schedule.
type ScheduledPayment struct {
	Seq         int
	DueAt       time.Time
	AmountMinor int64
}

// NewOrder creates a pending order and its installments when a lifetime-deal checkout starts.
type NewOrder struct {
	Provider          payment.Provider
	BillingCustomerID uuid.UUID
	Quote             OrderQuote
}

// InstallmentRun counts what one pass of the daily installments job did.
type InstallmentRun struct {
	MarkedDue int
	Reminded  int
	Overdue   int
	Suspended int
	Defaulted int
}

// SplitInstallments splits total into count amounts that add up to it
// exactly. Every amount is the same except the last, which absorbs the
// rounding (Q4). count must be at least 1 and total at least count.
func SplitInstallments(total int64, count int) []int64 {
	if count < 1 {
		return nil
	}
	each := total / int64(count)
	out := make([]int64, count)
	for i := range out {
		out[i] = each
	}
	out[count-1] = total - each*int64(count-1)
	return out
}
