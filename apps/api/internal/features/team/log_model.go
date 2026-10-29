package team

import (
	"time"

	"github.com/google/uuid"
)

// LogSummary is the access log's headline numbers.
type LogSummary struct {
	Events            int64
	PermissionChanges int64
	Expiring          []Expiring
	Alerts            []LoginAlert
}

// Expiring is a temporary permission ending soon.
type Expiring struct {
	UserID        uuid.UUID
	Name          string
	PermissionKey string
	ExpiresAt     time.Time
}

// LoginAlert is a member with repeated failed sign-ins.
type LoginAlert struct {
	UserID   uuid.UUID
	Name     string
	Failures int64
	LastAt   time.Time
}

// LogEntry is one row of the access log.
type LogEntry struct {
	ID           uuid.UUID
	CreatedAt    time.Time
	ActorID      *uuid.UUID
	ActorName    string
	SubjectID    *uuid.UUID
	SubjectName  string
	SubjectEmail *string
	Action       string
	Target       string
	Before       []byte
	After        []byte
	Note         string
	IP           string
	UserAgent    string
}

// LogFilter narrows the access log.
type LogFilter struct {
	Action    *string
	SubjectID *uuid.UUID
	Since     time.Time
	Before    *LogCursor
	PageSize  int32
}

// LogCursor is the last row of the previous page.
type LogCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}
