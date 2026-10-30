package model

import (
	"encoding/json"
	"fmt"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// ResourceKind is how a plan limits a resource (ADR 0008).
type ResourceKind string

// Resource kinds.
const (
	// KindCount caps how many of something a workspace has at once, counted live by the feature that owns it.
	KindCount ResourceKind = "count"
	// KindMonthly caps how many a workspace uses per calendar month (UTC).
	KindMonthly ResourceKind = "monthly"
	// KindAccess is a feature a plan either includes or not.
	KindAccess ResourceKind = "access"
)

// Resource is something a plan can include.
type Resource struct {
	Key  string
	Kind ResourceKind
	// Unit names one of it ("team seat"); Label names the resource ("Team seats").
	Unit  string
	Label string
	// Required resources are in every plan: a workspace can't work without them.
	Required bool
	// Enforced is true once a feature checks the limit. Until then a plan can
	// include it for the pricing table, but nothing stops a workspace going over.
	Enforced bool
}

// Resources are what plans can include, in display order: limits first, then features.
var Resources = []Resource{
	{Key: "members", Kind: KindCount, Unit: "team seat", Label: "Team seats", Required: true, Enforced: true},
	{Key: "whatsapp_numbers", Kind: KindCount, Unit: "WhatsApp number", Label: "WhatsApp numbers", Required: true, Enforced: true},
	{Key: "contacts", Kind: KindCount, Unit: "contact", Label: "Contacts", Enforced: true},
	{Key: "chatbot_flows", Kind: KindCount, Unit: "chatbot flow", Label: "Chatbot flows"},
	{Key: "broadcasts", Kind: KindMonthly, Unit: "broadcast", Label: "Broadcasts"},
	{Key: "api_calls", Kind: KindMonthly, Unit: "API call", Label: "API calls"},
	{Key: "automations", Kind: KindAccess, Label: "Automations & drip campaigns"},
	{Key: "ai_replies", Kind: KindAccess, Label: "AI replies"},
	{Key: "ecommerce", Kind: KindAccess, Label: "Shopify & WooCommerce"},
	{Key: "team_controls", Kind: KindAccess, Label: "Roles, audit log & SSO"},
	{Key: "priority_support", Kind: KindAccess, Label: "Priority support"},
}

// ResourceByKey finds a resource in the catalog.
func ResourceByKey(key string) (Resource, bool) {
	for _, r := range Resources {
		if r.Key == key {
			return r, true
		}
	}
	return Resource{}, false
}

// Limit values.
const (
	// Unlimited is a count or monthly limit with no cap.
	Unlimited = -1
	// NotIncluded is what a workspace gets of a resource its plan leaves out.
	NotIncluded = 0
	// Included marks an access resource as part of the plan.
	Included = 1
)

// Limits maps a resource key to what a plan includes: a cap for count and
// monthly resources (Unlimited for none), Included for access ones. A key
// that is absent is not included.
type Limits map[string]int

// Of is the cap on resource key: NotIncluded when absent.
func (l Limits) Of(key string) int {
	if v, ok := l[key]; ok {
		return v
	}
	return NotIncluded
}

// With is l with overrides on top, key by key. An override of NotIncluded takes a resource away.
func (l Limits) With(overrides Limits) Limits {
	out := make(Limits, len(l)+len(overrides))
	for k, v := range l {
		out[k] = v
	}
	for k, v := range overrides {
		out[k] = v
	}
	return out
}

// Complete has every catalog resource, NotIncluded where absent, so clients never apply the missing-key rule.
func (l Limits) Complete() Limits {
	out := make(Limits, len(Resources))
	for _, r := range Resources {
		out[r.Key] = l.Of(r.Key)
	}
	return out
}

// Check lists what is wrong with a plan's limits as a person entered them.
func (l Limits) Check(field string) []apperr.Detail {
	details := l.checkValues(field)
	for _, r := range Resources {
		if r.Required && l.Of(r.Key) == NotIncluded {
			details = append(details, apperr.Detail{Field: field + "." + r.Key, Message: "Every plan includes " + r.Label})
		}
	}
	return details
}

// CheckOverrides lists what is wrong with one workspace's overrides. They may
// take a resource away, but not a required one.
func (l Limits) CheckOverrides(field string) []apperr.Detail {
	details := l.checkValues(field)
	for k, v := range l {
		if r, ok := ResourceByKey(k); ok && r.Required && v == NotIncluded {
			details = append(details, apperr.Detail{Field: field + "." + k, Message: "Every workspace keeps " + r.Label})
		}
	}
	return details
}

func (l Limits) checkValues(field string) []apperr.Detail {
	var details []apperr.Detail
	bad := func(k, msg string) { details = append(details, apperr.Detail{Field: field + "." + k, Message: msg}) }
	for k, v := range l {
		r, ok := ResourceByKey(k)
		switch {
		case !ok:
			bad(k, "Not a resource plans can include")
		case r.Kind == KindAccess && v != NotIncluded && v != Included:
			bad(k, "A feature is included (1) or not (0)")
		case r.Kind != KindAccess && v < Unlimited:
			bad(k, "A limit is -1 (unlimited) or more")
		}
	}
	return details
}

// MarshalJSON writes no limits as {}: the columns hold JSON objects, never null.
func (l Limits) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("{}"), nil
	}
	b, err := json.Marshal(map[string]int(l))
	if err != nil {
		return nil, fmt.Errorf("encode limits: %w", err)
	}
	return b, nil
}

// ParseLimits decodes plans.limits or plan_grants.overrides.
func ParseLimits(raw []byte) (Limits, error) {
	l := Limits{}
	if len(raw) == 0 {
		return l, nil
	}
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, fmt.Errorf("parse plan limits: %w", err)
	}
	return l, nil
}
