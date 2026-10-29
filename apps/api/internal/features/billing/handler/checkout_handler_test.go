package handler_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

const checkoutsPath = "/v1/workspace/billing/checkout-sessions"

func TestCheckoutPermissions(t *testing.T) {
	expectGuarded(t, []route{{
		method: http.MethodPost, path: checkoutsPath,
		body: map[string]any{"plan_price_id": uuid.New()}, args: []any{idempotencyKey()},
	}}, authctx.PermBillingPay)
	expectGuarded(t, []route{{method: http.MethodGet, path: checkoutsPath + "/" + uuid.NewString()}},
		authctx.PermBillingView)
	expectGuarded(t, []route{{method: http.MethodPost, path: checkoutsPath + "/" + uuid.NewString() + "/test-complete"}},
		authctx.PermBillingPay)
}

func TestCreateCheckoutValidates(t *testing.T) {
	e := newEnv(t, nil)
	m := e.payer(uuid.New())
	price := map[string]any{"plan_price_id": uuid.New()}

	// The Idempotency-Key header is required and at least 8 characters.
	expect(t, m.do(http.MethodPost, checkoutsPath, price), http.StatusUnprocessableEntity)
	expect(t, m.do(http.MethodPost, checkoutsPath, price, "Idempotency-Key: short"), http.StatusUnprocessableEntity)
	// provider must be one Relay knows.
	expect(t, m.do(http.MethodPost, checkoutsPath,
		map[string]any{"plan_price_id": uuid.New(), "provider": "bitcoin"}, idempotencyKey()),
		http.StatusUnprocessableEntity)
	expect(t, m.do(http.MethodPost, checkoutsPath, map[string]any{}, idempotencyKey()), http.StatusUnprocessableEntity)
	expect(t, m.do(http.MethodGet, checkoutsPath+"/not-a-uuid", nil), http.StatusUnprocessableEntity)
}
