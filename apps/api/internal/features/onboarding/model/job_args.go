package model

import (
	"github.com/google/uuid"
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
