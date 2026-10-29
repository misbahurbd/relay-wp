package team

import (
	"github.com/google/uuid"
)

// ---------- roles ----------

type roleDTO struct {
	ID          uuid.UUID `json:"id"`
	SystemKey   *string   `json:"system_key" nullable:"true" enum:"owner,admin,agent,viewer" doc:"Null for a custom role"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color" enum:"green,blue,amber,red,gray"`
	ChatScope   string    `json:"chat_scope" enum:"assigned,team,all"`
	UsesSeat    bool      `json:"uses_seat"`
	Locked      bool      `json:"locked" doc:"The Owner role can't be edited"`
	Version     int32     `json:"version"`
	Permissions []string  `json:"permissions"`
	MemberCount int64     `json:"member_count"`
}

func toRoleDTO(r Role) roleDTO {
	d := roleDTO(r)
	if d.Permissions == nil {
		d.Permissions = []string{}
	}
	return d
}

type rolesOutput struct {
	Body struct {
		Roles []roleDTO `json:"roles"`
	}
}

type roleOutput struct {
	Body roleDTO
}

type roleBody struct {
	Name            string     `json:"name,omitempty" maxLength:"50"`
	Description     string     `json:"description,omitempty" maxLength:"200"`
	Color           string     `json:"color,omitempty" enum:"green,blue,amber,red,gray"`
	ChatScope       string     `json:"chat_scope,omitempty" enum:"assigned,team,all"`
	Permissions     []string   `json:"permissions,omitempty" doc:"Omit to keep the current set (or copy_from's)"`
	CopyFrom        *uuid.UUID `json:"copy_from_role_id,omitempty" doc:"Start a new role from another role"`
	ExpectedVersion int32      `json:"expected_version,omitempty" doc:"The version you edited; 409 if it changed"`
	NotifyMembers   bool       `json:"notify_members,omitempty"`
}

func (b roleBody) input() RoleInput {
	return RoleInput(b)
}

type createRoleInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	metaHeaders
	Body roleBody
}

type updateRoleInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	RoleID      uuid.UUID `path:"roleId"`
	metaHeaders
	Body roleBody
}

type resetRoleInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	RoleID      uuid.UUID `path:"roleId"`
	metaHeaders
}

type deleteRoleInput struct {
	WorkspaceID  uuid.UUID `path:"workspaceId"`
	RoleID       uuid.UUID `path:"roleId"`
	MoveToRoleID uuid.UUID `query:"move_to_role_id" required:"true" doc:"Where its members and pending invites go"`
	metaHeaders
}

type memberRefDTO struct {
	UserID uuid.UUID `json:"user_id"`
	Name   string    `json:"name"`
}

type changeDTO struct {
	Gains         []string       `json:"gains" doc:"Permission keys, or a chat access label"`
	Losses        []string       `json:"losses"`
	SensitiveGain bool           `json:"sensitive_gain"`
	Affected      []memberRefDTO `json:"affected"`
}

type changeOutput struct {
	Body changeDTO
}

func toChangeDTO(c RoleChange) changeDTO {
	d := changeDTO{Gains: nonNil(c.Gains), Losses: nonNil(c.Losses), SensitiveGain: c.SensitiveGain, Affected: []memberRefDTO{}}
	for _, m := range c.Affected {
		d.Affected = append(d.Affected, memberRefDTO(m))
	}
	return d
}
