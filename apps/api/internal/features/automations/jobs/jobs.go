// Package jobs holds automations' River workers.
package jobs

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/service"
)

// InboundWorker reacts to a customer's message.
type InboundWorker struct {
	river.WorkerDefaults[model.InboundArgs]
	svc *service.Service
}

// NewInboundWorker builds the worker.
func NewInboundWorker(svc *service.Service) *InboundWorker { return &InboundWorker{svc: svc} }

// Work implements river.Worker.
func (w *InboundWorker) Work(ctx context.Context, job *river.Job[model.InboundArgs]) error {
	return w.svc.HandleInbound(ctx, job.Args.Inbound)
}

// ProcessRunWorker fires a due run.
type ProcessRunWorker struct {
	river.WorkerDefaults[model.ProcessRunArgs]
	svc *service.Service
}

// NewProcessRunWorker builds the worker.
func NewProcessRunWorker(svc *service.Service) *ProcessRunWorker { return &ProcessRunWorker{svc: svc} }

// Work implements river.Worker.
func (w *ProcessRunWorker) Work(ctx context.Context, job *river.Job[model.ProcessRunArgs]) error {
	return w.svc.ProcessRun(ctx, job.Args.WorkspaceID, job.Args.RunID)
}

// TickWorker queues due runs.
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

// TickPeriodicJob runs the tick every minute. It picks up delayed runs, and any
// run a crash left behind.
func TickPeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(time.Minute),
		func() (river.JobArgs, *river.InsertOpts) { return model.TickArgs{}, nil }, nil)
}
