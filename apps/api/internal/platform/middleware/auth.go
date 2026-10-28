package middleware

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/jwt"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// TokenVerifier checks an access token.
type TokenVerifier interface {
	Verify(token string) (jwt.Claims, error)
}

// SessionChecker reports whether the session behind a valid token is still active.
type SessionChecker interface {
	SessionActive(ctx context.Context, userID, sessionID uuid.UUID) (bool, error)
}

var (
	errUnauthenticated = apperr.Unauthorized("unauthenticated", "Sign in to continue")
	errBadOrigin       = apperr.Forbidden("invalid_origin", "Request origin is not allowed")
	errSessionCheck    = apperr.Unavailable("session_check_failed", "Could not check your session. Try again.")
)

// RequireAuth is a huma middleware for operations that need a signed-in user.
// It accepts "Authorization: Bearer <token>" (mobile and API clients) or the
// access cookie (web). Cookie requests that change data must come from an
// allowed Origin, which blocks cross-site request forgery.
//
// After the token's signature is verified, the session it names must still be
// active, so revoking a session or logging out takes effect immediately.
func RequireAuth(api huma.API, v TokenVerifier, sessions SessionChecker, allowedOrigins []string) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		token, fromCookie := bearerToken(ctx.Header("Authorization")), false
		if token == "" {
			if c, err := huma.ReadCookie(ctx, authctx.AccessCookie); err == nil {
				token, fromCookie = c.Value, true
			}
		}
		if token == "" {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "", errUnauthenticated)
			return
		}
		if fromCookie && !SameOrigin(ctx.Method(), ctx.Header("Origin"), ctx.Header("Referer"), allowedOrigins) {
			_ = huma.WriteErr(api, ctx, http.StatusForbidden, "", errBadOrigin)
			return
		}

		claims, err := v.Verify(token)
		if err != nil {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "", errUnauthenticated)
			return
		}
		active, err := sessions.SessionActive(ctx.Context(), claims.UserID, claims.SessionID)
		if err != nil {
			_ = huma.WriteErr(api, ctx, http.StatusServiceUnavailable, "", errSessionCheck.Wrap(err))
			return
		}
		if !active {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "", errUnauthenticated)
			return
		}
		next(huma.WithContext(ctx, authctx.With(ctx.Context(), claims.UserID, claims.SessionID)))
	}
}

// SameOrigin reports whether a cookie-authenticated request may proceed.
// Safe methods always may; others need an Origin (or Referer) in allowed.
func SameOrigin(method, origin, referer string, allowed []string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	if origin == "" && referer != "" {
		if u, err := url.Parse(referer); err == nil && u.Host != "" {
			origin = u.Scheme + "://" + u.Host
		}
	}
	if origin == "" {
		return false
	}
	for _, a := range allowed {
		if strings.EqualFold(origin, a) {
			return true
		}
	}
	return false
}

func bearerToken(h string) string {
	scheme, token, ok := strings.Cut(h, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}
