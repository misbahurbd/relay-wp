package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
)

type SegmentConditionDTO struct {
	Field string `json:"field" enum:"tag,opt_in,last_message,added,source,language,country_code,attribute"`
	Op    string `json:"op" enum:"is,is_not,within,over,never,contains,starts_with"`
	Value string `json:"value" maxLength:"200" doc:"A tag ID, an opt-in status, a number of days, a source, a language, a code like +44, or a custom field's value"`
	Key   string `json:"key,omitempty" maxLength:"100" doc:"The custom field, for attribute"`
}

type SegmentGroupDTO struct {
	Match      string                `json:"match" enum:"all,any"`
	Conditions []SegmentConditionDTO `json:"conditions" maxItems:"10"`
}

type SegmentRulesDTO struct {
	Groups []SegmentGroupDTO `json:"groups" maxItems:"5" doc:"Joined by OR"`
}

func (r SegmentRulesDTO) Rules() model.SegmentRules {
	out := model.SegmentRules{Groups: make([]model.SegmentGroup, len(r.Groups))}
	for i, g := range r.Groups {
		out.Groups[i] = model.SegmentGroup{Match: g.Match, Conditions: make([]model.SegmentCondition, len(g.Conditions))}
		for j, c := range g.Conditions {
			out.Groups[i].Conditions[j] = model.SegmentCondition(c)
		}
	}
	return out
}

func toRulesDTO(r model.SegmentRules) SegmentRulesDTO {
	out := SegmentRulesDTO{Groups: make([]SegmentGroupDTO, len(r.Groups))}
	for i, g := range r.Groups {
		out.Groups[i] = SegmentGroupDTO{Match: g.Match, Conditions: make([]SegmentConditionDTO, len(g.Conditions))}
		for j, c := range g.Conditions {
			out.Groups[i].Conditions[j] = SegmentConditionDTO(c)
		}
	}
	return out
}

type SegmentCountDTO struct {
	Matched  int `json:"matched"`
	OptedIn  int `json:"opted_in" doc:"Who a broadcast to this segment reaches"`
	OptedOut int `json:"opted_out"`
	Pending  int `json:"pending"`
}

type SegmentDTO struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Rules       SegmentRulesDTO `json:"rules"`
	Count       SegmentCountDTO `json:"count" doc:"Who matches right now"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func ToSegmentDTO(s model.Segment) SegmentDTO {
	return SegmentDTO{
		ID: s.ID, Name: s.Name, Description: s.Description, Rules: toRulesDTO(s.Rules),
		Count: SegmentCountDTO(s.Count), CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

type SegmentPath struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	SegmentID   uuid.UUID `path:"segmentId"`
}

type SegmentBody struct {
	Name        string          `json:"name" maxLength:"200"`
	Description string          `json:"description,omitempty" maxLength:"400"`
	Rules       SegmentRulesDTO `json:"rules"`
}

func (b SegmentBody) Input() model.SegmentInput {
	return model.SegmentInput{Name: b.Name, Description: b.Description, Rules: b.Rules.Rules()}
}

type CreateSegmentInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Body        SegmentBody
}

type UpdateSegmentInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	SegmentID   uuid.UUID `path:"segmentId"`
	Body        SegmentBody
}

type PreviewSegmentInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Body        struct {
		Rules SegmentRulesDTO `json:"rules"`
	}
}

type SegmentOutput struct {
	Body SegmentDTO
}

type SegmentsOutput struct {
	Body struct {
		Items []SegmentDTO `json:"items"`
	}
}

type SegmentMatchDTO struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Phone string    `json:"phone"`
	OptIn string    `json:"opt_in" enum:"in,out,pending"`
}

type SegmentPreviewOutput struct {
	Body struct {
		Count  SegmentCountDTO   `json:"count"`
		Sample []SegmentMatchDTO `json:"sample" doc:"Up to 6 matching contacts, most recently active first"`
	}
}

func ToSegmentPreviewOutput(p model.SegmentPreview) *SegmentPreviewOutput {
	out := &SegmentPreviewOutput{}
	out.Body.Count = SegmentCountDTO(p.Count)
	out.Body.Sample = make([]SegmentMatchDTO, len(p.Sample))
	for i, m := range p.Sample {
		out.Body.Sample[i] = SegmentMatchDTO(m)
	}
	return out
}
