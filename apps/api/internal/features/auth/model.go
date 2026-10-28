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

// Sign-in providers recorded on sessions and events.
const providerPassword = "password"

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
