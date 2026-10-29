// Package jobs holds team's River job workers.
package jobs

import (
	"context"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/service"

	"github.com/riverqueue/river"
)

// MaintenanceArgs expires temporary permissions and keeps the access log's
// monthly partitions: two months ahead, twelve kept. Access checks already
// ignore expired permissions, so this only records the expiry and tidies up.
type MaintenanceArgs struct{}

// Kind is the job's stable name in the river_job table.
func (MaintenanceArgs) Kind() string { return "team_maintenance" }

// MaintenanceWorker works MaintenanceArgs.
type MaintenanceWorker struct {
	river.WorkerDefaults[MaintenanceArgs]
	svc *service.Service
}

// NewMaintenanceWorker builds the worker.
func NewMaintenanceWorker(svc *service.Service) *MaintenanceWorker {
	return &MaintenanceWorker{svc: svc}
}

// Work runs one maintenance pass.
func (w *MaintenanceWorker) Work(ctx context.Context, _ *river.Job[MaintenanceArgs]) error {
	if err := w.svc.ExpireOverrides(ctx); err != nil {
		return err
	}
	return w.svc.EnsureLogPartitions(ctx)
}

// MaintenancePeriodicJob runs maintenance every 15 minutes and at startup.
func MaintenancePeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(15*time.Minute),
		func() (river.JobArgs, *river.InsertOpts) { return MaintenanceArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true})
}
