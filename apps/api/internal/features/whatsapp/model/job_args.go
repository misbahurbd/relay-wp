package model

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// SetupArgs runs the setup tasks for a newly connected number.
type SetupArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	NumberID    uuid.UUID `json:"number_id"`
}

// Kind is the job's stable name in the river_job table.
func (SetupArgs) Kind() string { return "whatsapp_setup" }

// InsertOpts gives Meta about 6 minutes of retries before the number is marked failed.
func (SetupArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{MaxAttempts: 5} }

// WebhookArgs is one webhook delivery from Meta, already signature-checked.
type WebhookArgs struct {
	Payload json.RawMessage `json:"payload"`
}

// Kind is the job's stable name in the river_job table.
func (WebhookArgs) Kind() string { return "whatsapp_webhook" }
