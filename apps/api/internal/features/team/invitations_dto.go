package team

import (
	"time"

	"github.com/google/uuid"
)

// ---------- invitations ----------

type invitationDTO struct {
	ID                 uuid.UUID   `json:"id"`
	Email              string      `json:"email"`
	RoleID             uuid.UUID   `json:"role_id"`
	RoleName           string      `json:"role_name"`
	Status             string      `json:"status" enum:"pending,accepted,revoked,expired"`
	ExpiresAt          time.Time   `json:"expires_at"`
	LastSentAt         time.Time   `json:"last_sent_at"`
	SendCount          int32       `json:"send_count"`
	RenewalRequestedAt *time.Time  `json:"renewal_requested_at" doc:"The invitee asked for a new link"`
	CreatedAt          time.Time   `json:"created_at"`
	InvitedByName      string      `json:"invited_by_name"`
	TeamIDs            []uuid.UUID `json:"team_ids"`
}

func toInvitationDTO(i Invitation) invitationDTO {
	d := invitationDTO(i)
	d.TeamIDs = nonNilIDs(d.TeamIDs)
	return d
}

type invitationsOutput struct {
	Body struct {
		Invitations []invitationDTO `json:"invitations"`
	}
}

type inviteInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	metaHeaders
	Body struct {
		Emails          []string       `json:"emails" minItems:"1" maxItems:"50"`
		RoleID          uuid.UUID      `json:"role_id"`
		TeamIDs         []uuid.UUID    `json:"team_ids,omitempty"`
		NumberIDs       []uuid.UUID    `json:"number_ids,omitempty"`
		ChatScope       *string        `json:"chat_scope,omitempty" enum:"assigned,team,all" doc:"Omit to use the role's scope"`
		Overrides       []overrideBody `json:"overrides,omitempty" doc:"Changes for these people only; the role stays the same"`
		PersonalMessage string         `json:"personal_message,omitempty" maxLength:"500"`
		ExpiresIn       string         `json:"expires_in,omitempty" enum:"24h,7d,30d" default:"7d"`
		Require2FA      bool           `json:"require_2fa,omitempty"`
	}
}

type invitationActionInput struct {
	WorkspaceID  uuid.UUID `path:"workspaceId"`
	InvitationID uuid.UUID `path:"invitationId"`
	metaHeaders
}

type tokenInput struct {
	metaHeaders
	Body struct {
		Token string `json:"token" minLength:"20" maxLength:"100" doc:"From the invite link's #t= fragment"`
	}
}

type previewDTO struct {
	State         string    `json:"state" enum:"valid,expired,revoked,accepted"`
	WorkspaceID   uuid.UUID `json:"workspace_id"`
	WorkspaceName string    `json:"workspace_name"`
	Email         string    `json:"email" doc:"The address the invite is for"`
	RoleName      string    `json:"role_name"`
	InvitedByName string    `json:"invited_by_name"`
	InvitedByRole string    `json:"invited_by_role"`
	ExpiresAt     time.Time `json:"expires_at"`
	Message       string    `json:"message"`
	CanDo         []string  `json:"can_do" doc:"Labels of what they'll be able to do; empty unless valid"`
	Permissions   []string  `json:"permissions"`
	TeamNames     []string  `json:"team_names"`
	NumberLabels  []string  `json:"number_labels"`
}

type previewOutput struct {
	Body previewDTO
}

type acceptOutput struct {
	Body struct {
		WorkspaceID uuid.UUID `json:"workspace_id"`
	}
}
