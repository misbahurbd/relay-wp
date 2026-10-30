package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
)

type InboxMentionDTO struct {
	MessageID      uuid.UUID       `json:"message_id" doc:"The note"`
	ConversationID uuid.UUID       `json:"conversation_id"`
	ContactName    string          `json:"contact_name"`
	ContactPhone   string          `json:"contact_phone"`
	Body           string          `json:"body" doc:"The note, at most 280 characters"`
	Author         *InboxPersonDTO `json:"author"`
	SeenAt         *time.Time      `json:"seen_at"`
	CreatedAt      time.Time       `json:"created_at"`
}

type InboxMentionsOutput struct {
	Body struct {
		Items  []InboxMentionDTO `json:"items" doc:"Your 50 newest mentions"`
		Unseen int               `json:"unseen" doc:"For the bell"`
	}
}

func ToMentionsOutput(list []model.Mention, unseen int) *InboxMentionsOutput {
	out := &InboxMentionsOutput{}
	out.Body.Unseen = unseen
	out.Body.Items = make([]InboxMentionDTO, len(list))
	for i, m := range list {
		out.Body.Items[i] = InboxMentionDTO{
			MessageID: m.MessageID, ConversationID: m.ConversationID, ContactName: m.Contact.Name,
			ContactPhone: m.Contact.Phone, Body: m.Body, Author: toPerson(m.Author), SeenAt: m.SeenAt, CreatedAt: m.CreatedAt,
		}
	}
	return out
}

type MarkMentionsSeenInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Body        struct {
		MessageIDs []uuid.UUID `json:"message_ids,omitempty" maxItems:"100" doc:"Only these notes' mentions; omit for all"`
	}
}

type InboxMentionsSeenOutput struct {
	Body struct {
		Updated int64 `json:"updated"`
	}
}
