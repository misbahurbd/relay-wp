package dto

import (
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
)

type InboxTemplateButtonDTO struct {
	Type  string `json:"type" enum:"quick_reply,url,phone"`
	Text  string `json:"text"`
	Value string `json:"value"`
}

type InboxTemplateDTO struct {
	ID             uuid.UUID                `json:"id"`
	Name           string                   `json:"name"`
	Language       string                   `json:"language"`
	Category       string                   `json:"category" enum:"marketing,utility,authentication"`
	HeaderType     string                   `json:"header_type" enum:"none,text,image,video,document"`
	HeaderText     string                   `json:"header_text"`
	Body           string                   `json:"body"`
	Footer         string                   `json:"footer"`
	Buttons        []InboxTemplateButtonDTO `json:"buttons"`
	BodyVariables  int                      `json:"body_variables" doc:"params.body needs this many values"`
	HeaderVariable bool                     `json:"header_variable" doc:"params.header_text is needed"`
	HeaderMedia    bool                     `json:"header_media" doc:"media_key is needed, a file of header_type"`
	URLButtons     []int                    `json:"url_buttons" doc:"Indexes of buttons whose link needs a value in params.url_buttons"`
}

type ListTemplatesInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	ChannelType string    `query:"channel_type" enum:"whatsapp,messenger,instagram" default:"whatsapp" doc:"The channel to send on; 409 templates_unsupported when it has no templates"`
}

type InboxTemplatesOutput struct {
	Body struct {
		Items []InboxTemplateDTO `json:"items"`
	}
}

func ToTemplatesOutput(list []model.TemplateOption) *InboxTemplatesOutput {
	out := &InboxTemplatesOutput{}
	out.Body.Items = make([]InboxTemplateDTO, len(list))
	for i, t := range list {
		d := InboxTemplateDTO{
			ID: t.ID, Name: t.Name, Language: t.Language, Category: t.Category, HeaderType: t.HeaderType,
			HeaderText: t.HeaderText, Body: t.Body, Footer: t.Footer, Buttons: make([]InboxTemplateButtonDTO, len(t.Buttons)),
			BodyVariables: t.BodyVariables, HeaderVariable: t.HeaderVariable, HeaderMedia: t.HeaderMedia, URLButtons: t.URLButtons,
		}
		if d.HeaderType == "" {
			d.HeaderType = "none"
		}
		for j, b := range t.Buttons {
			d.Buttons[j] = InboxTemplateButtonDTO(b)
		}
		out.Body.Items[i] = d
	}
	return out
}
