package handler_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

const discountPreviewPath = "/v1/workspace/billing/discounts/preview"

func TestPreviewDiscountNeedsBillingView(t *testing.T) {
	expectGuarded(t, []route{{
		method: http.MethodPost, path: discountPreviewPath,
		body: map[string]any{"code": "LAUNCH20", "plan_price_id": uuid.New()},
	}}, authctx.PermBillingView)
}

func TestPreviewDiscountValidates(t *testing.T) {
	e := newEnv(t, nil)
	m := e.viewer(uuid.New())
	expect(t, m.do(http.MethodPost, discountPreviewPath,
		map[string]any{"code": "AB", "plan_price_id": uuid.New()}), http.StatusUnprocessableEntity)
	expect(t, m.do(http.MethodPost, discountPreviewPath, map[string]any{"code": "LAUNCH20"}), http.StatusUnprocessableEntity)
}
