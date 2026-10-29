package handler_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

const subscriptionPath = "/v1/workspace/billing/subscription"

func TestSubscriptionReadsNeedBillingView(t *testing.T) {
	expectGuarded(t, []route{
		{method: http.MethodGet, path: "/v1/workspace/billing"},
		{method: http.MethodGet, path: subscriptionPath},
		{method: http.MethodPost, path: subscriptionPath + "/change-preview", body: map[string]any{"plan_price_id": uuid.New()}},
	}, authctx.PermBillingView)
}

func TestSubscriptionChangesNeedBillingPay(t *testing.T) {
	expectGuarded(t, []route{
		{method: http.MethodPost, path: subscriptionPath + "/change", body: map[string]any{"plan_price_id": uuid.New()}},
		{method: http.MethodDelete, path: subscriptionPath + "/pending-change"},
		{method: http.MethodPost, path: subscriptionPath + "/cancel", body: map[string]any{"at_period_end": true}},
		{method: http.MethodPost, path: subscriptionPath + "/resume"},
	}, authctx.PermBillingPay)
}

func TestSubscriptionChangesValidate(t *testing.T) {
	e := newEnv(t, nil)
	m := e.payer(uuid.New())
	expect(t, m.do(http.MethodPost, subscriptionPath+"/change", map[string]any{}), http.StatusUnprocessableEntity)
	expect(t, m.do(http.MethodPost, subscriptionPath+"/change-preview",
		map[string]any{"plan_price_id": "nope"}), http.StatusUnprocessableEntity)
	expect(t, m.do(http.MethodPost, subscriptionPath+"/cancel", map[string]any{}), http.StatusUnprocessableEntity)
}
