package auth

import (
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/middleware"
)

// Security lists the schemes an authenticated operation accepts (see app.newAPI).
var Security = []map[string][]string{{"bearer": {}}, {"cookie": {}}}

// Register adds every auth route. requireAuth is middleware.RequireAuth.
func Register(api huma.API, h *Handler, requireAuth func(huma.Context, func(huma.Context))) {
	tags := []string{"auth"}
	public := func(id, method, path, summary string, status int) huma.Operation {
		return huma.Operation{OperationID: id, Method: method, Path: path, Summary: summary, Tags: tags, DefaultStatus: status}
	}
	authed := func(id, method, path, summary string, status int) huma.Operation {
		op := public(id, method, path, summary, status)
		op.Security = Security
		op.Middlewares = huma.Middlewares{requireAuth}
		return op
	}

	huma.Register(api, public("signup", http.MethodPost, "/v1/auth/signup",
		"Create a password account and send a verification email", http.StatusAccepted), h.signup)
	huma.Register(api, public("verify-email", http.MethodPost, "/v1/auth/verify-email",
		"Verify an email address from the emailed link", http.StatusNoContent), h.verifyEmail)
	huma.Register(api, public("resend-verification", http.MethodPost, "/v1/auth/verify-email/resend",
		"Send a new verification email", http.StatusAccepted), h.resendVerification)
	huma.Register(api, public("login", http.MethodPost, "/v1/auth/login",
		"Sign in with email and password", http.StatusOK), h.login)
	huma.Register(api, public("refresh-token", http.MethodPost, "/v1/auth/token/refresh",
		"Rotate the refresh token and get a new access token", http.StatusOK), h.refresh)
	huma.Register(api, public("forgot-password", http.MethodPost, "/v1/auth/password/forgot",
		"Email a password reset link", http.StatusAccepted), h.forgotPassword)
	huma.Register(api, public("reset-password", http.MethodPost, "/v1/auth/password/reset",
		"Set a new password from the emailed link; signs out every device", http.StatusNoContent), h.resetPassword)

	huma.Register(api, authed("logout", http.MethodPost, "/v1/auth/logout",
		"Sign out this device", http.StatusNoContent), h.logout)
	huma.Register(api, authed("logout-all", http.MethodPost, "/v1/auth/logout-all",
		"Sign out every device", http.StatusNoContent), h.logoutAll)
	huma.Register(api, authed("change-password", http.MethodPost, "/v1/auth/password/change",
		"Change password; signs out every other device", http.StatusNoContent), h.changePassword)
	huma.Register(api, authed("list-sessions", http.MethodGet, "/v1/auth/sessions",
		"List signed-in devices", http.StatusOK), h.listSessions)
	huma.Register(api, authed("revoke-session", http.MethodDelete, "/v1/auth/sessions/{id}",
		"Sign out one device", http.StatusNoContent), h.revokeSession)
	huma.Register(api, authed("list-auth-events", http.MethodGet, "/v1/auth/events",
		"Recent security activity", http.StatusOK), h.listEvents)
}

// RateLimits are per-IP limits for auth endpoints, on top of the global limit.
func RateLimits() []middleware.RouteLimit {
	return []middleware.RouteLimit{
		{Method: http.MethodPost, Path: "/v1/auth/login", Requests: 10, Window: time.Minute},
		{Method: http.MethodPost, Path: "/v1/auth/signup", Requests: 5, Window: time.Hour},
		{Method: http.MethodPost, Path: "/v1/auth/verify-email/resend", Requests: 5, Window: time.Hour},
		{Method: http.MethodPost, Path: "/v1/auth/password/forgot", Requests: 5, Window: time.Hour},
		{Method: http.MethodPost, Path: "/v1/auth/password/reset", Requests: 10, Window: time.Hour},
		{Method: http.MethodPost, Path: "/v1/auth/token/refresh", Requests: 60, Window: time.Minute},
	}
}
