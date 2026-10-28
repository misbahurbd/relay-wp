package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimitPerClientIP(t *testing.T) {
	ipr, _ := NewClientIPResolver(nil)
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := ipr.Middleware(RateLimit(2, time.Minute)(ok))

	call := func(remote string) int {
		req := httptest.NewRequestWithContext(t.Context(), "GET", "/", nil)
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	for i := range 2 {
		if c := call("203.0.113.1:1"); c != http.StatusOK {
			t.Fatalf("request %d: status %d", i+1, c)
		}
	}
	if c := call("203.0.113.1:1"); c != http.StatusTooManyRequests {
		t.Errorf("third request: status %d, want 429", c)
	}
	if c := call("203.0.113.2:1"); c != http.StatusOK {
		t.Errorf("other client: status %d, want 200", c)
	}
}

func TestRateLimitExemptPath(t *testing.T) {
	ipr, _ := NewClientIPResolver(nil)
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := ipr.Middleware(RateLimit(1, time.Minute, "/healthz")(ok))

	for i := range 3 {
		req := httptest.NewRequestWithContext(t.Context(), "GET", "/healthz", nil)
		req.RemoteAddr = "203.0.113.1:1"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d to exempt path: status %d", i+1, rec.Code)
		}
	}
}
