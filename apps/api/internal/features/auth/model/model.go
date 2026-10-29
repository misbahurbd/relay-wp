// Package model holds auth's domain types, constants and errors, shared by every layer of the
// feature.
package model

import (
	"time"
)

// Clients: web gets tokens as httpOnly cookies, mobile in the response body.
const (
	ClientWeb    = "web"
	ClientMobile = "mobile"
)

// Sign-in providers recorded on sessions, identities and events.
const (
	ProviderPassword = "password"
	ProviderGoogle   = "google"
	ProviderFacebook = "facebook"
)

// OAuth flow settings.
const (
	IntentLogin = "login"
	IntentLink  = "link"

	FlowStatusAwaitingEmail = "awaiting_email"
	FlowStatusReady         = "ready"

	OauthFlowTTL      = 10 * time.Minute // provider round trip
	OauthReadyTTL     = 2 * time.Minute  // mobile app redeeming its one-time code
	OauthAwaitingTTL  = 30 * time.Minute // user typing an email address
	OauthCodeMaxSends = 3                // codes one "add your email" step may send
)

// RefreshReuseGrace lets a just-rotated refresh token be presented again for a
// moment: browser tabs share one cookie and may refresh at the same time. Reuse
// after the window means the token was copied.
const RefreshReuseGrace = 30 * time.Second

// Email token purposes and lifetimes.
const (
	PurposeVerify = "verify"
	PurposeReset  = "reset"

	VerifyTokenTTL = 24 * time.Hour
	ResetTokenTTL  = time.Hour

	// Sign-up codes: short-lived, and dead after a few wrong tries.
	VerifyCodeTTL         = 10 * time.Minute
	VerifyCodeMaxAttempts = 5
	// verifyCodesPerHour caps codes per account (sign-up plus resends), so
	// at most verifyCodesPerHour*verifyCodeMaxAttempts guesses an hour.
	VerifyCodesPerHour = 4
)

// Session revoke reasons (match the CHECK constraint in 00002_auth.sql).
const (
	RevokeLogout          = "logout"
	RevokeLogoutAll       = "logout_all"
	RevokePasswordChanged = "password_changed"
	RevokePasswordReset   = "password_reset"
	RevokeReuseDetected   = "reuse_detected"
	RevokeByUser          = "revoked_by_user"
	RevokeAccountClaimed  = "account_claimed"
)

// Security events written to auth_events.
const (
	EventSignup                 = "signup"
	EventLoginSucceeded         = "login_succeeded"
	EventLoginFailed            = "login_failed"
	EventLogout                 = "logout"
	EventLogoutAll              = "logout_all"
	EventEmailVerified          = "email_verified"
	EventVerificationSent       = "verification_sent"
	EventPasswordChanged        = "password_changed"
	EventPasswordResetRequested = "password_reset_requested"
	EventPasswordResetCompleted = "password_reset_completed"
	EventSessionRevoked         = "session_revoked"
	EventRefreshReuseDetected   = "refresh_reuse_detected"
	EventPasswordSet            = "password_set"
	EventIdentityLinked         = "identity_linked"
	EventIdentityUnlinked       = "identity_unlinked"
)

// RequestMeta describes who is calling, for sessions and the event log.
type RequestMeta struct {
	IP           string
	UserAgent    string
	Client       string // web or mobile
	DeviceHeader string // X-Device-Name from the mobile app
}

// Tokens is the result of a successful sign-in or refresh.
type Tokens struct {
	Client           string
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}
