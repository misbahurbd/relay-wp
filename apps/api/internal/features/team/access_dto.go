package team

import (
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// ---------- access ----------

type accessDTO struct {
	RoleID        uuid.UUID   `json:"role_id"`
	IsOwner       bool        `json:"is_owner"`
	Permissions   []string    `json:"permissions" doc:"Every permission key the member has now"`
	ChatScope     string      `json:"chat_scope" enum:"assigned,team,all"`
	TeamIDs       []uuid.UUID `json:"team_ids"`
	NumberIDs     []uuid.UUID `json:"number_ids"`
	AccessVersion string      `json:"access_version" doc:"Same value as the X-Access-Version header"`
}

type accessOutput struct {
	Body accessDTO
}

func toAccessDTO(a authctx.Access) accessDTO {
	bits := a.Bits
	if a.Owner {
		bits = authctx.AllBits
	}
	return accessDTO{
		RoleID: a.RoleID, IsOwner: a.Owner, Permissions: bits.Keys(), ChatScope: a.Scope,
		TeamIDs: nonNilIDs(a.Teams), NumberIDs: nonNilIDs(a.Numbers),
		AccessVersion: itoa(a.RoleVersion) + "." + itoa(a.AccessVersion),
	}
}

type availabilityInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	Body        struct {
		Availability string `json:"availability" enum:"online,away,offline"`
	}
}

type permissionDTO struct {
	Key       string `json:"key" example:"contacts.export"`
	Group     string `json:"group" example:"contacts"`
	Label     string `json:"label" example:"Export contacts"`
	Sensitive bool   `json:"sensitive" doc:"Show a warning before giving it"`
}

type permissionsOutput struct {
	Body struct {
		Permissions []permissionDTO `json:"permissions"`
	}
}

type seatsDTO struct {
	Limit     int32 `json:"limit"`
	Used      int64 `json:"used" doc:"Members whose role uses a seat"`
	Reserved  int64 `json:"reserved" doc:"Pending invites whose role uses a seat"`
	Available int64 `json:"available"`
}

type seatsOutput struct {
	Body seatsDTO
}
