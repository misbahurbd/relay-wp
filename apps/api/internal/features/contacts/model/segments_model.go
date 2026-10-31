package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// Segment limits (ADR 0014).
const (
	MaxSegmentName        = 80
	MaxSegmentDescription = 200
	MaxSegmentGroups      = 5
	MaxSegmentConditions  = 10
	MaxSegmentDays        = 3650
	// SegmentSampleSize is how many matching contacts a preview shows.
	SegmentSampleSize = 6
)

// Segment fields: what a condition looks at.
const (
	SegTag         = "tag"
	SegOptIn       = "opt_in"
	SegLastMessage = "last_message"
	SegAdded       = "added"
	SegSource      = "source"
	SegLanguage    = "language"
	SegCountryCode = "country_code"
	SegAttribute   = "attribute"
)

// Segment operators.
const (
	OpIs       = "is"
	OpIsNot    = "is_not"
	OpWithin   = "within"
	OpOver     = "over"
	OpNever    = "never"
	OpContains = "contains"
	OpStarts   = "starts_with"
)

// SegmentOps are the operators each field takes.
var SegmentOps = map[string][]string{
	SegTag:         {OpIs, OpIsNot},
	SegOptIn:       {OpIs, OpIsNot},
	SegLastMessage: {OpWithin, OpOver, OpNever},
	SegAdded:       {OpWithin, OpOver},
	SegSource:      {OpIs, OpIsNot},
	SegLanguage:    {OpIs, OpIsNot},
	SegCountryCode: {OpStarts},
	SegAttribute:   {OpIs, OpContains},
}

// SegmentRules are groups joined by OR; a group matches all or any of its conditions.
type SegmentRules struct {
	Groups []SegmentGroup `json:"groups"`
}

// SegmentGroup is one set of conditions.
type SegmentGroup struct {
	Match      string             `json:"match"` // all | any
	Conditions []SegmentCondition `json:"conditions"`
}

// SegmentCondition tests one field. Key names the custom field for attribute.
type SegmentCondition struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value string `json:"value"`
	Key   string `json:"key,omitempty"`
}

// Days is the condition's value as a day count, for the date fields.
func (c SegmentCondition) Days() int {
	n, _ := strconv.Atoi(c.Value)
	return n
}

// Check lists what is wrong with the rules; empty when the builder can compile them.
func (r SegmentRules) Check() []apperr.Detail {
	var bad []apperr.Detail
	add := func(field, msg string) { bad = append(bad, apperr.Detail{Field: field, Message: msg}) }
	switch n := len(r.Groups); {
	case n == 0:
		add("rules.groups", "Add at least one condition")
	case n > MaxSegmentGroups:
		add("rules.groups", fmt.Sprintf("Use at most %d groups", MaxSegmentGroups))
	}
	for gi, g := range r.Groups {
		gf := fmt.Sprintf("rules.groups[%d]", gi)
		if g.Match != "all" && g.Match != "any" {
			add(gf+".match", "Match all or any")
		}
		switch n := len(g.Conditions); {
		case n == 0:
			add(gf+".conditions", "Add a condition, or remove the group")
		case n > MaxSegmentConditions:
			add(gf+".conditions", fmt.Sprintf("Use at most %d conditions", MaxSegmentConditions))
		}
		for ci, c := range g.Conditions {
			if msg := checkCondition(c); msg != "" {
				add(fmt.Sprintf("%s.conditions[%d]", gf, ci), msg)
			}
		}
	}
	return bad
}

func checkCondition(c SegmentCondition) string {
	ops, ok := SegmentOps[c.Field]
	if !ok {
		return "Choose a field"
	}
	valid := false
	for _, op := range ops {
		valid = valid || op == c.Op
	}
	if !valid {
		return "Choose a condition"
	}
	v := strings.TrimSpace(c.Value)
	switch c.Field {
	case SegTag:
		if _, err := uuid.Parse(v); err != nil {
			return "Choose a tag"
		}
	case SegOptIn:
		if v != OptInIn && v != OptInOut && v != OptInPending {
			return "Choose opted in, opted out or pending"
		}
	case SegLastMessage, SegAdded:
		if c.Op == OpNever {
			return ""
		}
		if n, err := strconv.Atoi(v); err != nil || n < 1 || n > MaxSegmentDays {
			return fmt.Sprintf("Enter 1 to %d days", MaxSegmentDays)
		}
	case SegSource:
		if v != "manual" && v != "import" && v != "inbox" && v != "api" {
			return "Choose a source"
		}
	case SegCountryCode:
		if len(v) < 2 || len(v) > 5 || v[0] != '+' || strings.Trim(v[1:], "0123456789") != "" {
			return "Enter a code like +44"
		}
	case SegAttribute:
		if strings.TrimSpace(c.Key) == "" {
			return "Choose the custom field"
		}
		if v == "" {
			return "Enter a value to use this condition"
		}
	default: // language
		if v == "" || len(v) > 35 {
			return "Enter a language"
		}
	}
	return ""
}

// Segment is a saved rule set and how many contacts it matches now.
type Segment struct {
	ID          uuid.UUID
	Name        string
	Description string
	Rules       SegmentRules
	Count       SegmentCount
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SegmentCount is who matches: everyone, and how many of them a broadcast reaches.
type SegmentCount struct {
	Matched, OptedIn, OptedOut, Pending int
}

// SegmentInput is what a person saves.
type SegmentInput struct {
	Name, Description string
	Rules             SegmentRules
}

// SegmentMatch is one contact in a preview.
type SegmentMatch struct {
	ID          uuid.UUID
	Name, Phone string
	OptIn       string
}

// SegmentPreview is who a rule set matches right now.
type SegmentPreview struct {
	Count  SegmentCount
	Sample []SegmentMatch
}
