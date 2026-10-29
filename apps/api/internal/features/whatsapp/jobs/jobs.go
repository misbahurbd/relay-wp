// Package jobs holds whatsapp's River job workers.
package jobs

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/service"
)

// SetupWorker works SetupArgs.
type SetupWorker struct {
	river.WorkerDefaults[model.SetupArgs]
	svc *service.Service
}

// NewSetupWorker builds the worker.
func NewSetupWorker(svc *service.Service) *SetupWorker { return &SetupWorker{svc: svc} }

// Work runs the remaining setup tasks.
func (w *SetupWorker) Work(ctx context.Context, job *river.Job[model.SetupArgs]) error {
	return w.svc.RunSetup(ctx, job.Args.WorkspaceID, job.Args.NumberID, job.Attempt >= job.MaxAttempts)
}

// WebhookWorker works WebhookArgs.
type WebhookWorker struct {
	river.WorkerDefaults[model.WebhookArgs]
	svc *service.Service
}

// NewWebhookWorker builds the worker.
func NewWebhookWorker(svc *service.Service) *WebhookWorker { return &WebhookWorker{svc: svc} }

// Work dispatches the webhook's changes to their handlers.
func (w *WebhookWorker) Work(ctx context.Context, job *river.Job[model.WebhookArgs]) error {
	return w.svc.ProcessWebhook(ctx, job.Args.Payload)
}
