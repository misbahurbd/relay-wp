package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/jwt"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// TokenVerifier checks an access token.
type TokenVerifier interface {
	Verify(token string) (jwt.Claims, error)
}

var (
	errUnauthenticated = apperr.Unauthorized("unauthenticated", "Sign in to continue")
	errBadOrigin       = apperr.Forbidden("invalid_origin", "Request origin is not allowed")
)

// RequireAuth is a huma middleware for operations that need a signed-in user.
// It accepts "Authorization: Bearer <token>" (mobile and API clients) or the
// access cookie (web). Cookie requests that change data must come from an
// allowed Origin, which blocks cross-site request forgery.
func RequireAuth(api huma.API, v TokenVerifier, allowedOrigins []string) func(huma.Context, func(huma.Context)) {
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
