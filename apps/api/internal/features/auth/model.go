// Package auth owns sign-in: users, password and email-link flows, sessions,
// refresh tokens and the security event log. Display data lives in features/profiles.
package auth

import (
	"time"
)

// Clients: web gets tokens as httpOnly cookies, mobile in the response body.
const (
	clientWeb    = "web"
	clientMobile = "mobile"
)

// Sign-in providers recorded on sessions, identities and events.
const (
	providerPassword = "password"
	providerGoogle   = "google"
	providerFacebook = "facebook"
)

// OAuth flow settings.
const (
	intentLogin = "login"
	intentLink  = "link"

	flowStatusAwaitingEmail = "awaiting_email"
	flowStatusReady         = "ready"

	oauthFlowTTL      = 10 * time.Minute // provider round trip
	oauthReadyTTL     = 2 * time.Minute  // mobile app redeeming its one-time code
	oauthAwaitingTTL  = 30 * time.Minute // user typing an email address
	oauthCodeMaxSends = 3                // codes one "add your email" step may send
)

// refreshReuseGrace lets a just-rotated refresh token be presented again for a
// moment: browser tabs share one cookie and may refresh at the same time. Reuse
// after the window means the token was copied.
const refreshReuseGrace = 30 * time.Second

// Email token purposes and lifetimes.
const (
	purposeVerify = "verify"
	purposeReset  = "reset"

	verifyTokenTTL = 24 * time.Hour
	resetTokenTTL  = time.Hour

	// Sign-up codes: short-lived, and dead after a few wrong tries.
	verifyCodeTTL         = 10 * time.Minute
	verifyCodeMaxAttempts = 5
	// verifyCodesPerHour caps codes per account (sign-up plus resends), so
	// at most verifyCodesPerHour*verifyCodeMaxAttempts guesses an hour.
	verifyCodesPerHour = 4
)

// Session revoke reasons (match the CHECK constraint in 00002_auth.sql).
const (
	revokeLogout          = "logout"
	revokeLogoutAll       = "logout_all"
	revokePasswordChanged = "password_changed"
	revokePasswordReset   = "password_reset"
	revokeReuseDetected   = "reuse_detected"
	revokeByUser          = "revoked_by_user"
	revokeAccountClaimed  = "account_claimed"
)

// Security events written to auth_events.
const (
	eventSignup                 = "signup"
	eventLoginSucceeded         = "login_succeeded"
	eventLoginFailed            = "login_failed"
	eventLogout                 = "logout"
	eventLogoutAll              = "logout_all"
	eventEmailVerified          = "email_verified"
	eventVerificationSent       = "verification_sent"
	eventPasswordChanged        = "password_changed"
	eventPasswordResetRequested = "password_reset_requested"
	eventPasswordResetCompleted = "password_reset_completed"
	eventSessionRevoked         = "session_revoked"
	eventRefreshReuseDetected   = "refresh_reuse_detected"
	eventPasswordSet            = "password_set"
	eventIdentityLinked         = "identity_linked"
	eventIdentityUnlinked       = "identity_unlinked"
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
