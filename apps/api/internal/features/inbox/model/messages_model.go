package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Message is one entry of a thread: a message, a private note or a system event.
type Message struct {
	ID         uuid.UUID
	Direction  string
	Kind       string
	Type       string
	Body       string
	Content    Content
	ReplyTo    *ReplyRef
	Status     *string
	ErrorCode  *string
	ErrorTitle *string
	Author     *Person
	Automated  bool
	Mentions   []uuid.UUID
	Reactions  Reactions
	CreatedAt  time.Time
	StatusAt   *time.Time
}

// ReplyRef is the message a message quotes.
type ReplyRef struct {
	ID        uuid.UUID
	Direction string
	Type      string
	Body      string
}

// Reactions are the emoji on a message, one per side; empty means none.
type Reactions struct {
	In  string `json:"in,omitempty"`
	Out string `json:"out,omitempty"`
}

// Content is a message's typed data, stored as JSON in our own shape (AD-4).
type Content struct {
	Media       *Media           `json:"media,omitempty"`
	Location    *Location        `json:"location,omitempty"`
	Contacts    json.RawMessage  `json:"contacts,omitempty"`
	Interactive *Interactive     `json:"interactive,omitempty"`
	Template    *TemplateContent `json:"template,omitempty"`
	Event       *EventContent    `json:"event,omitempty"`
	Referral    *Referral        `json:"referral,omitempty"`
}

// Media is a file. ExternalID is the channel's media ID, set for inbound media
// until it is fetched; the storage key is never sent to clients, who read the
// file through the API.
type Media struct {
	ExternalID string `json:"external_id,omitempty"`
	StorageKey string `json:"storage_key,omitempty"`
	Mime       string `json:"mime,omitempty"`
	Size       int64  `json:"size,omitempty"`
	Filename   string `json:"filename,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
}

// Location is a shared map pin.
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name,omitempty"`
	Address   string  `json:"address,omitempty"`
}

// Interactive is the customer's choice from buttons or a list.
type Interactive struct {
	Kind        string `json:"kind"`
	ID          string `json:"id,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// Referral describes the ad or post a chat started from.
type Referral struct {
	SourceType string `json:"source_type,omitempty"`
	SourceID   string `json:"source_id,omitempty"`
	SourceURL  string `json:"source_url,omitempty"`
	Headline   string `json:"headline,omitempty"`
	Body       string `json:"body,omitempty"`
}

// TemplateContent is the template a message sent, with its parameters.
type TemplateContent struct {
	ID         uuid.UUID      `json:"id"`
	Name       string         `json:"name"`
	Language   string         `json:"language"`
	Category   string         `json:"category"`
	HeaderType string         `json:"header_type,omitempty"`
	Params     TemplateParams `json:"params"`
	// HeaderMediaKey is the uploaded file for a media header; never sent to clients.
	HeaderMediaKey string `json:"header_media_key,omitempty"`
}

// TemplateParams fill a template's variables.
type TemplateParams struct {
	HeaderText string           `json:"header_text,omitempty"`
	Body       []string         `json:"body,omitempty"`
	URLButtons []URLButtonParam `json:"url_buttons,omitempty"`
}

// URLButtonParam fills the {{1}} of a dynamic URL button, by button index.
type URLButtonParam struct {
	Index int    `json:"index"`
	Value string `json:"value"`
}

// System event actions (F7).
const (
	ActionAssigned   = "assigned"
	ActionUnassigned = "unassigned"
	ActionClosed     = "closed"
	ActionReopened   = "reopened"
	ActionSnoozed    = "snoozed"
	ActionUnsnoozed  = "unsnoozed"
	// ActionTemplatePending: a template restarted a closed chat, which now waits in Pending.
	ActionTemplatePending = "template_pending"
	// ActionWindowReopened: the customer replied after the channel's window had closed.
	ActionWindowReopened = "window_reopened"
	// ActionOptedOut: the customer replied STOP.
	ActionOptedOut = "opted_out"
)

// EventContent is a system event in the thread.
type EventContent struct {
	Action   string     `json:"action"`
	ActorID  *uuid.UUID `json:"actor_id,omitempty"`
	TargetID *uuid.UUID `json:"target_id,omitempty"`
	Until    *time.Time `json:"until,omitempty"`
}

// SendInput is an agent's outbound message (section 6, POST …/messages).
type SendInput struct {
	// Type is text, media or template.
	Type       string
	Text       string
	MediaKey   string
	Caption    string
	ReplyToID  *uuid.UUID
	TemplateID *uuid.UUID
	Params     TemplateParams
}

// Send input types.
const (
	SendText     = "text"
	SendMedia    = "media"
	SendTemplate = "template"
)

// Upload is a file stored for sending (POST /media).
type Upload struct {
	Key      string
	Mime     string
	Size     int64
	Filename string
	// Kind is the message type it is sent as: image, video, audio or document.
	Kind string
}

// MediaFile describes a stored file being streamed to a client.
type MediaFile struct {
	Mime     string
	Filename string
	Size     int64
}

// InboundEvent tells automations about a customer's message.
type InboundEvent struct {
	WorkspaceID, ConversationID, ContactID, MessageID uuid.UUID
	Text                                              string
	NewConversation                                   bool
	// Returning means the customer came back after the channel's window closed.
	Returning bool
}

// AutomationConversation is what automations read before they send.
type AutomationConversation struct {
	ContactName string
	OptIn       string
	LastAgentAt *time.Time
}

// AutomatedText is what an automation or a broadcast sends.
type AutomatedText struct {
	WorkspaceID uuid.UUID
	// RunID links the message to its automation run; nil for a broadcast.
	RunID *uuid.UUID
	// ConversationID is nil when the sender only knows the contact.
	ConversationID, ContactID *uuid.UUID
	// ChannelID picks the WhatsApp number to send from when there is no
	// ConversationID; nil uses the contact's latest chat, or the default number.
	ChannelID *uuid.UUID
	Text      string
	// TemplateID sends a template instead of Text.
	TemplateID *uuid.UUID
	Params     []string
	HeaderText string
	// Source is how a chat this send starts is labelled: SourceAutomation when empty.
	Source string
	// Bulk sends on the bulk queue, so a broadcast never delays agents' and automations' messages (ADR 0013).
	Bulk bool
}

// Conversation sources a send without an agent can start.
const (
	SourceAutomation = "automation"
	SourceBroadcast  = "broadcast"
)
