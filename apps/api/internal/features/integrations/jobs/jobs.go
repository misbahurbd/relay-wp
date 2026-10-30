// Package jobs holds integrations' River job workers.
package jobs

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/service"
)

// DeliverWorker works DeliverArgs.
type DeliverWorker struct {
	river.WorkerDefaults[model.DeliverArgs]
	svc *service.Service
}

// NewDeliverWorker builds the worker.
func NewDeliverWorker(svc *service.Service) *DeliverWorker { return &DeliverWorker{svc: svc} }

// Work sends the delivery; an error makes River retry it with backoff.
func (w *DeliverWorker) Work(ctx context.Context, job *river.Job[model.DeliverArgs]) error {
	return w.svc.Deliver(ctx, job.Args.WorkspaceID, job.Args.DeliveryID, job.Attempt >= job.MaxAttempts)
}

// Timeout covers the 10-second request plus the bookkeeping around it.
func (w *DeliverWorker) Timeout(*river.Job[model.DeliverArgs]) time.Duration { return 30 * time.Second }

// CleanupPeriodicJob schedules CleanupArgs daily, and once at start.
func CleanupPeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(24*time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return model.CleanupArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true})
}

// CleanupWorker works CleanupArgs.
type CleanupWorker struct {
	river.WorkerDefaults[model.CleanupArgs]
	svc *service.Service
}

// NewCleanupWorker builds the worker.
func NewCleanupWorker(svc *service.Service) *CleanupWorker { return &CleanupWorker{svc: svc} }

// Work deletes delivery log rows older than 30 days.
func (w *CleanupWorker) Work(ctx context.Context, _ *river.Job[model.CleanupArgs]) error {
	_, err := w.svc.CleanupDeliveries(ctx)
	return err
}
