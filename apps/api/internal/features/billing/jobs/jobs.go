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

// InvoiceDocumentWorker works InvoiceDocumentArgs.
type InvoiceDocumentWorker struct {
	river.WorkerDefaults[model.InvoiceDocumentArgs]
	svc *service.Service
}

// NewInvoiceDocumentWorker builds the worker.
func NewInvoiceDocumentWorker(svc *service.Service) *InvoiceDocumentWorker {
	return &InvoiceDocumentWorker{svc: svc}
}

// Work renders, stores and emails one invoice's PDF; each step runs once.
func (w *InvoiceDocumentWorker) Work(ctx context.Context, job *river.Job[model.InvoiceDocumentArgs]) error {
	return w.svc.DeliverInvoiceDocument(ctx, job.Args.WorkspaceID, job.Args.InvoiceID)
}

// IssueRefundWorker works IssueRefundArgs.
type IssueRefundWorker struct {
	river.WorkerDefaults[model.IssueRefundArgs]
	svc *service.Service
}

// NewIssueRefundWorker builds the worker.
func NewIssueRefundWorker(svc *service.Service) *IssueRefundWorker {
	return &IssueRefundWorker{svc: svc}
}

// Work asks the provider for one recorded refund.
func (w *IssueRefundWorker) Work(ctx context.Context, job *river.Job[model.IssueRefundArgs]) error {
	return w.svc.IssueRefund(ctx, job.Args.WorkspaceID, job.Args.RefundID)
}

// InvoiceDocumentsSweepWorker works InvoiceDocumentsSweepArgs.
type InvoiceDocumentsSweepWorker struct {
	river.WorkerDefaults[model.InvoiceDocumentsSweepArgs]
	svc *service.Service
}

// NewInvoiceDocumentsSweepWorker builds the worker.
func NewInvoiceDocumentsSweepWorker(svc *service.Service) *InvoiceDocumentsSweepWorker {
	return &InvoiceDocumentsSweepWorker{svc: svc}
}

// Work queues document jobs for invoices still without a PDF.
func (w *InvoiceDocumentsSweepWorker) Work(ctx context.Context, _ *river.Job[model.InvoiceDocumentsSweepArgs]) error {
	_, err := w.svc.SweepInvoiceDocuments(ctx)
	return err
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
	_, err := w.svc.ExpireGrants(ctx)
	return err
}

// ReconcileCheckoutsWorker works ReconcileCheckoutsArgs.
type ReconcileCheckoutsWorker struct {
	river.WorkerDefaults[model.ReconcileCheckoutsArgs]
	svc *service.Service
}

// NewReconcileCheckoutsWorker builds the worker.
func NewReconcileCheckoutsWorker(svc *service.Service) *ReconcileCheckoutsWorker {
	return &ReconcileCheckoutsWorker{svc: svc}
}

// Work settles pending checkouts with the provider.
func (w *ReconcileCheckoutsWorker) Work(ctx context.Context, _ *river.Job[model.ReconcileCheckoutsArgs]) error {
	_, err := w.svc.ReconcileCheckouts(ctx)
	return err
}

// ReconcileSubscriptionsWorker works ReconcileSubscriptionsArgs.
type ReconcileSubscriptionsWorker struct {
	river.WorkerDefaults[model.ReconcileSubscriptionsArgs]
	svc *service.Service
}

// NewReconcileSubscriptionsWorker builds the worker.
func NewReconcileSubscriptionsWorker(svc *service.Service) *ReconcileSubscriptionsWorker {
	return &ReconcileSubscriptionsWorker{svc: svc}
}

// Work reads overdue subscriptions back from the provider, and asks it again
// to cancel what it refused to.
func (w *ReconcileSubscriptionsWorker) Work(ctx context.Context, _ *river.Job[model.ReconcileSubscriptionsArgs]) error {
	if _, err := w.svc.ReconcileSubscriptions(ctx); err != nil {
		return err
	}
	_, err := w.svc.RetryRefusedCancels(ctx)
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

// PastDueGraceWorker works PastDueGraceArgs.
type PastDueGraceWorker struct {
	river.WorkerDefaults[model.PastDueGraceArgs]
	svc *service.Service
}

// NewPastDueGraceWorker builds the worker.
func NewPastDueGraceWorker(svc *service.Service) *PastDueGraceWorker {
	return &PastDueGraceWorker{svc: svc}
}

// Work suspends plans whose past-due grace has ended.
func (w *PastDueGraceWorker) Work(ctx context.Context, _ *river.Job[model.PastDueGraceArgs]) error {
	_, err := w.svc.SuspendPastDue(ctx)
	return err
}

// StuckPaymentEventsWorker works StuckPaymentEventsArgs.
type StuckPaymentEventsWorker struct {
	river.WorkerDefaults[model.StuckPaymentEventsArgs]
	svc *service.Service
}

// NewStuckPaymentEventsWorker builds the worker.
func NewStuckPaymentEventsWorker(svc *service.Service) *StuckPaymentEventsWorker {
	return &StuckPaymentEventsWorker{svc: svc}
}

// Work logs webhook events that never got processed and refunds that never
// got sent.
func (w *StuckPaymentEventsWorker) Work(ctx context.Context, _ *river.Job[model.StuckPaymentEventsArgs]) error {
	if _, err := w.svc.ReportStuckPaymentEvents(ctx); err != nil {
		return err
	}
	_, err := w.svc.ReportStuckRefunds(ctx)
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
		every(15*time.Minute, model.ReconcileCheckoutsArgs{}),
		every(time.Hour, model.ReconcileSubscriptionsArgs{}),
		every(6*time.Hour, model.PurgeEventPayloadsArgs{}),
		every(24*time.Hour, model.PlanCacheDriftArgs{}),
		every(15*time.Minute, model.PastDueGraceArgs{}),
		every(time.Hour, model.StuckPaymentEventsArgs{}),
		every(15*time.Minute, model.InvoiceDocumentsSweepArgs{}),
	}
}
