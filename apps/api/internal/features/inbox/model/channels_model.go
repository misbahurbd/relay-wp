package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// The inbox's side of every channel boundary (AD-1, ADR 0010): internal/app
// converts a channel feature's parsed webhooks into these types, and these sends
// into its calls. Nothing here is specific to one channel.

// Channel types. A conversation's channel_type picks the registered Channel.
const (
	ChannelWhatsApp  = "whatsapp"
	ChannelMessenger = "messenger"
	ChannelInstagram = "instagram"
)

// Capabilities are what a channel allows (ADR 0010). Every rule that differs by
// channel reads these, never the channel's name.
type Capabilities struct {
	// Window is how long after the customer's last message free-form messages are
	// allowed; 0 means always.
	Window       time.Duration
	Templates    bool
	Reactions    bool
	ReadReceipts bool
	// Media are the file kinds an agent may send, with their limits.
	Media      []MediaKind
	MaxTextLen int
}

// MediaKind is one kind of file a channel takes.
type MediaKind struct {
	// Kind is the message type it is sent as: image, video, audio or document.
	Kind     string
	MaxBytes int64
	// MimeTypes are the accepted types; empty accepts any type the inbox can store.
	MimeTypes []string
}

// ChannelAccount is how the inbox shows a channel's account: for WhatsApp, the
// number's verified name and its display phone.
type ChannelAccount struct {
	Name   string
	Handle string
}

// Inbound is one message a customer sent on a channel.
type Inbound struct {
	WorkspaceID uuid.UUID
	ChannelType string
	// ChannelID is the provider's account row (whatsapp_numbers.id for WhatsApp).
	ChannelID uuid.UUID
	// ExternalID is the channel's ID for the message (WhatsApp's wamid).
	ExternalID string
	Sender     Sender
	Timestamp  time.Time
	// Type is the channel's message type, as sent.
	Type        string
	Text        string
	Media       *InboundMedia
	Location    *Location
	Interactive *Interactive
	Reaction    *InboundReaction
	Contacts    json.RawMessage
	// ContextExternalID is the message this one replies to, if any.
	ContextExternalID string
	Referral          *Referral
}

// Sender is who wrote an inbound message.
type Sender struct {
	// ExternalID is the sender's ID on the channel (the E.164 phone on WhatsApp).
	ExternalID string
	// Phone is the sender's E.164 number when the channel has one.
	Phone string
	// Name is the sender's profile name on the channel.
	Name string
}

// InboundMedia identifies a customer's file on the channel.
type InboundMedia struct {
	ID       string
	Mime     string
	SHA256   string
	Filename string
}

// InboundReaction is the customer's emoji on an earlier message; empty removes it.
type InboundReaction struct {
	ExternalID string
	Emoji      string
}

// StatusUpdate is the channel's report on a message we sent: sent, delivered, read or failed.
type StatusUpdate struct {
	WorkspaceID uuid.UUID
	ChannelType string
	ChannelID   uuid.UUID
	ExternalID  string
	Status      string
	Timestamp   time.Time
	ErrorCode   string
	ErrorTitle  string
}

// Contact is the customer as the inbox needs it.
type Contact struct {
	ID    uuid.UUID
	Name  string
	Phone string
	OptIn string
}

// OutboundMedia is a file uploaded to the channel, sent by its media ID.
type OutboundMedia struct {
	Kind     string
	MediaID  string
	Caption  string
	Filename string
}

// OutboundTemplate is an approved template with its parameters.
type OutboundTemplate struct {
	Name            string
	Language        string
	HeaderText      string
	HeaderKind      string
	HeaderMediaID   string
	BodyParams      []string
	URLButtonParams map[int]string
}

// SendFailure is a channel refusing a send. A permanent one is not retried (AD-5).
type SendFailure struct {
	Code      string
	Title     string
	Permanent bool
}

func (e *SendFailure) Error() string { return "send failed: " + e.Code + " " + e.Title }

// CategoryMarketing is the template category that needs the contact's opt-in.
const CategoryMarketing = "marketing"

// Template is an approved template from the catalog.
type Template struct {
	ID         uuid.UUID
	Name       string
	Language   string
	Category   string
	HeaderType string // none, text, image, video or document
	HeaderText string
	Body       string
	Footer     string
	Buttons    []TemplateButton
}

// TemplateButton is a template's quick reply, URL or phone button.
type TemplateButton struct {
	Type  string
	Text  string
	Value string
}

// TemplateOption is an approved template with the variables a send must fill.
type TemplateOption struct {
	Template
	BodyVariables int
	// HeaderVariable is true when a text header has {{1}}.
	HeaderVariable bool
	// HeaderMedia is true when the header is an image, video or document to upload.
	HeaderMedia bool
	// URLButtons are the indexes of URL buttons whose link ends in {{1}}.
	URLButtons []int
}
