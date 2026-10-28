package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/httprate"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/httpx"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// RateLimit allows requests per window for each client IP (from ClientIP).
// Paths in exempt (e.g. health checks) are never limited.
// Counters are in memory, so each instance limits on its own; move to a shared
// store (platform/cache) when running several instances.
func RateLimit(requests int, window time.Duration, exempt ...string) func(http.Handler) http.Handler {
	limit := httprate.LimitBy(requests, window,
		func(r *http.Request) (string, error) { return ClientIP(r.Context()), nil },
		httprate.WithLimitHandler(func(w http.ResponseWriter, _ *http.Request) {
			httpx.WriteError(w, apperr.RateLimited("rate_limited", "Too many requests. Try again shortly."))
		}),
	)
	skip := make(map[string]bool, len(exempt))
	for _, p := range exempt {
		skip[p] = true
	}
	return func(next http.Handler) http.Handler {
		limited := limit(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skip[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			limited.ServeHTTP(w, r)
		})
	}
}
