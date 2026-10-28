package middleware

import (
	"errors"
	"net/http"
	"runtime/debug"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/httpx"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/logger"
)

// Recoverer turns a panic into a logged 500 with the standard error body.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler { //nolint:errorlint // sentinel compared as net/http does
				panic(rec)
			}
			logger.FromContext(r.Context()).Error("panic recovered", "panic", rec, "stack", string(debug.Stack()))
			httpx.WriteError(w, errors.New("panic"))
		}()
		next.ServeHTTP(w, r)
	})
}
