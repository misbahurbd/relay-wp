package dto

import (
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/model"

	"github.com/google/uuid"
)

type overrideDTO struct {
	ID            uuid.UUID  `json:"id"`
	PermissionKey string     `json:"permission_key"`
	Effect        string     `json:"effect" enum:"grant,deny"`
	Reason        string     `json:"reason"`
	ExpiresAt     *time.Time `json:"expires_at"`
	RevokedAt     *time.Time `json:"revoked_at"`
	CreatedAt     time.Time  `json:"created_at"`
	GrantedBy     *uuid.UUID `json:"granted_by" nullable:"true"`
	GrantedByName string     `json:"granted_by_name"`
	Active        bool       `json:"active"`
}

type overrideBody struct {
	PermissionKey string     `json:"permission_key"`
	Effect        string     `json:"effect" enum:"grant,deny" default:"grant"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty" doc:"Removed automatically at this time"`
	Reason        string     `json:"reason,omitempty" maxLength:"200" doc:"Shown in the access log"`
}

func (b overrideBody) Input() model.OverrideInput {
	return model.OverrideInput(b)
}

type AddOverrideInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	UserID      uuid.UUID `path:"userId"`
	MetaHeaders
	Body overrideBody
}

type RevokeOverrideInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	UserID      uuid.UUID `path:"userId"`
	OverrideID  uuid.UUID `path:"overrideId"`
	MetaHeaders
}
