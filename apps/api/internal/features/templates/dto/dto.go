// Package dto holds templates' HTTP request and response shapes and the converters from domain types.
package dto

import (
	"encoding/base64"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/model"
)

// ---------- components ----------

type HeaderDTO struct {
	Type    string `json:"type" enum:"none,text,image,video,document"`
	Text    string `json:"text" doc:"Text header, at most 60 characters and one {{1}} variable"`
	Example string `json:"example" doc:"Example value for the text header's {{1}}"`
}

type BodyDTO struct {
	Text     string   `json:"text" doc:"Message text with {{1}}..{{N}} variables, at most 1024 characters"`
	Examples []string `json:"examples" doc:"One example per variable, in order"`
}

type FooterDTO struct {
	Text string `json:"text"`
}

type ButtonDTO struct {
	Type  string `json:"type" enum:"quick_reply,url,phone"`
	Text  string `json:"text"`
	Value string `json:"value" doc:"The https:// link or the E.164 phone number; empty for a quick reply"`
}

type ComponentsDTO struct {
	Header  HeaderDTO   `json:"header"`
	Body    BodyDTO     `json:"body"`
	Footer  FooterDTO   `json:"footer"`
	Buttons []ButtonDTO `json:"buttons"`
}

func toComponentsDTO(c model.Components) ComponentsDTO {
	out := ComponentsDTO{
		Header: HeaderDTO(c.Header), Body: BodyDTO(c.Body), Footer: FooterDTO(c.Footer),
		Buttons: make([]ButtonDTO, len(c.Buttons)),
	}
	if out.Body.Examples == nil {
		out.Body.Examples = []string{}
	}
	for i, b := range c.Buttons {
		out.Buttons[i] = ButtonDTO(b)
	}
	return out
}

// Request shapes are lenient (every field optional, no enums) so the service's
// AD-7 rules decide, and errors come back as invalid_template with field locations.

type HeaderInput struct {
	Type    string `json:"type,omitempty" maxLength:"20" doc:"none, text, image, video or document"`
	Text    string `json:"text,omitempty" maxLength:"1000"`
	Example string `json:"example,omitempty" maxLength:"1000"`
}

type BodyInput struct {
	Text     string   `json:"text,omitempty" maxLength:"4096"`
	Examples []string `json:"examples,omitempty" maxItems:"100"`
}

type FooterInput struct {
	Text string `json:"text,omitempty" maxLength:"1000"`
}

type ButtonInput struct {
	Type  string `json:"type,omitempty" maxLength:"20" doc:"quick_reply, url or phone"`
	Text  string `json:"text,omitempty" maxLength:"200"`
	Value string `json:"value,omitempty" maxLength:"2000"`
}

type ComponentsInput struct {
	Header  HeaderInput   `json:"header,omitempty"`
	Body    BodyInput     `json:"body,omitempty"`
	Footer  FooterInput   `json:"footer,omitempty"`
	Buttons []ButtonInput `json:"buttons,omitempty" maxItems:"20"`
}

func (c ComponentsInput) model() model.Components {
	out := model.Components{
		Header: model.Header(c.Header), Body: model.Body(c.Body), Footer: model.Footer(c.Footer),
		Buttons: make([]model.Button, len(c.Buttons)),
	}
	for i, b := range c.Buttons {
		out.Buttons[i] = model.Button(b)
	}
	return out
}

// ---------- template ----------

type SampleDTO struct {
	ContentType string `json:"content_type" enum:"image/jpeg,image/png,video/mp4,application/pdf"`
	Size        int64  `json:"size" doc:"Bytes"`
}

type WarningDTO struct {
	Code    string `json:"code" enum:"short_link"`
	Field   string `json:"field" doc:"Where the problem is, e.g. buttons[0].value"`
	Message string `json:"message"`
}

type TemplateDTO struct {
	ID             uuid.UUID     `json:"id"`
	Name           string        `json:"name"`
	Language       string        `json:"language" enum:"en_US,en_GB,es,pt_BR,hi,bn,ar" doc:"Meta's language code; synced templates may use others"`
	Category       string        `json:"category" enum:"marketing,utility,authentication"`
	Status         string        `json:"status" enum:"draft,pending,approved,rejected,paused,disabled"`
	MetaTemplateID *string       `json:"meta_template_id" nullable:"true"`
	RejectedReason *string       `json:"rejected_reason" nullable:"true"`
	Components     ComponentsDTO `json:"components"`
	HeaderSample   *SampleDTO    `json:"header_sample,omitempty" doc:"The uploaded media header sample; absent until one is uploaded"`
	SubmittedAt    *time.Time    `json:"submitted_at" nullable:"true"`
	CreatedBy      *uuid.UUID    `json:"created_by" nullable:"true" doc:"Null for templates synced from Meta"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	Warnings       []WarningDTO  `json:"warnings" doc:"Problems Meta may reject the template for; they don't block saving"`
}

func ToTemplateDTO(t model.Template) TemplateDTO {
	d := TemplateDTO{
		ID: t.ID, Name: t.Name, Language: t.Language, Category: t.Category, Status: t.Status,
		MetaTemplateID: t.MetaTemplateID, RejectedReason: t.RejectedReason, Components: toComponentsDTO(t.Components),
		SubmittedAt: t.SubmittedAt, CreatedBy: t.CreatedBy, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
		Warnings: make([]WarningDTO, len(t.Warnings)),
	}
	if t.HeaderSample != nil {
		d.HeaderSample = &SampleDTO{ContentType: t.HeaderSample.ContentType, Size: t.HeaderSample.Size}
	}
	for i, w := range t.Warnings {
		d.Warnings[i] = WarningDTO(w)
	}
	return d
}

type TemplateOutput struct {
	Body TemplateDTO
}

// ---------- inputs ----------

type TemplatePath struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	TemplateID  uuid.UUID `path:"templateId"`
}

type WorkspaceInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
}

type ListInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	Status      string    `query:"status" enum:"draft,pending,approved,rejected,paused,disabled"`
	Category    string    `query:"category" enum:"marketing,utility,authentication"`
	Q           string    `query:"q" maxLength:"100" doc:"Part of the name, case-insensitive"`
	Limit       int32     `query:"limit" minimum:"1" maximum:"100" default:"25"`
	Cursor      string    `query:"cursor" maxLength:"100"`
}

func (in ListInput) Filter() (model.ListFilter, bool) {
	f := model.ListFilter{Status: optional(in.Status), Category: optional(in.Category), Q: optional(in.Q), Limit: in.Limit}
	if in.Cursor != "" {
		id, ok := DecodeCursor(in.Cursor)
		if !ok {
			return f, false
		}
		f.BeforeID = &id
	}
	return f, true
}

type CountsDTO struct {
	All      int64 `json:"all"`
	Approved int64 `json:"approved"`
	Pending  int64 `json:"pending"`
	Rejected int64 `json:"rejected"`
}

type ListOutput struct {
	Body struct {
		Items      []TemplateDTO `json:"items"`
		NextCursor *string       `json:"next_cursor" nullable:"true"`
		Counts     CountsDTO     `json:"counts" doc:"Status tab counts for the same category and search"`
	}
}

type CreateInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	Body        struct {
		Name       string          `json:"name" maxLength:"1024" doc:"Lowercase letters, numbers and underscores"`
		Language   string          `json:"language" maxLength:"15" doc:"en_US, en_GB, es, pt_BR, hi, bn or ar"`
		Category   string          `json:"category" maxLength:"20" doc:"marketing or utility"`
		Components ComponentsInput `json:"components,omitempty"`
	}
}

func (in CreateInput) Draft() model.Draft {
	b := in.Body
	return model.Draft{Name: b.Name, Language: b.Language, Category: b.Category, Components: b.Components.model()}
}

type UpdateInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	TemplateID  uuid.UUID `path:"templateId"`
	Body        struct {
		Name       *string          `json:"name,omitempty" maxLength:"1024"`
		Language   *string          `json:"language,omitempty" maxLength:"15"`
		Category   *string          `json:"category,omitempty" maxLength:"20"`
		Components *ComponentsInput `json:"components,omitempty" doc:"Replaces all components"`
	}
}

func (in UpdateInput) Patch() model.Patch {
	b := in.Body
	p := model.Patch{Name: b.Name, Language: b.Language, Category: b.Category}
	if b.Components != nil {
		c := b.Components.model()
		p.Components = &c
	}
	return p
}

type SampleForm struct {
	File huma.FormFile `form:"file" contentType:"image/jpeg,image/png,video/mp4,application/pdf" required:"true" doc:"JPG or PNG up to 5 MB, MP4 up to 16 MB, or PDF up to 100 MB; must match the header type"`
}

type SampleInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	TemplateID  uuid.UUID `path:"templateId"`
	RawBody     huma.MultipartFormFiles[SampleForm]
}

type SyncOutput struct {
	Body struct {
		Synced int `json:"synced" doc:"Templates pulled from Meta and saved"`
	}
}

// ---------- helpers ----------

// EncodeCursor makes the opaque page cursor for the last template of a page.
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

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
