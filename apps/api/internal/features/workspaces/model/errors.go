package model

import "github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"

// Errors returned by the workspaces service.
var (
	// ErrNotFound is a workspace that doesn't exist or that the user can't see.
	ErrNotFound = apperr.NotFound("workspace_not_found", "Workspace not found")
	// ErrInvalidInput carries per-field details.
	ErrInvalidInput = apperr.Validation("invalid_input", "Some fields are invalid")
	// ErrEmailNotVerified blocks creating a workspace before the email is confirmed.
	ErrEmailNotVerified = apperr.Forbidden("email_not_verified", "Verify your email address before creating a workspace")
	// ErrLimitReached allows one trial workspace per user until it is activated.
	ErrLimitReached = apperr.Conflict("workspace_limit_reached",
		"You already have a workspace on a free trial. Finish setting it up first.")
)
