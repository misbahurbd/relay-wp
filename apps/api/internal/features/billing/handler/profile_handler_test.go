package handler_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

const profilePath = "/v1/workspace/billing/profile"

func TestProfilePermissions(t *testing.T) {
	expectGuarded(t, []route{{method: http.MethodGet, path: profilePath}}, authctx.PermBillingView)
	expectGuarded(t, []route{{
		method: http.MethodPut, path: profilePath,
		body: map[string]any{"legal_name": "Acme Ltd", "email": "billing@acme.test"},
	}}, authctx.PermBillingPay)
}

func TestUpdateProfileValidates(t *testing.T) {
	e := newEnv(t, nil)
	m := e.payer(uuid.New())
	for _, body := range []map[string]any{
		{"email": "billing@acme.test"},
		{"legal_name": "", "email": "billing@acme.test"},
		{"legal_name": "Acme Ltd", "email": "not an email"},
		{"legal_name": "Acme Ltd", "email": "billing@acme.test", "country": "bd"},
	} {
		expect(t, m.do(http.MethodPut, profilePath, body), http.StatusUnprocessableEntity)
	}
}
