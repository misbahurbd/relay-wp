package model

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

const (
	maxMessage      = 1024
	maxKeywordRules = 50
	maxDripSteps    = 10
	maxDiscountCode = 50
)

// Order and cart template variables read from these sources.
const (
	SourceCustomerFirstName = "customer_first_name"
	SourceOrderNumber       = "order_number"
	SourceOrderTotal        = "order_total"
	SourceTrackingURL       = "tracking_url"
	SourceCartTotal         = "cart_total"
	SourceCheckoutURL       = "checkout_url"
	SourceDiscountCode      = "discount_code"
	SourceCustom            = "custom"
)

// Keyword rule match modes.
const (
	MatchAny   = "any"
	MatchExact = "exact"
)

// Order confirmation triggers.
const (
	TriggerShopifyOrderCreated = "shopify_order_created"
	TriggerShopifyOrderPaid    = "shopify_order_paid"
	TriggerWooOrderCreated     = "woocommerce_order_created"
)

// Variable fills one {{n}} of a template, in order. Custom is the text when
// Source is SourceCustom.
type Variable struct {
	Source string `json:"source"`
	Custom string `json:"custom"`
}

func invalid(field, msg string) apperr.Detail { return apperr.Detail{Field: field, Message: msg} }

func checkMessage(field, text string, out []apperr.Detail) []apperr.Detail {
	switch n := utf8.RuneCountInString(text); {
	case strings.TrimSpace(text) == "":
		return append(out, invalid(field, "Write a message"))
	case n > maxMessage:
		return append(out, invalid(field, fmt.Sprintf("Keep it to %d characters", maxMessage)))
	}
	return out
}

func checkVariables(vars []Variable, allowed []string, out []apperr.Detail) []apperr.Detail {
	for i, v := range vars {
		field := fmt.Sprintf("variables[%d]", i)
		if !slices.Contains(allowed, v.Source) {
			out = append(out, invalid(field, "Choose a value for this variable"))
		} else if v.Source == SourceCustom && strings.TrimSpace(v.Custom) == "" {
			out = append(out, invalid(field, "Type the custom text"))
		}
	}
	return out
}

// ---------- welcome ----------

// WelcomeSettings greet a contact on their first message.
type WelcomeSettings struct {
	Message       string `json:"message"`
	DelayMinutes  int    `json:"delay_minutes"`
	FirstTimeOnly bool   `json:"first_time_only"`
}

func defaultWelcome() *WelcomeSettings {
	return &WelcomeSettings{
		Message:       "Hi {{name}}, thanks for messaging us! How can we help today?",
		FirstTimeOnly: true,
	}
}

// Validate implements Settings.
func (s *WelcomeSettings) Validate() []apperr.Detail {
	out := checkMessage("message", s.Message, nil)
	if !slices.Contains([]int{0, 1, 5}, s.DelayMinutes) {
		out = append(out, invalid("delay_minutes", "Choose when to send"))
	}
	return out
}

// Ready implements Settings.
func (s *WelcomeSettings) Ready() []apperr.Detail { return nil }

// ---------- away ----------

// Days lists the week in the order the drawer shows it.
var Days = []string{"sat", "sun", "mon", "tue", "wed", "thu", "fri"}

// WorkingDay is one day's hours, as HH:MM in the automation's timezone.
type WorkingDay struct {
	Day  string `json:"day"`
	Open bool   `json:"open"`
	From string `json:"from"`
	To   string `json:"to"`
}

// AwaySettings reply outside working hours. A workspace has no timezone of its
// own, so this automation stores one.
type AwaySettings struct {
	Timezone string       `json:"timezone"`
	Hours    []WorkingDay `json:"hours"`
	Message  string       `json:"message"`
}

func defaultAway() *AwaySettings {
	hours := make([]WorkingDay, len(Days))
	for i, d := range Days {
		hours[i] = WorkingDay{Day: d, Open: d != "fri", From: "10:00", To: "20:00"}
	}
	return &AwaySettings{
		Timezone: "UTC",
		Hours:    hours,
		Message:  "Thanks for your message! We're closed right now and will reply as soon as we're back.",
	}
}

// Validate implements Settings.
func (s *AwaySettings) Validate() []apperr.Detail {
	out := checkMessage("message", s.Message, nil)
	if _, err := time.LoadLocation(s.Timezone); s.Timezone == "" || err != nil {
		out = append(out, invalid("timezone", "Choose a timezone"))
	}
	if len(s.Hours) != len(Days) {
		return append(out, invalid("hours", "Set hours for every day"))
	}
	for i, d := range s.Hours {
		field := fmt.Sprintf("hours[%d]", i)
		if d.Day != Days[i] {
			out = append(out, invalid(field, "Set hours for every day"))
			continue
		}
		if !d.Open {
			continue
		}
		from, errFrom := time.Parse("15:04", d.From)
		to, errTo := time.Parse("15:04", d.To)
		if errFrom != nil || errTo != nil || !to.After(from) {
			out = append(out, invalid(field, "Closing time must be after opening time"))
		}
	}
	return out
}

// Ready implements Settings.
func (s *AwaySettings) Ready() []apperr.Detail { return nil }

// ---------- keyword ----------

// KeywordRule answers messages that contain Keywords, a comma-separated list.
type KeywordRule struct {
	Keywords string `json:"keywords"`
	Match    string `json:"match"`
	Reply    string `json:"reply"`
}

// KeywordSettings are the rules, first match wins.
type KeywordSettings struct {
	Rules []KeywordRule `json:"rules"`
}

func defaultKeyword() *KeywordSettings { return &KeywordSettings{Rules: []KeywordRule{}} }

// Validate implements Settings.
func (s *KeywordSettings) Validate() []apperr.Detail {
	var out []apperr.Detail
	if len(s.Rules) > maxKeywordRules {
		return append(out, invalid("rules", fmt.Sprintf("Keep it to %d rules", maxKeywordRules)))
	}
	for i, r := range s.Rules {
		if strings.TrimSpace(r.Keywords) == "" {
			out = append(out, invalid(fmt.Sprintf("rules[%d].keywords", i), "Add at least one keyword"))
		}
		if r.Match != MatchAny && r.Match != MatchExact {
			out = append(out, invalid(fmt.Sprintf("rules[%d].match", i), "Choose how to match"))
		}
		out = checkMessage(fmt.Sprintf("rules[%d].reply", i), r.Reply, out)
	}
	return out
}

// Ready implements Settings.
func (s *KeywordSettings) Ready() []apperr.Detail {
	if len(s.Rules) == 0 {
		return []apperr.Detail{invalid("rules", "Add at least one keyword rule")}
	}
	return nil
}

// ---------- abandoned cart ----------

// CartSettings remind shoppers who left checkout.
type CartSettings struct {
	DelayMinutes   int        `json:"delay_minutes"`
	TemplateID     *uuid.UUID `json:"template_id,omitempty"`
	Variables      []Variable `json:"variables"`
	DiscountCode   string     `json:"discount_code"`
	SecondReminder bool       `json:"second_reminder"`
}

var cartSources = []string{
	SourceCustomerFirstName, SourceCartTotal, SourceCheckoutURL, SourceDiscountCode, SourceCustom,
}

func defaultCart() *CartSettings {
	return &CartSettings{DelayMinutes: 60, Variables: []Variable{}, SecondReminder: true}
}

// Validate implements Settings.
func (s *CartSettings) Validate() []apperr.Detail {
	var out []apperr.Detail
	if !slices.Contains([]int{30, 60, 240, 1440}, s.DelayMinutes) {
		out = append(out, invalid("delay_minutes", "Choose when to send the reminder"))
	}
	if utf8.RuneCountInString(s.DiscountCode) > maxDiscountCode {
		out = append(out, invalid("discount_code", fmt.Sprintf("Keep it to %d characters", maxDiscountCode)))
	}
	return checkVariables(s.Variables, cartSources, out)
}

// Ready implements Settings.
func (s *CartSettings) Ready() []apperr.Detail {
	if s.TemplateID == nil {
		return []apperr.Detail{invalid("template_id", "Choose a template")}
	}
	return nil
}

// ---------- order confirmation ----------

// OrderSettings send a template when a store order arrives.
type OrderSettings struct {
	Trigger    string     `json:"trigger"`
	TemplateID *uuid.UUID `json:"template_id,omitempty"`
	Variables  []Variable `json:"variables"`
}

var orderSources = []string{
	SourceCustomerFirstName, SourceOrderNumber, SourceOrderTotal, SourceTrackingURL, SourceCustom,
}

func defaultOrder() *OrderSettings {
	return &OrderSettings{
		Trigger: TriggerShopifyOrderCreated,
		Variables: []Variable{
			{Source: SourceCustomerFirstName},
			{Source: SourceOrderNumber},
			{Source: SourceOrderTotal},
			{Source: SourceTrackingURL},
		},
	}
}

// Validate implements Settings.
func (s *OrderSettings) Validate() []apperr.Detail {
	var out []apperr.Detail
	if !slices.Contains([]string{TriggerShopifyOrderCreated, TriggerShopifyOrderPaid, TriggerWooOrderCreated}, s.Trigger) {
		out = append(out, invalid("trigger", "Choose a trigger"))
	}
	return checkVariables(s.Variables, orderSources, out)
}

// Ready implements Settings.
func (s *OrderSettings) Ready() []apperr.Detail {
	if s.TemplateID == nil {
		return []apperr.Detail{invalid("template_id", "Choose a template")}
	}
	return nil
}

// ---------- drip ----------

// DripStep sends a template Day days after the contact joins the sequence.
type DripStep struct {
	Day        int        `json:"day"`
	TemplateID *uuid.UUID `json:"template_id,omitempty"`
}

// DripSettings run a sequence for contacts given a tag.
type DripSettings struct {
	TagID       *uuid.UUID `json:"tag_id,omitempty"`
	Steps       []DripStep `json:"steps"`
	StopOnReply bool       `json:"stop_on_reply"`
}

func defaultDrip() *DripSettings {
	return &DripSettings{
		Steps:       []DripStep{{Day: 0}, {Day: 2}, {Day: 5}, {Day: 10}},
		StopOnReply: true,
	}
}

// Validate implements Settings.
func (s *DripSettings) Validate() []apperr.Detail {
	var out []apperr.Detail
	if len(s.Steps) == 0 || len(s.Steps) > maxDripSteps {
		return append(out, invalid("steps", fmt.Sprintf("Use 1 to %d steps", maxDripSteps)))
	}
	prev := -1
	for i, st := range s.Steps {
		if st.Day <= prev {
			out = append(out, invalid(fmt.Sprintf("steps[%d].day", i), "Each step must come after the one before"))
		}
		prev = st.Day
	}
	return out
}

// Ready implements Settings.
func (s *DripSettings) Ready() []apperr.Detail {
	var out []apperr.Detail
	if s.TagID == nil {
		out = append(out, invalid("tag_id", "Choose who gets this sequence"))
	}
	for i, st := range s.Steps {
		if st.TemplateID == nil {
			out = append(out, invalid(fmt.Sprintf("steps[%d].template_id", i), "Choose a template"))
		}
	}
	return out
}
