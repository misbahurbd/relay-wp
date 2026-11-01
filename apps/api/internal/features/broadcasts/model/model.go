// Package model holds broadcasts' domain types, constants and errors (ADR 0013).
package model

import (
	"time"

	"github.com/google/uuid"
)

// PlanResource is the monthly billing resource one sent broadcast uses (ADR 0008, AD-4).
const PlanResource = "broadcasts"

// EventCompleted is the integrations webhook event a finished broadcast sends.
const EventCompleted = "broadcast.completed"

// Status is where a broadcast is in its life.
type Status string

// Broadcast statuses.
const (
	StatusDraft     Status = "draft"
	StatusScheduled Status = "scheduled"
	StatusSending   Status = "sending"
	StatusCompleted Status = "completed"
	StatusCancelled Status = "cancelled"
	StatusFailed    Status = "failed"
)

// Editable says whether a broadcast's content can still change.
func (s Status) Editable() bool { return s == StatusDraft || s == StatusScheduled }

// Recipient states: pending until a batch handles it, then queued (a message
// exists), skipped (not sent on purpose) or failed (couldn't be sent).
const (
	RecipientPending = "pending"
	RecipientQueued  = "queued"
	RecipientSkipped = "skipped"
	RecipientFailed  = "failed"
)

// Reasons a recipient was skipped or failed, or a broadcast failed.
const (
	ReasonOptedOut         = "opted_out"
	ReasonCancelled        = "cancelled"
	ReasonTemplateUnusable = "template_unavailable"
	ReasonMissingVariable  = "missing_variable"
	ReasonNoChannel        = "no_channel"
	ReasonStalled          = "stalled"
	ReasonSegmentDeleted   = "segment_deleted"
)

// Send is one broadcast message, as the inbox queues it.
type Send struct {
	WorkspaceID, ContactID, NumberID, TemplateID uuid.UUID
	HeaderText                                   string
	Body                                         []string
}

// Template is what a broadcast needs to know about its template.
type Template struct {
	Name, Category string
	// HeaderVariable is true when the header text has {{1}}; BodyVariables counts {{1}}…{{n}}.
	HeaderVariable bool
	BodyVariables  int
}

// Number is the WhatsApp number a broadcast sends from.
type Number struct {
	Connected bool
	// TierLimit is Meta's cap on new customers a day; 0 when unknown or unlimited.
	TierLimit int
}

// Completed is the broadcast.completed webhook's data.
type Completed struct {
	BroadcastID uuid.UUID `json:"broadcast_id"`
	Name        string    `json:"name"`
	Recipients  int       `json:"recipients"`
	Sent        int       `json:"sent"`
	Delivered   int       `json:"delivered"`
	Read        int       `json:"read"`
	Failed      int       `json:"failed"`
	Skipped     int       `json:"skipped"`
	CompletedAt time.Time `json:"completed_at"`
}

// Audience is who a broadcast goes to (AD-5): everyone, or the contacts in any
// of its sources, counted once. Only opted-in contacts get it.
type Audience struct {
	// All is every contact; the sources below are then ignored.
	All    bool        `json:"all,omitempty"`
	TagIDs []uuid.UUID `json:"tag_ids"`
	// SegmentIDs are contact segments, evaluated when counting and when sending starts (ADR 0014).
	SegmentIDs []uuid.UUID `json:"segment_ids,omitempty"`
	// SkipRecentMarketing leaves out contacts sent a marketing template in the
	// last day, to protect the number's quality rating.
	SkipRecentMarketing bool `json:"skip_recent_marketing,omitempty"`
}

// Empty says the audience has no source: nobody would get it.
func (a Audience) Empty() bool { return !a.All && len(a.TagIDs) == 0 && len(a.SegmentIDs) == 0 }

// RecentMarketingWindow is how far back SkipRecentMarketing looks.
const RecentMarketingWindow = 24 * time.Hour

// Broadcast is one campaign.
type Broadcast struct {
	ID               uuid.UUID
	Name             string
	NumberID         *uuid.UUID
	TemplateID       *uuid.UUID
	Template         *TemplateRef
	Audience         Audience
	Variables        Variables
	Status           Status
	ScheduledAt      *time.Time
	StartedAt        *time.Time
	CompletedAt      *time.Time
	RecipientsTotal  int
	FailureReason    *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Stats            Stats
	FailureBreakdown []FailureCount
}

// TemplateRef names a broadcast's template, as it is now.
type TemplateRef struct {
	Name, Language, Category string
}

// Stats are a broadcast's delivery counts. Sent includes delivered and read;
// delivered includes read.
type Stats struct {
	Pending, Queued, Skipped, Failed, Sent, Delivered, Read, Replied int
}

// FailureCount is how many recipients one reason kept from getting a broadcast.
type FailureCount struct {
	Reason     string
	Recipients int
}

// Input is what a person edits on a draft or scheduled broadcast.
type Input struct {
	Name       string
	NumberID   *uuid.UUID
	TemplateID *uuid.UUID
	Audience   Audience
	Variables  Variables
}

// ListFilter narrows the list.
type ListFilter struct {
	Status   *Status
	Query    *string
	BeforeID *uuid.UUID
	Limit    int
}

// AudienceCount is how many contacts a broadcast would reach.
type AudienceCount struct {
	// Included opted in and will get it; LeftOut match but are pending or opted
	// out; Recent opted in but got marketing in the last day, and are skipped
	// when the audience asks (they are not in Included then).
	Included, LeftOut, Recent int
	// TierLimit is the number's Meta limit on new customers a day, 0 when unknown or unlimited.
	TierLimit int
}

// Recipient is one row of a broadcast's report.
type Recipient struct {
	ContactID     uuid.UUID
	Name, Phone   string
	State         string
	Reason        *string
	QueuedAt      *time.Time
	MessageStatus *string
	StatusAt      *time.Time
	Error         *string
	Replied       bool
}

// RecipientFilter values.
const (
	FilterFailed    = "failed"
	FilterDelivered = "delivered"
	FilterRead      = "read"
)
