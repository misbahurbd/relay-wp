package dto

import (
	"encoding/json"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
)

type InboxMediaDTO struct {
	Ready    bool   `json:"ready" doc:"The file is stored; GET …/media streams it. False while it loads from the channel"`
	Mime     string `json:"mime"`
	Size     int64  `json:"size"`
	Filename string `json:"filename"`
}

type InboxLocationDTO struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name"`
	Address   string  `json:"address"`
}

type InboxInteractiveDTO struct {
	Kind        string `json:"kind" doc:"button_reply, list_reply or button (a template's quick reply)"`
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type InboxReferralDTO struct {
	SourceType string `json:"source_type"`
	SourceID   string `json:"source_id"`
	SourceURL  string `json:"source_url"`
	Headline   string `json:"headline"`
	Body       string `json:"body"`
}

type InboxURLButtonParamDTO struct {
	Index int    `json:"index" minimum:"0" doc:"The button's position in the template"`
	Value string `json:"value" maxLength:"2000" doc:"Fills the link's {{1}}"`
}

type InboxTemplateParamsDTO struct {
	HeaderText string                   `json:"header_text,omitempty" maxLength:"60" doc:"Fills a text header's {{1}}"`
	Body       []string                 `json:"body,omitempty" maxItems:"100" doc:"Fills the body's {{1}}..{{N}} in order"`
	URLButtons []InboxURLButtonParamDTO `json:"url_buttons,omitempty" maxItems:"10"`
}

type InboxTemplateContentDTO struct {
	ID         uuid.UUID              `json:"id"`
	Name       string                 `json:"name"`
	Language   string                 `json:"language"`
	Category   string                 `json:"category"`
	HeaderType string                 `json:"header_type"`
	Params     InboxTemplateParamsDTO `json:"params"`
}

type InboxEventDTO struct {
	Action   string     `json:"action" enum:"assigned,unassigned,closed,reopened,snoozed,unsnoozed,template_pending,window_reopened,opted_out"`
	ActorID  *uuid.UUID `json:"actor_id" doc:"Null for the system"`
	TargetID *uuid.UUID `json:"target_id" doc:"The member assigned or unassigned"`
	Until    *time.Time `json:"until" doc:"Snoozed until"`
}

type InboxContentDTO struct {
	Media       *InboxMediaDTO           `json:"media,omitempty"`
	Location    *InboxLocationDTO        `json:"location,omitempty"`
	Contacts    any                      `json:"contacts,omitempty" doc:"The channel's contacts array, as the customer sent it"`
	Interactive *InboxInteractiveDTO     `json:"interactive,omitempty"`
	Template    *InboxTemplateContentDTO `json:"template,omitempty"`
	Event       *InboxEventDTO           `json:"event,omitempty"`
	Referral    *InboxReferralDTO        `json:"referral,omitempty"`
}

func toContentDTO(c model.Content) InboxContentDTO {
	out := InboxContentDTO{Event: (*InboxEventDTO)(c.Event)}
	if c.Media != nil {
		out.Media = &InboxMediaDTO{Ready: c.Media.StorageKey != "", Mime: c.Media.Mime, Size: c.Media.Size, Filename: c.Media.Filename}
	}
	if c.Location != nil {
		out.Location = (*InboxLocationDTO)(c.Location)
	}
	if len(c.Contacts) > 0 {
		var v any
		if json.Unmarshal(c.Contacts, &v) == nil {
			out.Contacts = v
		}
	}
	if c.Interactive != nil {
		out.Interactive = (*InboxInteractiveDTO)(c.Interactive)
	}
	if c.Referral != nil {
		out.Referral = (*InboxReferralDTO)(c.Referral)
	}
	if t := c.Template; t != nil {
		out.Template = &InboxTemplateContentDTO{
			ID: t.ID, Name: t.Name, Language: t.Language, Category: t.Category, HeaderType: t.HeaderType,
			Params: toParamsDTO(t.Params),
		}
		if t.HeaderMediaKey != "" {
			out.Media = &InboxMediaDTO{Ready: true}
		}
	}
	return out
}

func toParamsDTO(p model.TemplateParams) InboxTemplateParamsDTO {
	out := InboxTemplateParamsDTO{HeaderText: p.HeaderText, Body: p.Body}
	for _, b := range p.URLButtons {
		out.URLButtons = append(out.URLButtons, InboxURLButtonParamDTO(b))
	}
	return out
}

type InboxReplyRefDTO struct {
	ID        uuid.UUID `json:"id"`
	Direction string    `json:"direction" enum:"in,out"`
	Type      string    `json:"type"`
	Body      string    `json:"body" doc:"At most 120 characters"`
}

type InboxReactionsDTO struct {
	In  string `json:"in" doc:"The customer's emoji; empty for none"`
	Out string `json:"out" doc:"Your emoji; empty for none"`
}

type InboxMessageDTO struct {
	ID         uuid.UUID         `json:"id"`
	Direction  string            `json:"direction" enum:"in,out"`
	Kind       string            `json:"kind" enum:"message,note,event" doc:"note: private, never sent; event: a system line such as \"Assigned to Ada\""`
	Type       string            `json:"type" doc:"The channel's type, for WhatsApp: text, image, video, audio, document, sticker, location, contacts, interactive, button, template, unsupported; event for events"`
	Body       string            `json:"body" doc:"Text, caption or the rendered template"`
	Content    InboxContentDTO   `json:"content"`
	ReplyTo    *InboxReplyRefDTO `json:"reply_to"`
	Status     *string           `json:"status" enum:"pending,sent,delivered,read,failed" doc:"Outbound messages only"`
	ErrorCode  *string           `json:"error_code"`
	ErrorTitle *string           `json:"error_title"`
	Author     *InboxPersonDTO   `json:"author" doc:"Who sent the message or wrote the note"`
	Automated  bool              `json:"automated" doc:"An automation sent this message"`
	Mentions   []uuid.UUID       `json:"mentions"`
	Reactions  InboxReactionsDTO `json:"reactions"`
	CreatedAt  time.Time         `json:"created_at"`
	StatusAt   *time.Time        `json:"status_at"`
}

func ToMessageDTO(m model.Message) InboxMessageDTO {
	out := InboxMessageDTO{
		ID: m.ID, Direction: m.Direction, Kind: m.Kind, Type: m.Type, Body: m.Body, Content: toContentDTO(m.Content),
		Status: m.Status, ErrorCode: m.ErrorCode, ErrorTitle: m.ErrorTitle, Author: toPerson(m.Author), Automated: m.Automated,
		Mentions: m.Mentions, Reactions: InboxReactionsDTO(m.Reactions), CreatedAt: m.CreatedAt, StatusAt: m.StatusAt,
	}
	if m.ReplyTo != nil {
		out.ReplyTo = (*InboxReplyRefDTO)(m.ReplyTo)
	}
	if out.Mentions == nil {
		out.Mentions = []uuid.UUID{}
	}
	return out
}

type ListMessagesInput struct {
	WorkspaceID    uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	ConversationID uuid.UUID `path:"conversationId"`
	Before         string    `query:"before" format:"uuid" doc:"next_before from the previous page"`
}

type InboxMessagesOutput struct {
	Body struct {
		Items      []InboxMessageDTO `json:"items" doc:"Newest first, 50 per page"`
		NextBefore *uuid.UUID        `json:"next_before" doc:"Pass as before for older messages; null at the start of the chat"`
	}
}

type InboxMessageOutput struct {
	Body InboxMessageDTO
}

type SendMessageInput struct {
	WorkspaceID    uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	ConversationID uuid.UUID `path:"conversationId"`
	Body           struct {
		Type       string                 `json:"type" enum:"text,media,template" doc:"text and media need inbox.reply and the 24-hour window; template needs inbox.send_template"`
		Text       string                 `json:"text,omitempty" maxLength:"4096"`
		MediaKey   string                 `json:"media_key,omitempty" maxLength:"512" doc:"key from POST /media: the file to send, or a template's media header"`
		Caption    string                 `json:"caption,omitempty" maxLength:"1024"`
		ReplyToID  *uuid.UUID             `json:"reply_to_id,omitempty" doc:"Quote a message of this chat"`
		TemplateID *uuid.UUID             `json:"template_id,omitempty" doc:"An approved template, from GET /templates"`
		Params     InboxTemplateParamsDTO `json:"params,omitempty" doc:"The template's variables"`
	}
}

func (in *SendMessageInput) ToModel() model.SendInput {
	b := in.Body
	out := model.SendInput{
		Type: b.Type, Text: b.Text, MediaKey: b.MediaKey, Caption: b.Caption, ReplyToID: b.ReplyToID, TemplateID: b.TemplateID,
		Params: model.TemplateParams{HeaderText: b.Params.HeaderText, Body: b.Params.Body},
	}
	for _, u := range b.Params.URLButtons {
		out.Params.URLButtons = append(out.Params.URLButtons, model.URLButtonParam(u))
	}
	return out
}

type AddNoteInput struct {
	WorkspaceID    uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	ConversationID uuid.UUID `path:"conversationId"`
	Body           struct {
		Text     string      `json:"text" minLength:"1" maxLength:"4096"`
		Mentions []uuid.UUID `json:"mentions,omitempty" maxItems:"20" doc:"Members to notify; each must be able to see this chat"`
	}
}

type MessagePath struct {
	WorkspaceID    uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	ConversationID uuid.UUID `path:"conversationId"`
	MessageID      uuid.UUID `path:"messageId"`
}

type ReactInput struct {
	WorkspaceID    uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	ConversationID uuid.UUID `path:"conversationId"`
	MessageID      uuid.UUID `path:"messageId"`
	Body           struct {
		Emoji string `json:"emoji" maxLength:"32" doc:"One emoji; empty removes your reaction"`
	}
}

type UploadMediaForm struct {
	File huma.FormFile `form:"file" required:"true" doc:"Image (JPEG, PNG), video (MP4, 3GP), audio (AAC, AMR, MP3, M4A, OGG) or document (PDF, Office, TXT), within the largest limit of any channel (WhatsApp: 5, 16, 16 and 100 MB). The send checks the conversation's channel's limits"`
}

type UploadMediaInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	RawBody     huma.MultipartFormFiles[UploadMediaForm]
}

type InboxUploadOutput struct {
	Body struct {
		Key      string `json:"key" doc:"Send as media_key"`
		Mime     string `json:"mime"`
		Size     int64  `json:"size"`
		Filename string `json:"filename"`
		Kind     string `json:"kind" enum:"image,video,audio,document" doc:"How the channel will show it"`
	}
}

func ToUploadOutput(u model.Upload) *InboxUploadOutput {
	out := &InboxUploadOutput{}
	out.Body.Key, out.Body.Mime, out.Body.Size, out.Body.Filename, out.Body.Kind = u.Key, u.Mime, u.Size, u.Filename, u.Kind
	return out
}
