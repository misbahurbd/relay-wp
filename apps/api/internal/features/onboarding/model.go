package onboarding

import (
	"time"

	"github.com/google/uuid"
)

// Setup steps, in order. StepDone means every step is finished.
const (
	StepBusiness    = "business"
	StepWhatsApp    = "whatsapp"
	StepTestMessage = "test_message"
	StepDone        = "done"
)

var steps = []string{StepBusiness, StepWhatsApp, StepTestMessage}

// Step statuses shown in the setup stepper.
const (
	statusDone     = "done"
	statusCurrent  = "current"
	statusUpcoming = "upcoming"
	statusSkipped  = "skipped"
)

// Reminder timing, counted from workspace creation (setup) or trial end (trial).
var (
	setupReminderDelays = []time.Duration{24 * time.Hour, 72 * time.Hour}
	trialReminderDays   = []int{4, 1}
)

// StepState is one step in the setup stepper.
type StepState struct {
	ID     string
	Status string
}

// ChecklistItem is one row of the dashboard's "Get started" checklist.
type ChecklistItem struct {
	ID   string
	Done bool
}

// Onboarding is a workspace's setup progress as one member sees it.
type Onboarding struct {
	WorkspaceID     uuid.UUID
	CurrentStep     string
	Steps           []StepState
	FirstGoal       string
	SampleData      bool
	WhatsAppSkipped bool
	Completed       bool
	Activated       bool
	TrialEndsAt     *time.Time
	Checklist       []ChecklistItem
	TourSeen        bool
	ChecklistHidden bool
}

// MemberStateInput changes one member's tour and checklist state. Nil leaves a field as it is.
type MemberStateInput struct {
	TourSeen        *bool
	ChecklistHidden *bool
}
