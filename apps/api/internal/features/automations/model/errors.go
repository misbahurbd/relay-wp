package model

import "github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"

// Errors returned by the automations service.
var (
	// ErrInvalidSettings carries per-field details, located like "rules[1].reply".
	ErrInvalidSettings = apperr.Validation("invalid_automation_settings", "Some settings are invalid")
	// ErrNotReady carries what is missing, located the same way.
	ErrNotReady = apperr.Conflict("automation_not_ready", "Finish this automation's settings before turning it on")

	// ErrWindowClosed: a free-form message can't reach this contact any more.
	ErrWindowClosed = apperr.Conflict("automation_window_closed", "The reply window has closed")
	// ErrOptedOut: the contact has opted out.
	ErrOptedOut = apperr.Conflict("automation_opted_out", "This contact has opted out")
	// ErrTemplateUnavailable: the template can't be sent any more.
	ErrTemplateUnavailable = apperr.Conflict("automation_template_unavailable", "This template can't be sent")
	// ErrMissingVariable: a template variable has no value.
	ErrMissingVariable = apperr.Conflict("automation_missing_variable", "A template variable has no value")
	// ErrNoChannel: no connected number to start a chat from.
	ErrNoChannel = apperr.Conflict("automation_no_channel", "No WhatsApp number is connected")
	// ErrInvalidPhone: the phone can't reach anyone.
	ErrInvalidPhone = apperr.Validation("automation_invalid_phone", "This phone number isn't valid")
)
