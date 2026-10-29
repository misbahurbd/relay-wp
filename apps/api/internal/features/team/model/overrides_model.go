package model

import (
	"time"

	"github.com/google/uuid"
)

// Override is one person's extra (grant) or removed (deny) permission.
type Override struct {
	ID            uuid.UUID
	PermissionKey string
	Effect        string
	Reason        string
	ExpiresAt     *time.Time
	RevokedAt     *time.Time
	CreatedAt     time.Time
	GrantedBy     *uuid.UUID
	GrantedByName string
}

// Active reports whether the override applies now.
func (o Override) Active(now time.Time) bool {
	return o.RevokedAt == nil && (o.ExpiresAt == nil || o.ExpiresAt.After(now))
}

// OverrideInput grants or denies one permission to one member.
type OverrideInput struct {
	PermissionKey string
	Effect        string
	ExpiresAt     *time.Time
	Reason        string
}
