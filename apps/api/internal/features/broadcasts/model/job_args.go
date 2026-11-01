package model

import (
	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/queue"
)

// BatchSize is how many recipients one batch job queues.
const BatchSize = 100

// StartArgs takes a broadcast's audience and starts its batches.
type StartArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	BroadcastID uuid.UUID `json:"broadcast_id"`
}

// Kind is the job's stable name in the river_job table.
func (StartArgs) Kind() string { return "broadcasts_start" }

// InsertOpts runs on the bulk queue. It isn't unique: a reschedule queues a new
// start while the old one waits, and the start itself runs once (StartBroadcast).
func (StartArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: queue.Bulk, MaxAttempts: 10}
}

// BatchArgs queues the next BatchSize recipients of a broadcast. Seq tells one
// batch from the next, so the running job can queue its successor.
type BatchArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	BroadcastID uuid.UUID `json:"broadcast_id"`
	Seq         int       `json:"seq"`
}

// Kind is the job's stable name in the river_job table.
func (BatchArgs) Kind() string { return "broadcasts_batch" }

// InsertOpts runs each batch once, on the bulk queue (AD-3).
func (BatchArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: queue.Bulk, MaxAttempts: 10, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// TickArgs starts due scheduled broadcasts and finishes sent ones (AD-7).
type TickArgs struct{}

// Kind is the job's stable name in the river_job table.
func (TickArgs) Kind() string { return "broadcasts_tick" }

// InsertOpts tries once: the next tick does the same work.
func (TickArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{MaxAttempts: 1} }
