package dto

import (
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/model"

	"github.com/google/uuid"
)

type LeaveInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	MetaHeaders
}

// ---------- members ----------

// MemberDTO is exported so huma flattens it into memberDetailDTO (it skips unexported embeds).
type MemberDTO struct {
	UserID       uuid.UUID   `json:"user_id"`
	Email        string      `json:"email"`
	Name         string      `json:"name"`
	AvatarURL    *string     `json:"avatar_url"`
	RoleID       uuid.UUID   `json:"role_id"`
	RoleName     string      `json:"role_name"`
	RoleKey      *string     `json:"role_key" nullable:"true" enum:"owner,admin,agent,viewer" doc:"Null for a custom role"`
	UsesSeat     bool        `json:"uses_seat"`
	Status       string      `json:"status" enum:"active,suspended"`
	Availability string      `json:"availability" enum:"online,away,offline"`
	LastActiveAt time.Time   `json:"last_active_at"`
	JoinedAt     time.Time   `json:"joined_at"`
	TeamIDs      []uuid.UUID `json:"team_ids"`
}

func ToMemberDTO(m model.Member) MemberDTO {
	d := MemberDTO(m)
	d.TeamIDs = nonNilIDs(d.TeamIDs)
	return d
}

type ListMembersInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	RoleID      string    `query:"role_id" format:"uuid"`
	TeamID      string    `query:"team_id" format:"uuid"`
	Status      string    `query:"status" enum:"active,suspended"`
	Query       string    `query:"q" maxLength:"100"`
	Cursor      string    `query:"cursor" doc:"next_cursor from the previous page"`
	PageSize    int32     `query:"page_size" minimum:"1" maximum:"500" default:"100"`
}

type MembersOutput struct {
	Body struct {
		Members    []MemberDTO `json:"members"`
		NextCursor *string     `json:"next_cursor"`
	}
}

type MemberDetailDTO struct {
	MemberDTO
	ChatScope     *string       `json:"chat_scope" nullable:"true" enum:"assigned,team,all" doc:"Null means the role's scope"`
	RoleChatScope string        `json:"role_chat_scope" enum:"assigned,team,all"`
	SuspendedAt   *time.Time    `json:"suspended_at"`
	SuspendReason *string       `json:"suspend_reason"`
	AccessVersion int32         `json:"access_version"`
	NumberIDs     []uuid.UUID   `json:"number_ids"`
	Overrides     []overrideDTO `json:"overrides" doc:"Empty unless you can change roles"`
	Effective     []string      `json:"effective_permissions"`
}

type MemberOutput struct {
	Body MemberDetailDTO
}

func ToMemberDetailDTO(d model.MemberDetail, now time.Time) MemberDetailDTO {
	out := MemberDetailDTO{
		MemberDTO: ToMemberDTO(d.Member), ChatScope: d.ChatScope, RoleChatScope: d.RoleChatScope,
		SuspendedAt: d.SuspendedAt, SuspendReason: d.SuspendReason, AccessVersion: d.AccessVersion,
		NumberIDs: nonNilIDs(d.NumberIDs), Overrides: []overrideDTO{}, Effective: NonNil(d.Effective),
	}
	for _, o := range d.Overrides {
		out.Overrides = append(out.Overrides, overrideDTO{
			ID: o.ID, PermissionKey: o.PermissionKey, Effect: o.Effect, Reason: o.Reason, ExpiresAt: o.ExpiresAt,
			RevokedAt: o.RevokedAt, CreatedAt: o.CreatedAt, GrantedBy: o.GrantedBy, GrantedByName: o.GrantedByName,
			Active: o.Active(now),
		})
	}
	return out
}

type MemberRoleInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	UserID      uuid.UUID `path:"userId"`
	MetaHeaders
	Body struct {
		RoleID uuid.UUID `json:"role_id"`
	}
}

type MemberAccessInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	UserID      uuid.UUID `path:"userId"`
	MetaHeaders
	Body struct {
		ChatScope *string      `json:"chat_scope,omitempty" doc:"\"assigned\", \"team\" or \"all\"; \"\" to use the role's scope; omit to keep"`
		TeamIDs   *[]uuid.UUID `json:"team_ids,omitempty" doc:"Replaces the member's teams; omit to keep"`
		NumberIDs *[]uuid.UUID `json:"number_ids,omitempty" doc:"Replaces the member's WhatsApp numbers; omit to keep"`
	}
}

type SuspendInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	UserID      uuid.UUID `path:"userId"`
	MetaHeaders
	Body struct {
		Reason string `json:"reason,omitempty" maxLength:"200"`
	}
}

type MemberActionInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	UserID      uuid.UUID `path:"userId"`
	MetaHeaders
}

type RemoveInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	UserID      uuid.UUID `path:"userId"`
	MetaHeaders
	Body struct {
		ConfirmName string `json:"confirm_name" doc:"The member's name, typed to confirm"`
	}
}
