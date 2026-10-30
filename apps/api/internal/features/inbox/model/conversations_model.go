package model

import (
	"time"

	"github.com/google/uuid"
)

// List tabs and sorts (F2).
const (
	TabOpen    = "open"
	TabPending = "pending"
	TabClosed  = "closed"
	TabMine    = "mine"

	SortNewest  = "newest"
	SortWaiting = "waiting"
)

// ContactRef is the conversation's customer.
type ContactRef struct {
	ID    uuid.UUID
	Name  string
	Phone string // E.164
}

// ChannelRef is the channel account a conversation is on (ADR 0010).
type ChannelRef struct {
	Type string
	// ID is the provider's account row: whatsapp_numbers.id for WhatsApp.
	ID uuid.UUID
	// Name and Handle are empty when the account is gone (a chat outlives it).
	Name   string
	Handle string
}

// Conversation is one row of the inbox list.
type Conversation struct {
	ID                   uuid.UUID
	Contact              ContactRef
	Channel              ChannelRef
	Status               string
	Source               string
	TeamID               *uuid.UUID
	Assignee             *Person
	LastMessageAt        time.Time
	LastMessagePreview   string
	LastMessageDirection string
	WindowExpiresAt      *time.Time
	UnreadCount          int32
	WaitingSince         *time.Time
	SnoozedUntil         *time.Time
}

// ConversationDetail is one conversation with what the thread header and right panel need.
type ConversationDetail struct {
	Conversation
	ContactOptIn string
	TeamName     *string
	// Capabilities are the channel's; the zero value when the channel isn't registered.
	Capabilities  Capabilities
	LastInboundAt *time.Time
	ClosedAt      *time.Time
	CreatedAt     time.Time
	// TemplateSinceInbound is true when a template went out after the customer's
	// last message: "template sent, waiting for reply" (F5).
	TemplateSinceInbound bool
}

// Cursor is where a list page continues. At is the sort value of the last row
// (nil in the "not waiting" part of the waiting sort).
type Cursor struct {
	At *time.Time
	ID uuid.UUID
}

// ListFilter selects and pages the conversation list.
type ListFilter struct {
	Tab    string
	Query  string
	Sort   string
	Cursor *Cursor
	Limit  int32
}

// Counts are the tab counts.
type Counts struct {
	Open    int64
	Pending int64
	Closed  int64
	Mine    int64
}

// Page is one page of the conversation list.
type Page struct {
	Items  []Conversation
	Next   *Cursor
	Counts Counts
}
