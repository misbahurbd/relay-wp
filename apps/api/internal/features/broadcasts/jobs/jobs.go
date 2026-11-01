// Package jobs holds broadcasts' River workers.
package jobs

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/broadcasts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/broadcasts/service"
)

// StartWorker takes a due broadcast's audience.
type StartWorker struct {
	river.WorkerDefaults[model.StartArgs]
	svc *service.Service
}

// NewStartWorker builds the worker.
func NewStartWorker(svc *service.Service) *StartWorker { return &StartWorker{svc: svc} }

// Work implements river.Worker.
func (w *StartWorker) Work(ctx context.Context, job *river.Job[model.StartArgs]) error {
	return w.svc.Start(ctx, job.Args.WorkspaceID, job.Args.BroadcastID)
}

// BatchWorker queues one batch of a broadcast's messages.
type BatchWorker struct {
	river.WorkerDefaults[model.BatchArgs]
	svc *service.Service
}

// NewBatchWorker builds the worker.
func NewBatchWorker(svc *service.Service) *BatchWorker { return &BatchWorker{svc: svc} }

// Work implements river.Worker.
func (w *BatchWorker) Work(ctx context.Context, job *river.Job[model.BatchArgs]) error {
	return w.svc.Batch(ctx, job.Args.WorkspaceID, job.Args.BroadcastID, job.Args.Seq)
}

// TickWorker starts due broadcasts and finishes sent ones.
type TickWorker struct {
	river.WorkerDefaults[model.TickArgs]
	svc *service.Service
}

// NewTickWorker builds the worker.
func NewTickWorker(svc *service.Service) *TickWorker { return &TickWorker{svc: svc} }

// Work implements river.Worker.
func (w *TickWorker) Work(ctx context.Context, _ *river.Job[model.TickArgs]) error {
	return w.svc.Tick(ctx)
}

// TickPeriodicJob runs the tick every minute.
func TickPeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(time.Minute),
		func() (river.JobArgs, *river.InsertOpts) { return model.TickArgs{}, nil }, nil)
}
