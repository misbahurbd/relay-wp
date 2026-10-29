package model

import (
	"time"

	"github.com/google/uuid"
)

// Invitation is one row of the pending invites list.
type Invitation struct {
	ID                 uuid.UUID
	Email              string
	RoleID             uuid.UUID
	RoleName           string
	Status             string
	ExpiresAt          time.Time
	LastSentAt         time.Time
	SendCount          int32
	RenewalRequestedAt *time.Time
	CreatedAt          time.Time
	InvitedByName      string
	TeamIDs            []uuid.UUID
}

// InviteInput is the Invite teammates wizard.
type InviteInput struct {
	Emails          []string
	RoleID          uuid.UUID
	TeamIDs         []uuid.UUID
	NumberIDs       []uuid.UUID
	ChatScope       *string
	Overrides       []OverrideInput
	PersonalMessage string
	ExpiresIn       string
	Require2FA      bool
}

// InvitePreview is what the invite link shows before joining.
type InvitePreview struct {
	State          string
	WorkspaceName  string
	Email          string
	RoleName       string
	InvitedByName  string
	InvitedByRole  string
	ExpiresAt      time.Time
	Message        string
	CanDo          []string
	WorkspaceID    uuid.UUID
	PermissionKeys []string
	TeamNames      []string
	// NumberLabels are "Main line (+1 415 555 0142)".
	NumberLabels []string
}
