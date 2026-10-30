// Package dto holds the inbox's HTTP request and response shapes and the converters from domain
// types. Type names start with Inbox so they don't clash with other features' in the OpenAPI spec.
package dto

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
)

type InboxWorkspaceInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
}

type InboxPersonDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func toPerson(p *model.Person) *InboxPersonDTO {
	if p == nil {
		return nil
	}
	return &InboxPersonDTO{ID: p.ID, Name: p.Name}
}

type cursor struct {
	At *time.Time `json:"a,omitempty"`
	ID uuid.UUID  `json:"i"`
}

// EncodeCursor packs where the next page starts into an opaque string.
func EncodeCursor(c model.Cursor) string {
	b, _ := json.Marshal(cursor(c))
	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodeCursor reads a cursor made by EncodeCursor.
func DecodeCursor(s string) (model.Cursor, bool) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return model.Cursor{}, false
	}
	var c cursor
	if err := json.Unmarshal(b, &c); err != nil || c.ID == uuid.Nil {
		return model.Cursor{}, false
	}
	return model.Cursor(c), true
}
