package model

import "github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"

// Errors returned by the profiles service.
var (
	// ErrNotFound means the user has no profile (or doesn't exist).
	ErrNotFound = apperr.NotFound("profile_not_found", "Profile not found")
	// ErrInvalidInput carries per-field details.
	ErrInvalidInput = apperr.Validation("invalid_input", "Some fields are invalid")
)
