// Package model holds workspaces' domain types, constants and errors, shared by every layer of the
// feature.
package model

import (
	"time"

	"github.com/google/uuid"
)

// Answers on the Welcome screen.
const (
	StartSetup  = "setup"
	StartSample = "sample"
)

// The allowed onboarding answers and goal values.
var (
	FirstGoals = []string{"offers", "support", "automate"}
	Industries = []string{"retail_ecommerce", "food_restaurants", "education", "health_clinics", "travel", "real_estate", "other"}
	TeamSizes  = []string{"just_me", "2_5", "6_20", "20_plus"}
	GoalValues = []string{"marketing", "support", "sales"}
)

// Workspace is a business using Relay.
type Workspace struct {
	ID          uuid.UUID
	Name        string
	Industry    *string
	Website     *string
	Country     *string
	TeamSize    *string
	Goals       []string
	Plan        string
	TrialEndsAt *time.Time
	ActivatedAt *time.Time
}

// Summary is one row of the signed-in user's workspace list, with enough
// onboarding state for the frontend to pick where to send them.
type Summary struct {
	ID           uuid.UUID
	Name         string
	RoleName     string
	IsOwner      bool
	LastActiveAt time.Time
	CurrentStep  string
	Completed    bool
	Activated    bool
}

// CreateInput is the Welcome screen's answers.
type CreateInput struct {
	Name      string
	FirstGoal string
	StartMode string
}

// BusinessInput is setup step 1 ("Tell us about your business").
type BusinessInput struct {
	Name     string
	Industry string
	Website  string
	Country  string
	TeamSize string
	Goals    []string
}
