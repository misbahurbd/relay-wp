package model

import (
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// DeliverArgs sends one webhook delivery.
type DeliverArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	DeliveryID  uuid.UUID `json:"delivery_id"`
}

// Kind is the job's stable name in the river_job table.
func (DeliverArgs) Kind() string { return "webhook_deliver" }

// DeliverAttempts with River's attempt⁴-second backoff spread retries over about 1.3 hours.
const DeliverAttempts = 8

// InsertOpts sets the retry budget.
func (DeliverArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: DeliverAttempts}
}

// CleanupArgs deletes old delivery log rows.
type CleanupArgs struct{}

// Kind is the job's stable name in the river_job table.
func (CleanupArgs) Kind() string { return "integrations_cleanup" }
