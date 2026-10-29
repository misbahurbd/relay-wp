package model

import (
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// ImportArgs works one contact import after its mapping is confirmed.
type ImportArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	ImportID    uuid.UUID `json:"import_id"`
}

// Kind is the job's stable name in the river_job table.
func (ImportArgs) Kind() string { return "contacts_import" }

// InsertOpts retries a failed import twice; batches already written are upserted again.
func (ImportArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{MaxAttempts: 3} }
