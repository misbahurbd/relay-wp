// Package jobs holds onboarding's River job workers.
package jobs

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/onboarding/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/onboarding/service"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// SetupReminderWorker works SetupReminderArgs.
type SetupReminderWorker struct {
	river.WorkerDefaults[model.SetupReminderArgs]
	svc *service.Service
}

// NewSetupReminderWorker builds the worker.
func NewSetupReminderWorker(svc *service.Service) *SetupReminderWorker {
	return &SetupReminderWorker{svc: svc}
}

// Work sends the reminder if setup is still unfinished.
func (w *SetupReminderWorker) Work(ctx context.Context, job *river.Job[model.SetupReminderArgs]) error {
	return ignoreGone(w.svc.SendSetupReminder(ctx, job.Args.WorkspaceID))
}

// TrialReminderWorker works TrialReminderArgs.
type TrialReminderWorker struct {
	river.WorkerDefaults[model.TrialReminderArgs]
	svc *service.Service
}

// NewTrialReminderWorker builds the worker.
func NewTrialReminderWorker(svc *service.Service) *TrialReminderWorker {
	return &TrialReminderWorker{svc: svc}
}

// Work sends the reminder while the trial is still running.
func (w *TrialReminderWorker) Work(ctx context.Context, job *river.Job[model.TrialReminderArgs]) error {
	return ignoreGone(w.svc.SendTrialReminder(ctx, job.Args.WorkspaceID, job.Args.DaysLeft))
}

// ignoreGone ends a reminder quietly when its workspace was deleted; retrying can't help.
func ignoreGone(err error) error {
	if apperr.IsKind(err, apperr.KindNotFound) {
		return nil
	}
	return err
}
