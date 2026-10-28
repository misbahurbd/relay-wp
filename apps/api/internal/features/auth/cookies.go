package auth

import (
	"net/http"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// refreshCookiePath limits the refresh cookie to the one endpoint that needs it.
const refreshCookiePath = "/v1/auth/token/refresh"

// CookieConfig controls the auth cookies sent to the web app.
type CookieConfig struct {
	Domain string
	Secure bool
}

func (c CookieConfig) set(t Tokens, now time.Time) []http.Cookie {
	return []http.Cookie{
		c.cookie(authctx.AccessCookie, t.AccessToken, "/", t.AccessExpiresAt.Sub(now)),
		c.cookie(authctx.RefreshCookie, t.RefreshToken, refreshCookiePath, t.RefreshExpiresAt.Sub(now)),
	}
}

func (c CookieConfig) clear() []http.Cookie {
	return []http.Cookie{
		c.cookie(authctx.AccessCookie, "", "/", -1),
		c.cookie(authctx.RefreshCookie, "", refreshCookiePath, -1),
	}
}

func (c CookieConfig) cookie(name, value, path string, ttl time.Duration) http.Cookie {
	maxAge := int(ttl.Seconds())
	if ttl < 0 {
		maxAge = -1 // delete
	}
	// Secure comes from COOKIE_SECURE: true in production, false for plain-HTTP local development.
	return http.Cookie{ //nolint:gosec // Secure is configuration-driven, see above
		Name:     name,
		Value:    value,
		Path:     path,
		Domain:   c.Domain,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   c.Secure,
		SameSite: http.SameSiteLaxMode,
	}
}
