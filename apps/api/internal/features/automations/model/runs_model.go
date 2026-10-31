package model

import (
	"time"

	"github.com/google/uuid"
)

// Run states.
const (
	StateScheduled = "scheduled"
	StateDone      = "done"
	StateSkipped   = "skipped"
	StateCancelled = "cancelled"
	StateFailed    = "failed"
)

// Why a run ended without sending.
const (
	ReasonAgentActive   = "agent_active"
	ReasonLowerPriority = "lower_priority"
	ReasonRateLimited   = "rate_limited"
	ReasonOptedOut      = "opted_out"
	ReasonPaused        = "paused"
	ReasonPlan          = "plan"
	ReasonWindowClosed  = "window_closed"
	ReasonNoPhone       = "no_phone"
	ReasonNoConsent     = "no_consent"
	ReasonReplied       = "replied"
	ReasonRedacted      = "redacted"
	// ReasonTemplateUnavailable: the template was removed or lost its approval.
	ReasonTemplateUnavailable = "template_unavailable"
	// ReasonMissingVariable: the event had no value for a template variable.
	ReasonMissingVariable = "missing_variable"
	// ReasonNoChannel: no connected number to start the chat from.
	ReasonNoChannel = "no_channel"
)

// OptOutSource is the contact's opt_in_source after replying STOP.
const OptOutSource = "whatsapp_reply"

// Contact is what a send needs from a contact.
type Contact struct {
	OptIn string
	Name  string
}

// Inbound is a customer's message.
type Inbound struct {
	WorkspaceID    uuid.UUID `json:"workspace_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	ContactID      uuid.UUID `json:"contact_id"`
	MessageID      uuid.UUID `json:"message_id"`
	Text           string    `json:"text"`
	// NewConversation is the first message of this thread.
	NewConversation bool `json:"new_conversation"`
	// Returning means the customer came back after the reply window closed.
	Returning bool `json:"returning"`
}

// Conversation is what automations read from the inbox before sending.
type Conversation struct {
	ContactName string
	OptIn       string
	LastAgentAt *time.Time
}

// AutomatedSend is one message an automation sends.
type AutomatedSend struct {
	WorkspaceID uuid.UUID
	// ConversationID is nil when the run started from a store event.
	ConversationID *uuid.UUID
	ContactID      *uuid.UUID
	RunID          uuid.UUID
	Text           string
	// TemplateID sends a template instead of Text.
	TemplateID *uuid.UUID
	Params     []string
}

// RunContext is stored on a run.
type RunContext struct {
	Text       string     `json:"text,omitempty"`
	TemplateID *uuid.UUID `json:"template_id,omitempty"`
	Params     []string   `json:"params,omitempty"`
	// Consent means the shopper accepted marketing at checkout.
	Consent bool `json:"consent,omitempty"`
	// Second sends one more reminder.
	Second bool `json:"second,omitempty"`
}

// CheckoutEvent is a shopper's checkout, as it changes.
type CheckoutEvent struct {
	WorkspaceID uuid.UUID
	Token       string
	FirstName   string
	LastName    string
	Phone       string
	Total       string
	CheckoutURL string
	// Consent means the shopper accepted marketing.
	Consent bool
}

// OrderEvent is a store order.
type OrderEvent struct {
	WorkspaceID uuid.UUID
	Provider    string
	Status      string
	OrderID     string
	Number      string
	Total       string
	FirstName   string
	LastName    string
	Phone       string
	StatusURL   string
	// CheckoutToken links the order to the checkout it came from.
	CheckoutToken string
	AmountMinor   int64
	Currency      string
}

// Stats are a kind's counts this month.
type Stats struct {
	Sent, Failed, Read int64
	// Recovered counts carts bought after a reminder, worth RecoveredMinor.
	Recovered, RecoveredMinor int64
	// Active counts contacts still waiting, whenever they started.
	Active            int64
	RecoveredCurrency string
}
