package auth

import "github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"

// Errors returned by the auth service. Handlers return them as-is; platform/httpx
// maps each kind to its HTTP status.
var (
	// ErrInvalidCredentials covers wrong email, wrong password and social-only
	// accounts alike, so responses never reveal which emails have accounts.
	ErrInvalidCredentials = apperr.Unauthorized("invalid_credentials", "Email or password is incorrect")
	// ErrEmailNotVerified is only returned after the correct password.
	ErrEmailNotVerified = apperr.Forbidden("email_not_verified", "Verify your email address before signing in")
	// ErrAccountDisabled is only returned after the correct password.
	ErrAccountDisabled = apperr.Forbidden("account_disabled", "This account has been disabled")
	// ErrTooManyAttempts blocks an email after repeated failed sign-ins.
	ErrTooManyAttempts = apperr.RateLimited("too_many_attempts", "Too many failed sign-in attempts. Try again in a few minutes.")
	// ErrInvalidLink is an unknown, used or expired email link.
	ErrInvalidLink = apperr.Validation("invalid_link", "This link is invalid or has expired")
	// ErrInvalidRefresh ends the client's session: it must sign in again.
	ErrInvalidRefresh = apperr.Unauthorized("invalid_refresh_token", "Your session has ended. Sign in again.")
	// ErrWeakPassword carries one detail per unmet password rule.
	ErrWeakPassword = apperr.Validation("weak_password", "Password doesn't meet the requirements")
	// ErrBreachedPassword means the password appears in known data breaches.
	ErrBreachedPassword = apperr.Validation("breached_password", "This password has appeared in a data breach. Choose a different one.")
	// ErrWrongPassword is a wrong current password when changing it.
	ErrWrongPassword = apperr.Validation("wrong_password", "Current password is incorrect")
	// ErrNoPassword is a password change on a social-only account.
	ErrNoPassword = apperr.Validation("no_password", "This account doesn't have a password yet")
	// ErrInvalidInput carries per-field details.
	ErrInvalidInput = apperr.Validation("invalid_input", "Some fields are invalid")
	// ErrSessionNotFound is an unknown session, or one owned by someone else.
	ErrSessionNotFound = apperr.NotFound("session_not_found", "Session not found")
)
