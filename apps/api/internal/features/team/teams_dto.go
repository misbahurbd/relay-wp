package team

import (
	"github.com/google/uuid"
)

// ---------- teams ----------

type teamDTO struct {
	ID            uuid.UUID   `json:"id"`
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	Color         string      `json:"color" enum:"green,blue,amber,red,gray"`
	NumberID      *uuid.UUID  `json:"number_id" nullable:"true"`
	RoutingMethod string      `json:"routing_method" enum:"round_robin,least_busy,manual"`
	MemberIDs     []uuid.UUID `json:"member_ids"`
}

func toTeamDTO(t Team) teamDTO {
	d := teamDTO(t)
	d.MemberIDs = nonNilIDs(d.MemberIDs)
	return d
}

type teamsOutput struct {
	Body struct {
		Teams []teamDTO `json:"teams"`
	}
}

type teamOutput struct {
	Body teamDTO
}

type teamBody struct {
	Name          string     `json:"name" minLength:"1" maxLength:"50"`
	Description   string     `json:"description,omitempty" maxLength:"200"`
	Color         string     `json:"color,omitempty" enum:"green,blue,amber,red,gray"`
	NumberID      *uuid.UUID `json:"number_id,omitempty"`
	RoutingMethod string     `json:"routing_method,omitempty" enum:"round_robin,least_busy,manual"`
}

func (b teamBody) input() Details {
	return Details(b)
}

type createTeamInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	metaHeaders
	Body teamBody
}

type updateTeamInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	TeamID      uuid.UUID `path:"teamId"`
	metaHeaders
	Body teamBody
}

type teamMembersInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	TeamID      uuid.UUID `path:"teamId"`
	metaHeaders
	Body struct {
		UserIDs []uuid.UUID `json:"user_ids" maxItems:"5000"`
	}
}

type deleteTeamInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	TeamID      uuid.UUID `path:"teamId"`
	metaHeaders
}
