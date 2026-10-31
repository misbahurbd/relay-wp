// Package dto holds broadcasts' HTTP request and response shapes and the converters from domain types.
package dto

import (
	"encoding/base64"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/broadcasts/model"
)

type WorkspaceInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
}

type BroadcastPath struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	BroadcastID uuid.UUID `path:"broadcastId"`
}

// ---------- shapes ----------

type VariableDTO struct {
	Source   string `json:"source" enum:"text,name,phone,attribute" doc:"Fixed text, or a field of the contact it is sent to"`
	Value    string `json:"value" maxLength:"1024" doc:"The text, or the attribute's key; empty for name and phone"`
	Fallback string `json:"fallback,omitempty" maxLength:"1024" doc:"Used when the contact's field is empty"`
}

type VariablesDTO struct {
	Header *VariableDTO  `json:"header,omitempty" doc:"Fills the header's {{1}}"`
	Body   []VariableDTO `json:"body,omitempty" maxItems:"30" doc:"body[0] fills {{1}}, body[1] fills {{2}}, …"`
}

type AudienceDTO struct {
	All                 bool        `json:"all,omitempty" doc:"Every opted-in contact; the sources below are then ignored"`
	TagIDs              []uuid.UUID `json:"tag_ids,omitempty" maxItems:"20" doc:"Opted-in contacts with any of these tags"`
	SegmentIDs          []uuid.UUID `json:"segment_ids,omitempty" maxItems:"10" doc:"Opted-in contacts in any of these segments"`
	SkipRecentMarketing bool        `json:"skip_recent_marketing,omitempty" doc:"Leave out contacts sent a marketing template in the last 24 hours"`
}

func (a AudienceDTO) Audience() model.Audience {
	return model.Audience{All: a.All, TagIDs: a.TagIDs, SegmentIDs: a.SegmentIDs, SkipRecentMarketing: a.SkipRecentMarketing}
}

func ToAudienceDTO(a model.Audience) AudienceDTO {
	out := AudienceDTO{All: a.All, TagIDs: a.TagIDs, SegmentIDs: a.SegmentIDs, SkipRecentMarketing: a.SkipRecentMarketing}
	if out.TagIDs == nil {
		out.TagIDs = []uuid.UUID{}
	}
	if out.SegmentIDs == nil {
		out.SegmentIDs = []uuid.UUID{}
	}
	return out
}

type TemplateRefDTO struct {
	Name     string `json:"name"`
	Language string `json:"language,omitempty"`
	Category string `json:"category,omitempty" enum:"marketing,utility,authentication"`
}

type StatsDTO struct {
	Pending   int `json:"pending" doc:"Recipients not handled yet"`
	Queued    int `json:"queued" doc:"Messages waiting to go to Meta"`
	Sent      int `json:"sent" doc:"Messages Meta accepted, including delivered and read"`
	Delivered int `json:"delivered" doc:"Delivered, including read"`
	Read      int `json:"read"`
	Replied   int `json:"replied" doc:"Recipients who wrote back after it"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped" doc:"Not sent on purpose: opted out, or cancelled"`
}

type FailureDTO struct {
	Reason     string `json:"reason" doc:"Meta's error, or opted_out, cancelled, template_unavailable, missing_variable, no_channel, stalled, segment_deleted"`
	Recipients int    `json:"recipients"`
}

type BroadcastDTO struct {
	ID              uuid.UUID       `json:"id"`
	Name            string          `json:"name"`
	NumberID        *uuid.UUID      `json:"number_id" nullable:"true"`
	TemplateID      *uuid.UUID      `json:"template_id" nullable:"true"`
	Template        *TemplateRefDTO `json:"template,omitempty" doc:"Absent when none is chosen, or it was deleted"`
	Audience        AudienceDTO     `json:"audience"`
	Variables       VariablesDTO    `json:"variables"`
	Status          string          `json:"status" enum:"draft,scheduled,sending,completed,cancelled,failed"`
	ScheduledAt     *time.Time      `json:"scheduled_at" nullable:"true"`
	StartedAt       *time.Time      `json:"started_at" nullable:"true"`
	CompletedAt     *time.Time      `json:"completed_at" nullable:"true"`
	RecipientsTotal int             `json:"recipients_total" doc:"Set when sending starts"`
	FailureReason   *string         `json:"failure_reason" nullable:"true"`
	Stats           StatsDTO        `json:"stats"`
	Failures        []FailureDTO    `json:"failures" doc:"Why recipients missed it, most common first"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type BroadcastSummaryDTO struct {
	ID              uuid.UUID       `json:"id"`
	Name            string          `json:"name"`
	Template        *TemplateRefDTO `json:"template,omitempty" doc:"Absent when none is chosen, or it was deleted"`
	Status          string          `json:"status" enum:"draft,scheduled,sending,completed,cancelled,failed"`
	ScheduledAt     *time.Time      `json:"scheduled_at" nullable:"true"`
	StartedAt       *time.Time      `json:"started_at" nullable:"true"`
	CompletedAt     *time.Time      `json:"completed_at" nullable:"true"`
	RecipientsTotal int             `json:"recipients_total"`
	Sent            int             `json:"sent"`
	Delivered       int             `json:"delivered"`
	Read            int             `json:"read"`
	Failed          int             `json:"failed"`
	CreatedAt       time.Time       `json:"created_at"`
}

type RecipientDTO struct {
	ContactID     uuid.UUID  `json:"contact_id"`
	Name          string     `json:"name"`
	Phone         string     `json:"phone"`
	State         string     `json:"state" enum:"pending,queued,skipped,failed"`
	Reason        *string    `json:"reason" nullable:"true"`
	QueuedAt      *time.Time `json:"queued_at" nullable:"true"`
	MessageStatus *string    `json:"message_status" nullable:"true" enum:"pending,sent,delivered,read,failed"`
	StatusAt      *time.Time `json:"status_at" nullable:"true"`
	Error         *string    `json:"error" nullable:"true" doc:"Meta's reason a send failed"`
	Replied       bool       `json:"replied"`
}

// ---------- requests ----------

type ListInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Status      string    `query:"status" enum:"draft,scheduled,sending,completed,cancelled,failed"`
	Q           string    `query:"q" maxLength:"100" doc:"Part of the name, case-insensitive"`
	Limit       int       `query:"limit" minimum:"1" maximum:"100" default:"25"`
	Cursor      string    `query:"cursor" maxLength:"100"`
}

func (in ListInput) Filter() (model.ListFilter, bool) {
	f := model.ListFilter{Limit: in.Limit}
	if in.Status != "" {
		st := model.Status(in.Status)
		f.Status = &st
	}
	if in.Q != "" {
		f.Query = &in.Q
	}
	if in.Cursor != "" {
		id, ok := DecodeCursor(in.Cursor)
		if !ok {
			return f, false
		}
		f.BeforeID = &id
	}
	return f, true
}

type BroadcastBody struct {
	Name       string       `json:"name" maxLength:"200"`
	NumberID   *uuid.UUID   `json:"number_id,omitempty" doc:"The WhatsApp number to send from"`
	TemplateID *uuid.UUID   `json:"template_id,omitempty" doc:"An approved template"`
	Audience   AudienceDTO  `json:"audience,omitempty"`
	Variables  VariablesDTO `json:"variables,omitempty"`
}

func (b BroadcastBody) Input() model.Input {
	in := model.Input{
		Name: b.Name, NumberID: b.NumberID, TemplateID: b.TemplateID,
		Audience:  b.Audience.Audience(),
		Variables: model.Variables{Body: make([]model.Variable, len(b.Variables.Body))},
	}
	if h := b.Variables.Header; h != nil {
		in.Variables.Header = &model.Variable{Source: h.Source, Value: h.Value, Fallback: h.Fallback}
	}
	for i, v := range b.Variables.Body {
		in.Variables.Body[i] = model.Variable(v)
	}
	return in
}

type CreateInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Body        BroadcastBody
}

type UpdateInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	BroadcastID uuid.UUID `path:"broadcastId"`
	Body        BroadcastBody
}

type SendInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	BroadcastID uuid.UUID `path:"broadcastId"`
	Body        struct {
		ScheduledAt *time.Time `json:"scheduled_at,omitempty" doc:"When to send; absent sends now. Up to 90 days ahead"`
	}
}

type AudienceInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Body        struct {
		AudienceDTO
		NumberID *uuid.UUID `json:"number_id,omitempty" doc:"Adds the number's daily Meta tier to the answer"`
	}
}

type RecipientsInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	BroadcastID uuid.UUID `path:"broadcastId"`
	Filter      string    `query:"filter" enum:"failed,delivered,read"`
	Limit       int       `query:"limit" minimum:"1" maximum:"100" default:"50"`
	Cursor      string    `query:"cursor" maxLength:"100"`
}

// ---------- responses ----------

type BroadcastOutput struct {
	Body BroadcastDTO
}

type ListOutput struct {
	Body struct {
		Items      []BroadcastSummaryDTO `json:"items"`
		NextCursor *string               `json:"next_cursor" nullable:"true"`
	}
}

type AudienceOutput struct {
	Body struct {
		Included  int `json:"included" doc:"Opted-in contacts who will get it"`
		LeftOut   int `json:"left_out" doc:"Contacts that match but are pending or opted out"`
		Recent    int `json:"recent" doc:"Opted-in contacts sent marketing in the last 24 hours; not in included when the audience skips them"`
		TierLimit int `json:"tier_limit" doc:"The number's Meta limit on new customers a day; 0 when unknown or unlimited"`
	}
}

type RecipientsOutput struct {
	Body struct {
		Items      []RecipientDTO `json:"items"`
		NextCursor *string        `json:"next_cursor" nullable:"true"`
	}
}

// ---------- converters ----------

func ToBroadcastDTO(b model.Broadcast) BroadcastDTO {
	out := BroadcastDTO{
		ID: b.ID, Name: b.Name, NumberID: b.NumberID, TemplateID: b.TemplateID, Template: toTemplateRef(b.Template),
		Audience: ToAudienceDTO(b.Audience), Variables: toVariables(b.Variables), Status: string(b.Status),
		ScheduledAt: b.ScheduledAt, StartedAt: b.StartedAt, CompletedAt: b.CompletedAt, RecipientsTotal: b.RecipientsTotal,
		FailureReason: b.FailureReason, Stats: toStats(b.Stats), Failures: make([]FailureDTO, len(b.FailureBreakdown)),
		CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
	}
	for i, f := range b.FailureBreakdown {
		out.Failures[i] = FailureDTO(f)
	}
	return out
}

func ToSummaryDTO(b model.Broadcast) BroadcastSummaryDTO {
	return BroadcastSummaryDTO{
		ID: b.ID, Name: b.Name, Template: toTemplateRef(b.Template), Status: string(b.Status),
		ScheduledAt: b.ScheduledAt, StartedAt: b.StartedAt, CompletedAt: b.CompletedAt, RecipientsTotal: b.RecipientsTotal,
		Sent: b.Stats.Sent, Delivered: b.Stats.Delivered, Read: b.Stats.Read, Failed: b.Stats.Failed, CreatedAt: b.CreatedAt,
	}
}

func ToRecipientDTO(r model.Recipient) RecipientDTO {
	return RecipientDTO{
		ContactID: r.ContactID, Name: r.Name, Phone: r.Phone, State: r.State, Reason: r.Reason, QueuedAt: r.QueuedAt,
		MessageStatus: r.MessageStatus, StatusAt: r.StatusAt, Error: r.Error, Replied: r.Replied,
	}
}

func toStats(s model.Stats) StatsDTO {
	return StatsDTO{
		Pending: s.Pending, Queued: s.Queued, Sent: s.Sent, Delivered: s.Delivered, Read: s.Read,
		Replied: s.Replied, Failed: s.Failed, Skipped: s.Skipped,
	}
}

func toTemplateRef(t *model.TemplateRef) *TemplateRefDTO {
	if t == nil {
		return nil
	}
	return &TemplateRefDTO{Name: t.Name, Language: t.Language, Category: t.Category}
}

func toVariables(v model.Variables) VariablesDTO {
	out := VariablesDTO{Body: make([]VariableDTO, len(v.Body))}
	if v.Header != nil {
		h := VariableDTO(*v.Header)
		out.Header = &h
	}
	for i, b := range v.Body {
		out.Body[i] = VariableDTO(b)
	}
	return out
}

// EncodeCursor makes the opaque page cursor for the last item of a page.
func EncodeCursor(id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString(id[:])
}

func DecodeCursor(s string) (uuid.UUID, bool) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return uuid.Nil, false
	}
	id, err := uuid.FromBytes(b)
	return id, err == nil
}
