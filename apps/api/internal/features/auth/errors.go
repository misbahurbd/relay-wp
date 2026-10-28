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

	// ErrProviderNotAvailable is an unknown provider, or one with no credentials configured.
	ErrProviderNotAvailable = apperr.NotFound("provider_not_available", "This sign-in method isn't available")
	// ErrReauthRequired asks the user to sign in again before a sensitive action.
	ErrReauthRequired = apperr.Forbidden("reauthentication_required", "Sign in again to continue")
	// ErrInvalidOAuthState is a missing, expired, reused or foreign sign-in attempt.
	ErrInvalidOAuthState = apperr.Validation("invalid_oauth_state", "This sign-in attempt has expired. Try again.")
	// ErrOAuthFailed is a provider that refused or couldn't complete the sign-in.
	ErrOAuthFailed = apperr.Validation("oauth_failed", "Sign-in with that provider didn't complete. Try again.")
	// ErrEmailInUse is a provider email that matches an account but isn't
	// provider-verified: the owner must sign in and connect the provider from settings.
	ErrEmailInUse = apperr.Conflict("email_in_use_sign_in_first", "An account already uses this email. Sign in, then connect this provider from settings.")
	// ErrIdentityInUse is a provider account already connected to a different user.
	ErrIdentityInUse = apperr.Conflict("identity_in_use", "That account is already connected to another user")
	// ErrProviderAlreadyLinked is a second account of the same provider.
	ErrProviderAlreadyLinked = apperr.Conflict("provider_already_linked", "You already have an account from this provider connected")
	// ErrLastSignInMethod refuses to remove the only way left to sign in.
	ErrLastSignInMethod = apperr.Conflict("last_sign_in_method", "Add a password or another sign-in method before disconnecting this one")
	// ErrIdentityNotFound is a provider the user hasn't connected.
	ErrIdentityNotFound = apperr.NotFound("identity_not_found", "That sign-in method isn't connected")
	// ErrPasswordAlreadySet is POST /auth/password/set on an account with a password.
	ErrPasswordAlreadySet = apperr.Conflict("password_already_set", "This account already has a password. Change it instead.")
	// ErrInvalidRedirect is a redirect_to that isn't in AUTH_ALLOWED_REDIRECTS.
	ErrInvalidRedirect = apperr.Validation("invalid_redirect", "redirect_to is not an allowed address")
	// ErrInvalidAppCode is a wrong, used or expired mobile one-time code or verifier.
	ErrInvalidAppCode = apperr.Unauthorized("invalid_code", "This sign-in code is invalid or has expired")
)
