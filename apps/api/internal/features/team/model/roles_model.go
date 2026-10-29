package model

import (
	"github.com/google/uuid"
)

// Role is a named set of permissions plus a default chat scope.
type Role struct {
	ID          uuid.UUID
	SystemKey   *string
	Name        string
	Description string
	Color       string
	ChatScope   string
	UsesSeat    bool
	Locked      bool
	Version     int32
	Permissions []string
	MemberCount int64
}

// RoleInput is a create or edit from the Roles editor.
type RoleInput struct {
	Name        string
	Description string
	Color       string
	ChatScope   string
	Permissions []string
	// CopyFrom starts a new role from another role's permissions and scope.
	CopyFrom *uuid.UUID
	// ExpectedVersion guards an edit against a change saved in the meantime.
	ExpectedVersion int32
	NotifyMembers   bool
}

// RoleChange is what a role edit would give and take away, shown before saving.
type RoleChange struct {
	Gains         []string
	Losses        []string
	SensitiveGain bool
	Affected      []MemberRef
}

// MemberRef names a member in a preview or log.
type MemberRef struct {
	UserID uuid.UUID
	Name   string
}
