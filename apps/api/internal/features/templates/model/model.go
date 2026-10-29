// Package model holds templates' domain types, constants and errors, shared by every layer of the
// feature (docs/PHASE-4-CONTACTS-TEMPLATES.md AD-6 to AD-9).
package model

import (
	"time"

	"github.com/google/uuid"
)

// Template statuses. Only draft is ours; the rest mirror Meta's review.
const (
	StatusDraft    = "draft"
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
	StatusPaused   = "paused"
	StatusDisabled = "disabled"
)

// Statuses lists every template status.
var Statuses = []string{StatusDraft, StatusPending, StatusApproved, StatusRejected, StatusPaused, StatusDisabled}

// Template categories. Authentication templates are synced and listed but can't be
// created or submitted yet (AD-11).
const (
	CategoryMarketing      = "marketing"
	CategoryUtility        = "utility"
	CategoryAuthentication = "authentication"
)

// Categories lists every template category.
var Categories = []string{CategoryMarketing, CategoryUtility, CategoryAuthentication}

// Languages are the Meta language codes a template can be written in; the web shares the list.
var Languages = []string{"en_US", "en_GB", "es", "pt_BR", "hi", "bn", "ar"}

// Header types.
const (
	HeaderNone     = "none"
	HeaderText     = "text"
	HeaderImage    = "image"
	HeaderVideo    = "video"
	HeaderDocument = "document"
)

// HeaderTypes lists every header type.
var HeaderTypes = []string{HeaderNone, HeaderText, HeaderImage, HeaderVideo, HeaderDocument}

// Button types.
const (
	ButtonQuickReply = "quick_reply"
	ButtonURL        = "url"
	ButtonPhone      = "phone"
)

// ButtonTypes lists every button type.
var ButtonTypes = []string{ButtonQuickReply, ButtonURL, ButtonPhone}

// Components is a template's content in our own shape, stored as JSON. It is
// converted to Meta's format only at the Meta boundary.
type Components struct {
	Header  Header   `json:"header"`
	Body    Body     `json:"body"`
	Footer  Footer   `json:"footer"`
	Buttons []Button `json:"buttons"`
}

// Header is the optional first line: text, or a media sample.
type Header struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	Example string `json:"example"`
}

// Body is the message text with {{n}} variables and one example per variable.
type Body struct {
	Text     string   `json:"text"`
	Examples []string `json:"examples"`
}

// Footer is optional small text under the body.
type Footer struct {
	Text string `json:"text"`
}

// Button is a quick reply, a link or a phone call.
type Button struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Value string `json:"value"`
}

// Sample is the uploaded example file for a media header.
type Sample struct {
	Key         string // storage key; never sent to clients
	ContentType string
	Size        int64
}

// Template is a message template, a draft or one Meta knows.
type Template struct {
	ID             uuid.UUID
	Name           string
	Language       string
	Category       string
	Status         string
	MetaTemplateID *string
	RejectedReason *string
	Components     Components
	HeaderSample   *Sample
	SubmittedAt    *time.Time
	CreatedBy      *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	// Warnings are problems Meta may reject the template for, but that don't block saving.
	Warnings []Warning
}

// Warning is a non-blocking problem tied to a field, such as a short link.
type Warning struct {
	Code    string
	Field   string
	Message string
}

// Draft is what a create sets.
type Draft struct {
	Name       string
	Language   string
	Category   string
	Components Components
}

// Patch is a partial edit; nil fields stay as they are.
type Patch struct {
	Name       *string
	Language   *string
	Category   *string
	Components *Components
}

// ListFilter narrows and pages the template list.
type ListFilter struct {
	Status   *string
	Category *string
	Q        *string
	Limit    int32
	BeforeID *uuid.UUID
}

// Counts are the status tab counts of the template list.
type Counts struct {
	All      int64
	Approved int64
	Pending  int64
	Rejected int64
}

// RemoteTemplate is a template as exchanged with Meta through MetaTemplates.
type RemoteTemplate struct {
	// MetaID is Meta's template ID; empty for a template not yet created on Meta.
	MetaID         string
	Name           string
	Language       string
	Category       string
	Status         string
	RejectedReason string
	Components     Components
	// HeaderHandle is the Resumable Upload handle of the media header sample, on submit.
	HeaderHandle string
}
