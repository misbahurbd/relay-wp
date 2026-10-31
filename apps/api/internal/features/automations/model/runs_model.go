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
)

// OptOutSource is the contact's opt_in_source after replying STOP.
const OptOutSource = "whatsapp_reply"

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
	WorkspaceID    uuid.UUID
	ConversationID uuid.UUID
	RunID          uuid.UUID
	Text           string
}

// RunContext is stored on a run.
type RunContext struct {
	Text string `json:"text"`
}
