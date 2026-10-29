package handler_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

const ordersPath = "/v1/workspace/billing/orders"

func TestOrderReadsNeedBillingView(t *testing.T) {
	expectGuarded(t, []route{
		{method: http.MethodPost, path: ordersPath + "/quote", body: map[string]any{"plan_price_id": uuid.New()}},
		{method: http.MethodGet, path: ordersPath + "/current"},
		{method: http.MethodGet, path: ordersPath + "/" + uuid.NewString()},
	}, authctx.PermBillingView)
}

func TestOrderPaymentsNeedBillingPay(t *testing.T) {
	order := ordersPath + "/" + uuid.NewString()
	expectGuarded(t, []route{
		{method: http.MethodPost, path: order + "/installments/" + uuid.NewString() + "/pay", args: []any{idempotencyKey()}},
		{method: http.MethodPost, path: order + "/resume", args: []any{idempotencyKey()}},
	}, authctx.PermBillingPay)
}

func TestOrderPaymentsNeedAnIdempotencyKey(t *testing.T) {
	e := newEnv(t, nil)
	m := e.payer(uuid.New())
	order := ordersPath + "/" + uuid.NewString()
	for _, path := range []string{order + "/installments/" + uuid.NewString() + "/pay", order + "/resume"} {
		expect(t, m.do(http.MethodPost, path, nil), http.StatusUnprocessableEntity)
		expect(t, m.do(http.MethodPost, path, nil, "Idempotency-Key: short"), http.StatusUnprocessableEntity)
	}
	expect(t, m.do(http.MethodPost, ordersPath+"/quote", map[string]any{
		"plan_price_id": uuid.New(), "discount_code": string(make([]byte, 41)),
	}), http.StatusUnprocessableEntity)
}
