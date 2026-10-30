package model

import (
	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// ProcessPaymentEventArgs is one verified webhook, already stored in
// payment_events. River owns its retries; processing is idempotent.
type ProcessPaymentEventArgs struct {
	PaymentEventID uuid.UUID `json:"payment_event_id"`
	// Event is the normalised event. Its raw payload is not in the job (it has `json:"-"`).
	Event payment.Event `json:"event"`
}

// Kind is the job's stable name in the river_job table.
func (ProcessPaymentEventArgs) Kind() string { return "billing_process_payment_event" }

// InstallmentsDueArgs is the daily installments pass: mark due, remind,
// mark overdue and suspend, default (Q2).
type InstallmentsDueArgs struct{}

// Kind is the job's stable name in the river_job table.
func (InstallmentsDueArgs) Kind() string { return "billing_installments_due" }

// ExpireTrialsArgs ends card-less trials and staff grants that reached their end.
type ExpireTrialsArgs struct{}

// Kind is the job's stable name in the river_job table.
func (ExpireTrialsArgs) Kind() string { return "billing_expire_trials" }

// ExpireCheckoutsArgs expires old pending checkout sessions and releases their reserved discounts.
type ExpireCheckoutsArgs struct{}

// Kind is the job's stable name in the river_job table.
func (ExpireCheckoutsArgs) Kind() string { return "billing_expire_checkouts" }

// PurgeEventPayloadsArgs nulls payment_events.payload after 90 days.
type PurgeEventPayloadsArgs struct{}

// Kind is the job's stable name in the river_job table.
func (PurgeEventPayloadsArgs) Kind() string { return "billing_purge_event_payloads" }

// PlanCacheDriftArgs logs workspaces whose cached plan disagrees with their grant (Q8).
type PlanCacheDriftArgs struct{}

// Kind is the job's stable name in the river_job table.
func (PlanCacheDriftArgs) Kind() string { return "billing_plan_cache_drift" }

// PastDueGraceArgs suspends the plans of past-due subscriptions whose grace ran out.
type PastDueGraceArgs struct{}

// Kind is the job's stable name in the river_job table.
func (PastDueGraceArgs) Kind() string { return "billing_past_due_grace" }

// StuckPaymentEventsArgs reports webhook events still unprocessed long after they arrived.
type StuckPaymentEventsArgs struct{}

// Kind is the job's stable name in the river_job table.
func (StuckPaymentEventsArgs) Kind() string { return "billing_stuck_payment_events" }

// Ensure every job's args satisfy River's interface.
var (
	_ river.JobArgs = ProcessPaymentEventArgs{}
	_ river.JobArgs = InstallmentsDueArgs{}
	_ river.JobArgs = ExpireTrialsArgs{}
	_ river.JobArgs = ExpireCheckoutsArgs{}
	_ river.JobArgs = PurgeEventPayloadsArgs{}
	_ river.JobArgs = PlanCacheDriftArgs{}
	_ river.JobArgs = PastDueGraceArgs{}
	_ river.JobArgs = StuckPaymentEventsArgs{}
)
