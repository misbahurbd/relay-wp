package team

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ---------- access log ----------

type logDTO struct {
	ID           uuid.UUID       `json:"id"`
	CreatedAt    time.Time       `json:"created_at"`
	ActorID      *uuid.UUID      `json:"actor_id" doc:"Null for the system" nullable:"true"`
	ActorName    string          `json:"actor_name"`
	SubjectID    *uuid.UUID      `json:"subject_id" nullable:"true"`
	SubjectName  string          `json:"subject_name"`
	SubjectEmail *string         `json:"subject_email"`
	Action       string          `json:"action"`
	Target       string          `json:"target"`
	Before       json.RawMessage `json:"before" nullable:"true"`
	After        json.RawMessage `json:"after" nullable:"true"`
	Note         string          `json:"note"`
	IP           string          `json:"ip"`
	UserAgent    string          `json:"user_agent"`
}

type listLogInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	Action      string    `query:"action" maxLength:"40"`
	SubjectID   string    `query:"member" format:"uuid"`
	Since       time.Time `query:"since" doc:"Oldest entry to include; at most 12 months back"`
	Cursor      string    `query:"cursor"`
	PageSize    int32     `query:"page_size" minimum:"1" maximum:"200" default:"50"`
}

type logOutput struct {
	Body struct {
		Entries    []logDTO `json:"entries"`
		NextCursor *string  `json:"next_cursor"`
	}
}

func toLogDTO(e LogEntry) logDTO {
	return logDTO{
		ID: e.ID, CreatedAt: e.CreatedAt, ActorID: e.ActorID, ActorName: e.ActorName, SubjectID: e.SubjectID,
		SubjectName: e.SubjectName, SubjectEmail: e.SubjectEmail, Action: e.Action, Target: e.Target,
		Before: rawOrNil(e.Before), After: rawOrNil(e.After), Note: e.Note, IP: e.IP, UserAgent: e.UserAgent,
	}
}

type expiringDTO struct {
	UserID        uuid.UUID `json:"user_id"`
	Name          string    `json:"name"`
	PermissionKey string    `json:"permission_key"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type loginAlertDTO struct {
	UserID   uuid.UUID `json:"user_id"`
	Name     string    `json:"name"`
	Failures int64     `json:"failures"`
	LastAt   time.Time `json:"last_at"`
}

type logSummaryOutput struct {
	Body struct {
		Events            int64           `json:"events" doc:"Access log entries in the last 30 days"`
		PermissionChanges int64           `json:"permission_changes"`
		Expiring          []expiringDTO   `json:"expiring" doc:"Temporary permissions ending within 7 days"`
		Alerts            []loginAlertDTO `json:"alerts" doc:"Members with 3+ failed sign-ins in 30 days"`
	}
}
