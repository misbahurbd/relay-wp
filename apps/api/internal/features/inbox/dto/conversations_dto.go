package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
)

type InboxContactDTO struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name" doc:"Empty when the customer has no profile name; show the phone"`
	Phone string    `json:"phone" doc:"E.164" example:"+447700900412"`
}

type InboxChannelDTO struct {
	Type   string    `json:"type" enum:"whatsapp,messenger,instagram"`
	ID     uuid.UUID `json:"id" doc:"The channel's account: the WhatsApp number's id for whatsapp"`
	Name   string    `json:"name" doc:"The account's name (WhatsApp: the number's verified name); empty when the account is gone"`
	Handle string    `json:"handle" doc:"The account's handle (WhatsApp: the display phone); empty when the account is gone"`
}

type InboxMediaKindDTO struct {
	Kind      string   `json:"kind" enum:"image,video,audio,document"`
	MaxBytes  int64    `json:"max_bytes"`
	MimeTypes []string `json:"mime_types" doc:"Accepted types; empty accepts any type the inbox can send"`
}

type InboxCapabilitiesDTO struct {
	WindowHours   int                 `json:"window_hours" doc:"Hours after the customer's last message that free-form messages are allowed; 0 means no window"`
	Templates     bool                `json:"templates" doc:"Approved templates can be sent"`
	Reactions     bool                `json:"reactions"`
	ReadReceipts  bool                `json:"read_receipts"`
	Media         []InboxMediaKindDTO `json:"media" doc:"File kinds an agent can send, with their limits"`
	MaxTextLength int                 `json:"max_text_length"`
}

func toCapabilities(c model.Capabilities) InboxCapabilitiesDTO {
	out := InboxCapabilitiesDTO{
		WindowHours: int(c.Window / time.Hour), Templates: c.Templates, Reactions: c.Reactions, ReadReceipts: c.ReadReceipts,
		Media: make([]InboxMediaKindDTO, len(c.Media)), MaxTextLength: c.MaxTextLen,
	}
	for i, m := range c.Media {
		types := m.MimeTypes
		if types == nil {
			types = []string{}
		}
		out.Media[i] = InboxMediaKindDTO{Kind: m.Kind, MaxBytes: m.MaxBytes, MimeTypes: types}
	}
	return out
}

type InboxConversationDTO struct {
	ID                   uuid.UUID       `json:"id"`
	Contact              InboxContactDTO `json:"contact"`
	Channel              InboxChannelDTO `json:"channel" doc:"The channel account the chat is on"`
	Status               string          `json:"status" enum:"open,pending,closed"`
	Source               string          `json:"source" enum:"organic,ctwa_ad,template"`
	TeamID               *uuid.UUID      `json:"team_id"`
	Assignee             *InboxPersonDTO `json:"assignee"`
	LastMessageAt        time.Time       `json:"last_message_at"`
	LastMessagePreview   string          `json:"last_message_preview" doc:"At most 120 characters"`
	LastMessageDirection string          `json:"last_message_direction" enum:"in,out"`
	WindowExpiresAt      *time.Time      `json:"window_expires_at" doc:"Free-form replies are allowed until then; null before the customer has written, or on a channel without a window"`
	UnreadCount          int32           `json:"unread_count"`
	WaitingSince         *time.Time      `json:"waiting_since" doc:"The customer's first unanswered message; null when answered"`
	SnoozedUntil         *time.Time      `json:"snoozed_until"`
}

func ToConversationDTO(c model.Conversation) InboxConversationDTO {
	return InboxConversationDTO{
		ID: c.ID, Contact: InboxContactDTO{ID: c.Contact.ID, Name: c.Contact.Name, Phone: c.Contact.Phone},
		Channel: InboxChannelDTO(c.Channel), Status: c.Status, Source: c.Source, TeamID: c.TeamID, Assignee: toPerson(c.Assignee),
		LastMessageAt: c.LastMessageAt, LastMessagePreview: c.LastMessagePreview,
		LastMessageDirection: c.LastMessageDirection, WindowExpiresAt: c.WindowExpiresAt,
		UnreadCount: c.UnreadCount, WaitingSince: c.WaitingSince, SnoozedUntil: c.SnoozedUntil,
	}
}

type InboxCountsDTO struct {
	Open    int64 `json:"open"`
	Pending int64 `json:"pending"`
	Closed  int64 `json:"closed"`
	Mine    int64 `json:"mine" doc:"Open and pending chats assigned to you"`
}

type ListConversationsInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Tab         string    `query:"tab" enum:"open,pending,closed,mine" default:"open"`
	Query       string    `query:"q" maxLength:"100" doc:"Contact name, phone or last message contains this"`
	Sort        string    `query:"sort" enum:"newest,waiting" default:"newest" doc:"newest: last message first; waiting: waiting longest first"`
	Cursor      string    `query:"cursor" doc:"next_cursor from the previous page"`
	Limit       int32     `query:"limit" minimum:"1" maximum:"50" default:"25"`
}

func (in *ListConversationsInput) ToModel() (model.ListFilter, bool) {
	f := model.ListFilter{Tab: in.Tab, Query: in.Query, Sort: in.Sort, Limit: in.Limit}
	if in.Cursor != "" {
		c, ok := DecodeCursor(in.Cursor)
		if !ok {
			return f, false
		}
		f.Cursor = &c
	}
	return f, true
}

type InboxConversationsOutput struct {
	Body struct {
		Items      []InboxConversationDTO `json:"items"`
		NextCursor *string                `json:"next_cursor"`
		Counts     InboxCountsDTO         `json:"counts" doc:"Tab counts in your scope; the search doesn't narrow them"`
	}
}

func ToConversationsOutput(p model.Page) *InboxConversationsOutput {
	out := &InboxConversationsOutput{}
	out.Body.Items = make([]InboxConversationDTO, len(p.Items))
	for i, c := range p.Items {
		out.Body.Items[i] = ToConversationDTO(c)
	}
	if p.Next != nil {
		c := EncodeCursor(*p.Next)
		out.Body.NextCursor = &c
	}
	out.Body.Counts = InboxCountsDTO(p.Counts)
	return out
}

type ConversationPath struct {
	WorkspaceID    uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	ConversationID uuid.UUID `path:"conversationId"`
}

type InboxConversationDetailDTO struct {
	InboxConversationDTO
	ContactOptIn  string               `json:"contact_opt_in" enum:"in,out,pending"`
	TeamName      *string              `json:"team_name"`
	Capabilities  InboxCapabilitiesDTO `json:"capabilities" doc:"What the channel allows: drives the window bar, template button, reactions and attachments"`
	LastInboundAt *time.Time           `json:"last_inbound_at"`
	ClosedAt      *time.Time           `json:"closed_at"`
	CreatedAt     time.Time            `json:"created_at"`
	WindowOpen    bool                 `json:"window_open" doc:"Free-form replies are allowed now (always on a channel without a window)"`
	// TemplateSinceInbound drives "template sent, waiting for reply".
	TemplateSinceInbound bool `json:"template_since_inbound" doc:"A template went out after the customer's last message"`
}

func ToConversationDetailDTO(c model.ConversationDetail, now time.Time) InboxConversationDetailDTO {
	return InboxConversationDetailDTO{
		InboxConversationDTO: ToConversationDTO(c.Conversation),
		ContactOptIn:         c.ContactOptIn, TeamName: c.TeamName, Capabilities: toCapabilities(c.Capabilities),
		LastInboundAt: c.LastInboundAt, ClosedAt: c.ClosedAt, CreatedAt: c.CreatedAt,
		WindowOpen:           c.Capabilities.Window <= 0 || (c.WindowExpiresAt != nil && now.Before(*c.WindowExpiresAt)),
		TemplateSinceInbound: c.TemplateSinceInbound,
	}
}

type InboxConversationOutput struct {
	Body InboxConversationDetailDTO
}

type AssignConversationInput struct {
	WorkspaceID    uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	ConversationID uuid.UUID `path:"conversationId"`
	Body           struct {
		UserID *uuid.UUID `json:"user_id" required:"false" nullable:"true" doc:"The member to assign; null or omitted unassigns"`
	}
}

type SetConversationStatusInput struct {
	WorkspaceID    uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	ConversationID uuid.UUID `path:"conversationId"`
	Body           struct {
		Status      string     `json:"status,omitempty" enum:"open,closed" doc:"Open or close; omit when snoozing"`
		SnoozeUntil *time.Time `json:"snooze_until,omitempty" doc:"Snooze: pending until then, at most 90 days ahead"`
	}
}
