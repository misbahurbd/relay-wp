package dto

import (
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
)

type TagDTO struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	ContactCount int64     `json:"contact_count"`
}

func ToTagDTO(t model.Tag) TagDTO { return TagDTO(t) }

type TagsOutput struct {
	Body []TagDTO
}

type CreateTagInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Body        struct {
		Name string `json:"name" minLength:"1" maxLength:"50"`
	}
}

type TagOutput struct {
	Status int `doc:"201 when created, 200 when a tag with this name already existed"`
	Body   TagDTO
}
