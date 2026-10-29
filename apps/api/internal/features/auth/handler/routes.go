package handler

import (
	"net/http"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/model"

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
	huma.Register(api, public("invite-signup", http.MethodPost, "/v1/auth/signup/invite",
		"Create an account from an invite link, sign in and join the workspace", http.StatusOK), h.inviteSignup)
	huma.Register(api, public("verify-email", http.MethodPost, "/v1/auth/verify-email",
		"Verify an email address from the emailed link (with the account's password, if it has one)", http.StatusNoContent), h.verifyEmail)
	huma.Register(api, public("verify-email-code", http.MethodPost, "/v1/auth/verify-email/code",
		"Verify an email address with the emailed 6-digit code and sign in", http.StatusOK), h.verifyEmailCode)
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

	// Social sign-in (docs/PHASE-2-AUTH.md section 4; flow notes in oauth.go).
	huma.Register(api, public("list-oauth-providers", http.MethodGet, "/v1/auth/providers",
		"Social sign-in providers this server offers", http.StatusOK), h.listProviders)
	huma.Register(api, public("oauth-start", http.MethodGet, "/v1/auth/oauth/{provider}/start",
		"Start social sign-in: redirects to the provider", http.StatusFound), h.oauthStart)
	huma.Register(api, public("oauth-callback", http.MethodGet, "/v1/auth/oauth/{provider}/callback",
		"Provider callback: signs in, then redirects to redirect_to", http.StatusFound), h.oauthCallback)
	huma.Register(api, public("oauth-complete", http.MethodPost, "/v1/auth/oauth/complete",
		"Social sign-in without a provider-verified email: email a code to the address given", http.StatusAccepted), h.oauthComplete)
	huma.Register(api, public("oauth-complete-verify", http.MethodPost, "/v1/auth/oauth/complete/verify",
		"Finish that sign-in with the emailed code", http.StatusOK), h.oauthCompleteVerify)
	huma.Register(api, public("oauth-token", http.MethodPost, "/v1/auth/oauth/token",
		"Mobile: redeem the one-time code from the callback for tokens", http.StatusOK), h.oauthToken)
	huma.Register(api, public("facebook-data-deletion", http.MethodPost, "/v1/auth/oauth/facebook/data-deletion",
		"Meta data-deletion callback: disconnects that Facebook account", http.StatusOK), h.facebookDataDeletion)
	huma.Register(api, authed("oauth-link", http.MethodGet, "/v1/auth/oauth/{provider}/link",
		"Connect a provider from settings (needs a recent sign-in)", http.StatusFound), h.oauthLink)
	huma.Register(api, authed("list-identities", http.MethodGet, "/v1/auth/identities",
		"Connected sign-in providers, and whether a password is set", http.StatusOK), h.listIdentities)
	huma.Register(api, authed("unlink-identity", http.MethodDelete, "/v1/auth/identities/{provider}",
		"Disconnect a provider (needs a recent sign-in; never the last sign-in method)", http.StatusNoContent), h.unlinkIdentity)
	huma.Register(api, authed("set-password", http.MethodPost, "/v1/auth/password/set",
		"Add a password to an account that only uses social sign-in (needs a recent sign-in)", http.StatusNoContent), h.setPassword)
}

// RateLimits are per-IP limits for auth endpoints, on top of the global limit.
func RateLimits() []middleware.RouteLimit {
	limits := []middleware.RouteLimit{
		{Method: http.MethodPost, Path: "/v1/auth/login", Requests: 10, Window: time.Minute},
		{Method: http.MethodPost, Path: "/v1/auth/signup", Requests: 5, Window: time.Hour},
		{Method: http.MethodPost, Path: "/v1/auth/signup/invite", Requests: 10, Window: time.Hour},
		{Method: http.MethodPost, Path: "/v1/auth/verify-email/resend", Requests: 5, Window: time.Hour},
		// The link page checks a password, so it gets a login-like limit.
		{Method: http.MethodPost, Path: "/v1/auth/verify-email", Requests: 10, Window: time.Minute},
		// Each code also dies after 5 wrong tries; this stops one IP cycling through accounts.
		{Method: http.MethodPost, Path: "/v1/auth/verify-email/code", Requests: 20, Window: 15 * time.Minute},
		{Method: http.MethodPost, Path: "/v1/auth/password/forgot", Requests: 5, Window: time.Hour},
		{Method: http.MethodPost, Path: "/v1/auth/password/reset", Requests: 10, Window: time.Hour},
		{Method: http.MethodPost, Path: "/v1/auth/token/refresh", Requests: 60, Window: time.Minute},
		{Method: http.MethodPost, Path: "/v1/auth/oauth/complete", Requests: 10, Window: time.Hour},
		{Method: http.MethodPost, Path: "/v1/auth/oauth/complete/verify", Requests: 20, Window: 15 * time.Minute},
		{Method: http.MethodPost, Path: "/v1/auth/password/change", Requests: 10, Window: 15 * time.Minute},
		{Method: http.MethodPost, Path: "/v1/auth/oauth/token", Requests: 30, Window: time.Minute},
		{Method: http.MethodPost, Path: "/v1/auth/password/set", Requests: 10, Window: time.Hour},
	}
	for _, p := range []string{model.ProviderGoogle, model.ProviderFacebook, "apple"} {
		limits = append(limits,
			middleware.RouteLimit{Method: http.MethodGet, Path: "/v1/auth/oauth/" + p + "/start", Requests: 30, Window: time.Minute},
			middleware.RouteLimit{Method: http.MethodGet, Path: "/v1/auth/oauth/" + p + "/callback", Requests: 30, Window: time.Minute},
		)
	}
	return limits
}
