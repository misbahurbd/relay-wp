package model

import (
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// InboundArgs reacts to one customer message.
type InboundArgs struct {
	Inbound
}

// Kind is the job's stable name in the river_job table.
func (InboundArgs) Kind() string { return "automations_inbound" }

// InsertOpts handles each message once.
func (InboundArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 5, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// ProcessRunArgs fires one step of a due run.
type ProcessRunArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	RunID       uuid.UUID `json:"run_id"`
	Step        int32     `json:"step"`
}

// Kind is the job's stable name in the river_job table.
func (ProcessRunArgs) Kind() string { return "automations_process_run" }

// InsertOpts queues a step once, however often the tick sees it due. A finished
// job doesn't count: a run held by a pause must be able to queue again.
func (ProcessRunArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 5, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRetryable,
		rivertype.JobStateRunning, rivertype.JobStateScheduled,
	}}}
}

// TickArgs finds due runs.
type TickArgs struct{}

// Kind is the job's stable name in the river_job table.
func (TickArgs) Kind() string { return "automations_tick" }
