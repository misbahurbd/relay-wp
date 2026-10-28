package whatsapp

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// SetupArgs runs the setup tasks for a newly connected number.
type SetupArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	NumberID    uuid.UUID `json:"number_id"`
}

// Kind is the job's stable name in the river_job table.
func (SetupArgs) Kind() string { return "whatsapp_setup" }

// InsertOpts gives Meta about 6 minutes of retries before the number is marked failed.
func (SetupArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{MaxAttempts: 5} }

// SetupWorker works SetupArgs.
type SetupWorker struct {
	river.WorkerDefaults[SetupArgs]
	svc *Service
}

// NewSetupWorker builds the worker.
func NewSetupWorker(svc *Service) *SetupWorker { return &SetupWorker{svc: svc} }

// Work runs the remaining setup tasks.
func (w *SetupWorker) Work(ctx context.Context, job *river.Job[SetupArgs]) error {
	return w.svc.runSetup(ctx, job.Args.WorkspaceID, job.Args.NumberID, job.Attempt >= job.MaxAttempts)
}

// WebhookArgs is one webhook delivery from Meta, already signature-checked.
type WebhookArgs struct {
	Payload json.RawMessage `json:"payload"`
}

// Kind is the job's stable name in the river_job table.
func (WebhookArgs) Kind() string { return "whatsapp_webhook" }

// WebhookWorker works WebhookArgs.
type WebhookWorker struct {
	river.WorkerDefaults[WebhookArgs]
	svc *Service
}

// NewWebhookWorker builds the worker.
func NewWebhookWorker(svc *Service) *WebhookWorker { return &WebhookWorker{svc: svc} }

// Work dispatches the webhook's changes to their handlers.
func (w *WebhookWorker) Work(ctx context.Context, job *river.Job[WebhookArgs]) error {
	return w.svc.processWebhook(ctx, job.Args.Payload)
}
