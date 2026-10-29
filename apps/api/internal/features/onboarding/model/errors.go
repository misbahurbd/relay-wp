package model

import "github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"

// Errors returned by the onboarding service.
var (
	// ErrNotFound is a workspace with no onboarding row.
	ErrNotFound = apperr.NotFound("onboarding_not_found", "Setup progress not found")
	// ErrStepInvalid is an action that doesn't fit the current step.
	ErrStepInvalid = apperr.Conflict("onboarding_step_invalid", "You can only skip while connecting WhatsApp.")
	// ErrNotConnected needs a connected WhatsApp number first.
	ErrNotConnected = apperr.Conflict("whatsapp_not_connected", "Connect a WhatsApp number first.")
)
