package model

import "github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"

// Errors returned by the automations service.
var (
	// ErrInvalidSettings carries per-field details, located like "rules[1].reply".
	ErrInvalidSettings = apperr.Validation("invalid_automation_settings", "Some settings are invalid")
	// ErrNotReady carries what is missing, located the same way.
	ErrNotReady = apperr.Conflict("automation_not_ready", "Finish this automation's settings before turning it on")
)
