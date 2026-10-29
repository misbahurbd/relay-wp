// Package jobs holds billing's River job workers. Track T5 owns this file;
// Wave 2 registers the workers and periodic jobs in internal/app/workers.go.
package jobs

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/service"
)

// ProcessPaymentEventWorker works ProcessPaymentEventArgs.
type ProcessPaymentEventWorker struct {
	river.WorkerDefaults[model.ProcessPaymentEventArgs]
	svc *service.Service
}

// NewProcessPaymentEventWorker builds the worker.
func NewProcessPaymentEventWorker(svc *service.Service) *ProcessPaymentEventWorker {
	return &ProcessPaymentEventWorker{svc: svc}
}

// Work applies one stored webhook event.
func (w *ProcessPaymentEventWorker) Work(ctx context.Context, job *river.Job[model.ProcessPaymentEventArgs]) error {
	return w.svc.ProcessPaymentEvent(ctx, job.Args.PaymentEventID, job.Args.Event)
}

// InstallmentsDueWorker works InstallmentsDueArgs.
type InstallmentsDueWorker struct {
	river.WorkerDefaults[model.InstallmentsDueArgs]
	svc *service.Service
}

// NewInstallmentsDueWorker builds the worker.
func NewInstallmentsDueWorker(svc *service.Service) *InstallmentsDueWorker {
	return &InstallmentsDueWorker{svc: svc}
}

// Work runs one installments pass.
func (w *InstallmentsDueWorker) Work(ctx context.Context, _ *river.Job[model.InstallmentsDueArgs]) error {
	_, err := w.svc.RunInstallmentsDue(ctx)
	return err
}

// ExpireTrialsWorker works ExpireTrialsArgs.
type ExpireTrialsWorker struct {
	river.WorkerDefaults[model.ExpireTrialsArgs]
	svc *service.Service
}

// NewExpireTrialsWorker builds the worker.
func NewExpireTrialsWorker(svc *service.Service) *ExpireTrialsWorker {
	return &ExpireTrialsWorker{svc: svc}
}

// Work ends trials that reached their end.
func (w *ExpireTrialsWorker) Work(ctx context.Context, _ *river.Job[model.ExpireTrialsArgs]) error {
	_, err := w.svc.ExpireTrials(ctx)
	return err
}

// ExpireCheckoutsWorker works ExpireCheckoutsArgs.
type ExpireCheckoutsWorker struct {
	river.WorkerDefaults[model.ExpireCheckoutsArgs]
	svc *service.Service
}

// NewExpireCheckoutsWorker builds the worker.
func NewExpireCheckoutsWorker(svc *service.Service) *ExpireCheckoutsWorker {
	return &ExpireCheckoutsWorker{svc: svc}
}

// Work expires old pending checkouts.
func (w *ExpireCheckoutsWorker) Work(ctx context.Context, _ *river.Job[model.ExpireCheckoutsArgs]) error {
	_, err := w.svc.ExpireCheckouts(ctx)
	return err
}

// PurgeEventPayloadsWorker works PurgeEventPayloadsArgs.
type PurgeEventPayloadsWorker struct {
	river.WorkerDefaults[model.PurgeEventPayloadsArgs]
	svc *service.Service
}

// NewPurgeEventPayloadsWorker builds the worker.
func NewPurgeEventPayloadsWorker(svc *service.Service) *PurgeEventPayloadsWorker {
	return &PurgeEventPayloadsWorker{svc: svc}
}

// Work nulls old webhook payloads.
func (w *PurgeEventPayloadsWorker) Work(ctx context.Context, _ *river.Job[model.PurgeEventPayloadsArgs]) error {
	_, err := w.svc.PurgeEventPayloads(ctx)
	return err
}

// PlanCacheDriftWorker works PlanCacheDriftArgs.
type PlanCacheDriftWorker struct {
	river.WorkerDefaults[model.PlanCacheDriftArgs]
	svc *service.Service
}

// NewPlanCacheDriftWorker builds the worker.
func NewPlanCacheDriftWorker(svc *service.Service) *PlanCacheDriftWorker {
	return &PlanCacheDriftWorker{svc: svc}
}

// Work logs plan cache drift.
func (w *PlanCacheDriftWorker) Work(ctx context.Context, _ *river.Job[model.PlanCacheDriftArgs]) error {
	_, err := w.svc.CheckPlanCacheDrift(ctx)
	return err
}

// PeriodicJobs are billing's scheduled passes. They run on the leader only.
func PeriodicJobs() []*river.PeriodicJob {
	every := func(d time.Duration, args river.JobArgs) *river.PeriodicJob {
		return river.NewPeriodicJob(river.PeriodicInterval(d),
			func() (river.JobArgs, *river.InsertOpts) { return args, nil },
			&river.PeriodicJobOpts{RunOnStart: true})
	}
	return []*river.PeriodicJob{
		every(time.Hour, model.InstallmentsDueArgs{}),
		every(15*time.Minute, model.ExpireTrialsArgs{}),
		every(15*time.Minute, model.ExpireCheckoutsArgs{}),
		every(6*time.Hour, model.PurgeEventPayloadsArgs{}),
		every(24*time.Hour, model.PlanCacheDriftArgs{}),
	}
}
