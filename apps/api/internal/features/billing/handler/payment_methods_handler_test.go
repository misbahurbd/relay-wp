package handler_test

import (
	"net/http"
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

func TestPaymentMethodNeedsBillingView(t *testing.T) {
	expectGuarded(t, []route{{method: http.MethodGet, path: "/v1/workspace/billing/payment-method"}}, authctx.PermBillingView)
}

func TestPortalSessionNeedsBillingPay(t *testing.T) {
	expectGuarded(t, []route{{method: http.MethodPost, path: "/v1/workspace/billing/portal-session"}}, authctx.PermBillingPay)
}
