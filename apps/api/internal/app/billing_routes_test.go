package app

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

var pathParamRe = regexp.MustCompile(`\{[A-Za-z]+\}`)

// billingRoutes lists every billing route in the OpenAPI spec as method and a
// concrete path (path parameters filled in).
func billingRoutes(t *testing.T) [][2]string {
	t.Helper()
	var routes [][2]string
	for path, item := range OpenAPI().Paths {
		if !strings.Contains(path, "/billing") && !strings.HasPrefix(path, "/v1/webhooks/payments/") {
			continue
		}
		concrete := pathParamRe.ReplaceAllStringFunc(path, func(p string) string {
			if p == "{provider}" {
				return "stripe"
			}
			return uuid.NewString()
		})
		for method, op := range map[string]*huma.Operation{
			http.MethodGet: item.Get, http.MethodPost: item.Post, http.MethodPut: item.Put,
			http.MethodPatch: item.Patch, http.MethodDelete: item.Delete,
		} {
			if op != nil {
				routes = append(routes, [2]string{method, concrete})
			}
		}
	}
	return routes
}

// TestBillingRoutesAreStubs: Wave 0 registers every billing route; each answers
// 501 until its track builds it. Delete this test once none do.
func TestBillingRoutesAreStubs(t *testing.T) {
	e := newAuthEnv(t)
	w := newTeamWorkspace(e)
	owner := w.owner
	owner.idempotency = "wave0-" + uuid.NewString()

	routes := billingRoutes(t)
	if len(routes) < 21 {
		t.Fatalf("found %d billing routes in the spec, want at least 21", len(routes))
	}
	for _, r := range routes {
		method, path := r[0], r[1]
		var body any
		if method != http.MethodGet && method != http.MethodDelete {
			body = billingBody(path)
		}
		got := owner.do(method, path, body)
		if got.code != http.StatusNotImplemented || got.body["code"] != "not_implemented" {
			t.Errorf("%s %s: status %d, want 501 not_implemented; body: %s", method, path, got.code, got.raw)
		}
	}

	// The workspace routes still check who is asking.
	stranger := e.browser(chromeMac)
	expect(t, stranger.in(w.id).do(http.MethodGet, "/v1/workspace/billing", nil), http.StatusUnauthorized)
	// The catalog is public.
	expect(t, stranger.do(http.MethodGet, "/v1/billing/plans", nil), http.StatusNotImplemented, "not_implemented")
}

// billingBody is a request body that passes validation for path, so the stub is reached.
func billingBody(path string) any {
	price := uuid.NewString()
	switch {
	case strings.HasSuffix(path, "/checkout-sessions"):
		return map[string]any{"plan_price_id": price}
	case strings.HasSuffix(path, "/discounts/preview"):
		return map[string]any{"code": "LAUNCH20", "plan_price_id": price}
	case strings.HasSuffix(path, "/subscription/change"), strings.HasSuffix(path, "/subscription/change-preview"):
		return map[string]any{"plan_price_id": price}
	case strings.HasSuffix(path, "/subscription/cancel"):
		return map[string]any{"at_period_end": true}
	case strings.HasSuffix(path, "/orders/quote"):
		return map[string]any{"plan_price_id": price}
	case strings.HasSuffix(path, "/profile"):
		return map[string]any{"legal_name": "Acme Ltd", "email": "billing@acme.test"}
	case strings.HasPrefix(path, "/v1/webhooks/payments/"):
		return map[string]any{"id": "evt_1"}
	default:
		return map[string]any{}
	}
}
