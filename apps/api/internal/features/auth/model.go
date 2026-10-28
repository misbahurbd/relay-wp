// Package auth owns sign-in: users, password and email-link flows, sessions,
// refresh tokens and the security event log. Display data lives in features/profiles.
package auth

import (
	"time"

	"github.com/google/uuid"
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

	oauthFlowTTL     = 10 * time.Minute // provider round trip
	oauthReadyTTL    = 2 * time.Minute  // mobile app redeeming its one-time code
	oauthAwaitingTTL = 30 * time.Minute // user typing an email address
)

// Email token purposes and lifetimes.
const (
	purposeVerify = "verify"
	purposeReset  = "reset"

	verifyTokenTTL = 24 * time.Hour
	resetTokenTTL  = time.Hour
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

// SignupInput is a new password account.
type SignupInput struct {
	Email     string
	Password  string
	FirstName string
	LastName  *string
	Timezone  string
}

// Session is one signed-in device, as shown to its owner.
type Session struct {
	ID         uuid.UUID
	Provider   string
	Client     string
	DeviceName string
	DeviceType string
	LastIP     string
	LastUsedAt time.Time
	CreatedAt  time.Time
	Current    bool
}

// Event is one entry of a user's security activity.
type Event struct {
	ID        uuid.UUID
	Event     string
	Provider  string
	IP        string
	Device    string
	CreatedAt time.Time
}

// OAuthStart begins a social sign-in or a link from settings.
type OAuthStart struct {
	Provider   string
	Intent     string // login or link
	LinkUserID uuid.UUID
	SessionID  uuid.UUID // link only: the session must have signed in recently
	Client     string
	RedirectTo string
	Timezone   string
	// AppCodeChallenge is the mobile app's PKCE S256 challenge (required for mobile).
	AppCodeChallenge string
}

// OAuthResult is where a callback sends the browser next. Exactly one outcome is set.
type OAuthResult struct {
	RedirectTo string
	Tokens     *Tokens // web sign-in: set cookies
	AppCode    string  // mobile sign-in: one-time code for POST /auth/oauth/token
	// CompleteToken means the provider gave no email: ask for one (POST /auth/oauth/complete).
	CompleteToken string
	Linked        string // provider connected from settings
	ErrorCode     string // sign-in failed; shown by the page at RedirectTo
}

// Identity is one connected social account, as shown to its owner.
type Identity struct {
	Provider   string
	Email      string
	LastUsedAt time.Time
	CreatedAt  time.Time
}
