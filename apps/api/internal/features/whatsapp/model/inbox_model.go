package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// The inbox contract (docs/PHASE-7-INBOX.md AD-1): whatsapp parses Meta's
// webhooks into these types and hands them to an InboundSink, and sends what the
// inbox asks for. It never stores chats.

// InboundMessage is one message a customer sent to a connected number.
type InboundMessage struct {
	WorkspaceID uuid.UUID
	// NumberID is whatsapp_numbers.id; PhoneNumberID is Meta's ID for it.
	NumberID      uuid.UUID
	PhoneNumberID string
	WAMID         string
	// From is the customer's number in E.164 (with +); ProfileName is their WhatsApp name.
	From        string
	ProfileName string
	Timestamp   time.Time
	// Type is Meta's message type: text, image, video, audio, document, sticker,
	// location, contacts, interactive, button, reaction, order or unsupported.
	Type string
	// Text is the text body, a media caption, or the title of a button or list reply.
	Text        string
	Media       *InboundMedia
	Location    *Location
	Interactive *InteractiveReply
	Reaction    *Reaction
	// Contacts is Meta's contacts array, as sent.
	Contacts json.RawMessage
	// ContextWAMID is the message this one replies to, if any.
	ContextWAMID string
	// Referral is set when the chat started from a Click-to-WhatsApp ad.
	Referral *Referral
	// Raw is Meta's message object, for types the inbox doesn't model.
	Raw json.RawMessage
}

// InboundMedia identifies a media file on Meta; FetchMedia downloads it.
type InboundMedia struct {
	ID       string
	MimeType string
	SHA256   string
	Filename string
}

// Location is a shared map pin.
type Location struct {
	Latitude  float64
	Longitude float64
	Name      string
	Address   string
}

// InteractiveReply is the customer's choice from buttons or a list.
type InteractiveReply struct {
	// Kind is button_reply, list_reply or button (a template quick reply).
	Kind        string
	ID          string
	Title       string
	Description string
}

// Reaction is an emoji on an earlier message; an empty Emoji removes it.
type Reaction struct {
	WAMID string
	Emoji string
}

// Referral describes the ad or post a chat started from.
type Referral struct {
	SourceType string
	SourceID   string
	SourceURL  string
	Headline   string
	Body       string
}

// StatusUpdate is Meta's report on a message we sent.
type StatusUpdate struct {
	WorkspaceID uuid.UUID
	NumberID    uuid.UUID
	WAMID       string
	// Status is sent, delivered, read or failed.
	Status      string
	Timestamp   time.Time
	RecipientID string
	// ErrorCode and ErrorTitle are set when Status is failed.
	ErrorCode  int
	ErrorTitle string
}

// OutboundMedia is a file already uploaded to Meta (UploadMedia), sent by ID.
type OutboundMedia struct {
	// Kind is image, video, audio or document.
	Kind     string
	MediaID  string
	Caption  string
	Filename string
}

// OutboundTemplate is an approved template with its parameters.
type OutboundTemplate struct {
	Name     string
	Language string
	// HeaderText fills a text header's {{1}}; HeaderMediaID (with HeaderKind
	// image, video or document) fills a media header.
	HeaderText    string
	HeaderKind    string
	HeaderMediaID string
	// BodyParams fill the body's {{1}}..{{N}} in order.
	BodyParams []string
	// URLButtonParams fill each dynamic URL button's suffix, by button index.
	URLButtonParams map[int]string
}

// SendError is Meta refusing a message. Permanent errors must not be retried
// (for example 131047, the 24-hour window is closed).
type SendError struct {
	Code      int
	Title     string
	Message   string
	Permanent bool
}

func (e *SendError) Error() string { return "whatsapp send failed: " + e.Title }
