// Package model holds the inbox's domain types, constants and errors, shared by
// every layer of the feature. The inbox owns conversations, messages, notes,
// quick replies and inbox actions (docs/PHASE-7-INBOX.md, slice A).
package model

import (
	"time"

	"github.com/google/uuid"
)

// Conversation statuses (AD-2).
const (
	StatusOpen    = "open"
	StatusPending = "pending"
	StatusClosed  = "closed"
)

// Message directions, kinds and outbound statuses (AD-4, AD-7).
const (
	DirectionIn  = "in"
	DirectionOut = "out"

	KindMessage = "message"
	KindNote    = "note"
	KindEvent   = "event"

	MessagePending   = "pending"
	MessageSent      = "sent"
	MessageDelivered = "delivered"
	MessageRead      = "read"
	MessageFailed    = "failed"
)

// Message types the inbox sends or treats specially; inbound types are the channel's as sent.
const (
	TypeText     = "text"
	TypeImage    = "image"
	TypeVideo    = "video"
	TypeAudio    = "audio"
	TypeDocument = "document"
	TypeSticker  = "sticker"
	TypeTemplate = "template"
	TypeReaction = "reaction"
	TypeEvent    = "event"
)

// Where a conversation came from.
const (
	SourceOrganic  = "organic"
	SourceCTWA     = "ctwa_ad"
	SourceTemplate = "template"
)

// Live-update kinds (AD-8). Mention events are addressed to one member.
const (
	SignalMessage      = "message"
	SignalStatus       = "status"
	SignalConversation = "conversation"
	SignalNote         = "note"
	SignalMention      = "mention"
)

// Integrations event types the inbox publishes (F12).
const (
	EventMessageReceived = "message.received"
	EventMessageStatus   = "message.status"
)

// Limits.
const (
	MaxPreview = 120
	// MaxTextLen is the text limit for a channel that doesn't set its own.
	MaxTextLen       = 4096
	MaxCaptionLen    = 1024
	MaxNoteLen       = 4096
	MaxMentions      = 20
	MaxEmojiLen      = 32
	ThreadPageSize   = 50
	DefaultPageLimit = 25
	MaxPageLimit     = 50
	MaxSnooze        = 90 * 24 * time.Hour
)

// Person is a workspace member as shown in the inbox.
type Person struct {
	ID   uuid.UUID
	Name string
}
