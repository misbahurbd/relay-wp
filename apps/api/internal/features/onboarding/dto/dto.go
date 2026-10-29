// Package dto holds onboarding's HTTP request and response shapes and the converters from domain
// types.
package dto

import (
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/onboarding/model"

	"github.com/google/uuid"
)

type stepDTO struct {
	ID     string `json:"id" enum:"business,whatsapp,test_message"`
	Status string `json:"status" enum:"done,current,upcoming,skipped"`
}

type checklistItemDTO struct {
	ID   string `json:"id" enum:"account,whatsapp,template,contacts,broadcast,team"`
	Done bool   `json:"done"`
}

type onboardingDTO struct {
	CurrentStep     string             `json:"current_step" enum:"business,whatsapp,test_message,done"`
	Steps           []stepDTO          `json:"steps"`
	FirstGoal       string             `json:"first_goal" enum:"offers,support,automate"`
	SampleData      bool               `json:"sample_data"`
	WhatsAppSkipped bool               `json:"whatsapp_skipped"`
	Completed       bool               `json:"completed" doc:"Setup finished or skipped: open the dashboard"`
	Activated       bool               `json:"activated"`
	TrialEndsAt     *time.Time         `json:"trial_ends_at"`
	Checklist       []checklistItemDTO `json:"checklist"`
	TourSeen        bool               `json:"tour_seen" doc:"This member's own state"`
	ChecklistHidden bool               `json:"checklist_hidden" doc:"This member's own state"`
}

type OnboardingOutput struct {
	Body onboardingDTO
}

func ToOutput(o model.Onboarding) *OnboardingOutput {
	out := &OnboardingOutput{Body: onboardingDTO{
		CurrentStep: o.CurrentStep, FirstGoal: o.FirstGoal, SampleData: o.SampleData,
		WhatsAppSkipped: o.WhatsAppSkipped, Completed: o.Completed, Activated: o.Activated,
		TrialEndsAt: o.TrialEndsAt, TourSeen: o.TourSeen, ChecklistHidden: o.ChecklistHidden,
		Steps:     make([]stepDTO, 0, len(o.Steps)),
		Checklist: make([]checklistItemDTO, 0, len(o.Checklist)),
	}}
	for _, s := range o.Steps {
		out.Body.Steps = append(out.Body.Steps, stepDTO(s))
	}
	for _, c := range o.Checklist {
		out.Body.Checklist = append(out.Body.Checklist, checklistItemDTO(c))
	}
	return out
}

type WorkspaceInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
}

type MemberStateInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	Body        struct {
		TourSeen        *bool `json:"tour_seen,omitempty"`
		ChecklistHidden *bool `json:"checklist_hidden,omitempty"`
	}
}
