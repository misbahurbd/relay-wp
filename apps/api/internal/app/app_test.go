package app

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database/dbtest"
)

func testApp(t *testing.T) *App {
	t.Helper()
	cfg := config.Config{HTTP: config.HTTP{
		CORSAllowedOrigins: []string{"http://localhost:3000"},
		RateLimitRequests:  100,
		RateLimitWindow:    time.Minute,
	}}
	return &App{cfg: cfg, log: slog.Default(), db: dbtest.New(t)}
}

func do(t *testing.T, a *App, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	h, err := a.Handler()
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthAndReady(t *testing.T) {
	a := testApp(t)

	for _, path := range []string{"/healthz", "/readyz"} {
		rec := do(t, a, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d, body %s", path, rec.Code, rec.Body)
		}
	}
}

func TestReadyFailsWhenDatabaseIsDown(t *testing.T) {
	a := testApp(t)
	a.db.Close()

	rec := do(t, a, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", rec.Code)
	}
	var body struct{ Code, Message string }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "database_unavailable" {
		t.Errorf("code = %q", body.Code)
	}
}

func TestUnknownRouteReturnsJSON404(t *testing.T) {
	rec := do(t, testApp(t), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/nope", nil))
	if rec.Code != http.StatusNotFound || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status %d, content-type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
}

func TestCORSAllowsConfiguredOriginOnly(t *testing.T) {
	a := testApp(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	if got := do(t, a, req).Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("allowed origin header = %q", got)
	}

	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://evil.test")
	if got := do(t, a, req).Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("unexpected allow-origin for foreign origin: %q", got)
	}
}

func TestOpenAPIListsRoutesAndErrorShape(t *testing.T) {
	spec := OpenAPI()
	for _, p := range []string{"/healthz", "/readyz"} {
		if spec.Paths[p] == nil {
			t.Errorf("spec is missing %s", p)
		}
	}
	if _, ok := spec.Components.Schemas.Map()["ErrorBody"]; !ok {
		t.Error("spec is missing the ErrorBody schema")
	}
}
