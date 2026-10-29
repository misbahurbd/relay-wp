// Package jobs holds contacts' River job workers.
package jobs

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/service"
)

// ImportWorker works ImportArgs.
type ImportWorker struct {
	river.WorkerDefaults[model.ImportArgs]
	svc *service.Service
}

// NewImportWorker builds the worker.
func NewImportWorker(svc *service.Service) *ImportWorker { return &ImportWorker{svc: svc} }

// Timeout gives a 50,000-row file room; River's default is one minute.
func (w *ImportWorker) Timeout(*river.Job[model.ImportArgs]) time.Duration { return 15 * time.Minute }

// Work imports the file.
func (w *ImportWorker) Work(ctx context.Context, job *river.Job[model.ImportArgs]) error {
	return w.svc.RunImport(ctx, job.Args.WorkspaceID, job.Args.ImportID, job.Attempt >= job.MaxAttempts)
}
