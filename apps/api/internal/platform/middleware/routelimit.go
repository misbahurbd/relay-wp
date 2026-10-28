package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/httprate"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/httpx"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// RouteLimit is a per-IP limit for one method and exact path.
type RouteLimit struct {
	Method   string
	Path     string
	Requests int
	Window   time.Duration
}

// RouteLimits applies each RouteLimit to its route; other routes pass through.
// Like RateLimit, counters are in memory per instance.
func RouteLimits(limits ...RouteLimit) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		routes := make(map[string]http.Handler, len(limits))
		for _, l := range limits {
			routes[l.Method+" "+l.Path] = httprate.LimitBy(l.Requests, l.Window,
				func(r *http.Request) (string, error) { return ClientIP(r.Context()), nil },
				httprate.WithLimitHandler(func(w http.ResponseWriter, _ *http.Request) {
					httpx.WriteError(w, apperr.RateLimited("rate_limited", "Too many requests. Try again later."))
				}),
			)(next)
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if h, ok := routes[r.Method+" "+r.URL.Path]; ok {
				h.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
