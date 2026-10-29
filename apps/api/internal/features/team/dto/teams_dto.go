package dto

import (
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/model"
)

// ---------- teams ----------

type TeamDTO struct {
	ID            uuid.UUID   `json:"id"`
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	Color         string      `json:"color" enum:"green,blue,amber,red,gray"`
	NumberID      *uuid.UUID  `json:"number_id" nullable:"true"`
	RoutingMethod string      `json:"routing_method" enum:"round_robin,least_busy,manual"`
	MemberIDs     []uuid.UUID `json:"member_ids"`
}

func ToTeamDTO(t model.Team) TeamDTO {
	d := TeamDTO(t)
	d.MemberIDs = nonNilIDs(d.MemberIDs)
	return d
}

type TeamsOutput struct {
	Body struct {
		Teams []TeamDTO `json:"teams"`
	}
}

type TeamOutput struct {
	Body TeamDTO
}

type teamBody struct {
	Name          string     `json:"name" minLength:"1" maxLength:"50"`
	Description   string     `json:"description,omitempty" maxLength:"200"`
	Color         string     `json:"color,omitempty" enum:"green,blue,amber,red,gray"`
	NumberID      *uuid.UUID `json:"number_id,omitempty"`
	RoutingMethod string     `json:"routing_method,omitempty" enum:"round_robin,least_busy,manual"`
}

func (b teamBody) Input() model.Details {
	return model.Details(b)
}

type CreateTeamInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	MetaHeaders
	Body teamBody
}

type UpdateTeamInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	TeamID      uuid.UUID `path:"teamId"`
	MetaHeaders
	Body teamBody
}

type TeamMembersInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	TeamID      uuid.UUID `path:"teamId"`
	MetaHeaders
	Body struct {
		UserIDs []uuid.UUID `json:"user_ids" maxItems:"5000"`
	}
}

type DeleteTeamInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	TeamID      uuid.UUID `path:"teamId"`
	MetaHeaders
}
