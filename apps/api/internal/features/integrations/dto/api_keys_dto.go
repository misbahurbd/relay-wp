package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
)

type userRefDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type APIKeyDTO struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name" example:"Website orders"`
	App        *string    `json:"app" nullable:"true" doc:"The no-code app it was made for: zapier, make or n8n"`
	Prefix     string     `json:"prefix" example:"relay_live_AbCd" doc:"Start of the key"`
	Last4      string     `json:"last4" example:"7f3a" doc:"End of the key"`
	CreatedBy  userRefDTO `json:"created_by" doc:"The key acts as this member"`
	LastUsedAt *time.Time `json:"last_used_at" nullable:"true"`
	CreatedAt  time.Time  `json:"created_at"`
}

func ToAPIKeyDTO(k model.APIKey) APIKeyDTO {
	return APIKeyDTO{
		ID: k.ID, Name: k.Name, App: k.App, Prefix: k.Prefix, Last4: k.Last4,
		CreatedBy: userRefDTO{ID: k.CreatedBy, Name: k.CreatedByName}, LastUsedAt: k.LastUsedAt, CreatedAt: k.CreatedAt,
	}
}

type APIKeysOutput struct {
	Body struct {
		Keys []APIKeyDTO `json:"keys"`
	}
}

type CreateAPIKeyInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Body        struct {
		Name string  `json:"name" minLength:"1" maxLength:"80"`
		App  *string `json:"app,omitempty" enum:"zapier,make,n8n" doc:"Tag the key with the no-code app it's for"`
	}
}

type APIKeyInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	KeyID       uuid.UUID `path:"keyId"`
}

type NewAPIKeyOutput struct {
	Body struct {
		APIKey APIKeyDTO `json:"api_key"`
		Key    string    `json:"key" doc:"The full key. Shown only now; store it somewhere safe."`
	}
}

func ToNewAPIKeyOutput(k model.NewAPIKey) *NewAPIKeyOutput {
	out := &NewAPIKeyOutput{}
	out.Body.APIKey, out.Body.Key = ToAPIKeyDTO(k.APIKey), k.Key
	return out
}
