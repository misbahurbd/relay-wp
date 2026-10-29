package model

import (
	"time"

	"github.com/google/uuid"
)

// Member is one row of the Team › Members table.
type Member struct {
	UserID       uuid.UUID
	Email        string
	Name         string
	AvatarURL    *string
	RoleID       uuid.UUID
	RoleName     string
	RoleKey      *string
	UsesSeat     bool
	Status       string
	Availability string
	LastActiveAt time.Time
	JoinedAt     time.Time
	TeamIDs      []uuid.UUID
}

// MemberFilter narrows the members list.
type MemberFilter struct {
	RoleID   *uuid.UUID
	TeamID   *uuid.UUID
	Status   *string
	Query    *string
	After    *MemberCursor
	PageSize int32
}

// MemberCursor is the last row of the previous page.
type MemberCursor struct {
	JoinedAt time.Time
	UserID   uuid.UUID
}

// MemberDetail is the member page: who they are and what they can do.
type MemberDetail struct {
	Member
	ChatScope     *string
	RoleChatScope string
	SuspendedAt   *time.Time
	SuspendReason *string
	AccessVersion int32
	NumberIDs     []uuid.UUID
	Overrides     []Override
	// Effective is the role's permissions plus active grants, minus active denies.
	Effective []string
}

// AccessInput changes a member's reach. A nil field is left as it is.
type AccessInput struct {
	// ChatScope "" means use the role's scope.
	ChatScope *string
	TeamIDs   []uuid.UUID
	NumberIDs []uuid.UUID
	SetTeams  bool
	SetNumber bool
}

// RemoveInput confirms a removal.
type RemoveInput struct {
	ConfirmName string
}
