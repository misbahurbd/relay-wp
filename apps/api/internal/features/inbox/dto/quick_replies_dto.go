package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
)

type InboxQuickReplyDTO struct {
	ID        uuid.UUID `json:"id"`
	Shortcut  string    `json:"shortcut" doc:"Typed after / in the composer"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type InboxQuickRepliesOutput struct {
	Body struct {
		Items []InboxQuickReplyDTO `json:"items"`
	}
}

func ToQuickRepliesOutput(list []model.QuickReply) *InboxQuickRepliesOutput {
	out := &InboxQuickRepliesOutput{}
	out.Body.Items = make([]InboxQuickReplyDTO, len(list))
	for i, q := range list {
		out.Body.Items[i] = InboxQuickReplyDTO(q)
	}
	return out
}

type InboxQuickReplyOutput struct {
	Body InboxQuickReplyDTO
}

func ToQuickReplyOutput(q model.QuickReply) *InboxQuickReplyOutput {
	return &InboxQuickReplyOutput{Body: InboxQuickReplyDTO(q)}
}

type CreateQuickReplyInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Body        struct {
		Shortcut string `json:"shortcut" maxLength:"33" doc:"1 to 32 of a-z, 0-9, - and _; a leading / is dropped"`
		Title    string `json:"title" maxLength:"80"`
		Body     string `json:"body" maxLength:"1024"`
	}
}

func (in *CreateQuickReplyInput) ToModel() model.QuickReplyInput {
	return model.QuickReplyInput{Shortcut: &in.Body.Shortcut, Title: &in.Body.Title, Body: &in.Body.Body}
}

type QuickReplyPath struct {
	WorkspaceID  uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	QuickReplyID uuid.UUID `path:"quickReplyId"`
}

type UpdateQuickReplyInput struct {
	WorkspaceID  uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	QuickReplyID uuid.UUID `path:"quickReplyId"`
	Body         struct {
		Shortcut *string `json:"shortcut,omitempty" maxLength:"33"`
		Title    *string `json:"title,omitempty" maxLength:"80"`
		Body     *string `json:"body,omitempty" maxLength:"1024"`
	}
}

func (in *UpdateQuickReplyInput) ToModel() model.QuickReplyInput {
	return model.QuickReplyInput{Shortcut: in.Body.Shortcut, Title: in.Body.Title, Body: in.Body.Body}
}
