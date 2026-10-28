package onboarding

import (
	"context"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// SetupReminderArgs emails an owner who hasn't connected WhatsApp yet. Scheduled
// at workspace creation (+24 h, +3 days); it does nothing once a number is connected.
type SetupReminderArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	N           int       `json:"n"`
}

// Kind is the job's stable name in the river_job table.
func (SetupReminderArgs) Kind() string { return "onboarding_reminder" }

// TrialReminderArgs emails the owner before the trial ends (day 10 and day 13).
type TrialReminderArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	DaysLeft    int       `json:"days_left"`
}

// Kind is the job's stable name in the river_job table.
func (TrialReminderArgs) Kind() string { return "trial_reminder" }

// SetupReminderWorker works SetupReminderArgs.
type SetupReminderWorker struct {
	river.WorkerDefaults[SetupReminderArgs]
	svc *Service
}

// NewSetupReminderWorker builds the worker.
func NewSetupReminderWorker(svc *Service) *SetupReminderWorker { return &SetupReminderWorker{svc: svc} }

// Work sends the reminder if setup is still unfinished.
func (w *SetupReminderWorker) Work(ctx context.Context, job *river.Job[SetupReminderArgs]) error {
	return ignoreGone(w.svc.sendSetupReminder(ctx, job.Args.WorkspaceID))
}

// TrialReminderWorker works TrialReminderArgs.
type TrialReminderWorker struct {
	river.WorkerDefaults[TrialReminderArgs]
	svc *Service
}

// NewTrialReminderWorker builds the worker.
func NewTrialReminderWorker(svc *Service) *TrialReminderWorker { return &TrialReminderWorker{svc: svc} }

// Work sends the reminder while the trial is still running.
func (w *TrialReminderWorker) Work(ctx context.Context, job *river.Job[TrialReminderArgs]) error {
	return ignoreGone(w.svc.sendTrialReminder(ctx, job.Args.WorkspaceID, job.Args.DaysLeft))
}

// ignoreGone ends a reminder quietly when its workspace was deleted; retrying can't help.
func ignoreGone(err error) error {
	if apperr.IsKind(err, apperr.KindNotFound) {
		return nil
	}
	return err
}
