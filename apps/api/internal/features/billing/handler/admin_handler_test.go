package handler_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestAdminRoutesNeedStaff(t *testing.T) {
	e := dbEnv(t)
	ws := e.workspace()
	// A workspace owner is still not staff.
	owner := e.payer(ws)
	staff := e.payer(ws)
	e.staff[staff.userID] = true

	expect(t, e.do(http.MethodGet, "/v1/admin/billing/catalog", nil), http.StatusUnauthorized)
	expect(t, owner.do(http.MethodGet, "/v1/admin/billing/catalog", nil), http.StatusForbidden, "staff_only")
	expect(t, owner.do(http.MethodPost, "/v1/admin/billing/plans", map[string]any{
		"code": "nope", "name": "Nope", "limits": map[string]any{"members": 1, "whatsapp_numbers": 1},
	}), http.StatusForbidden, "staff_only")

	expect(t, staff.do(http.MethodGet, "/v1/admin/billing/catalog", nil), http.StatusOK)
	r := staff.do(http.MethodPost, "/v1/admin/billing/plans", map[string]any{
		"code": "adm_" + uuid.NewString()[:8], "name": "Staff plan", "trial_days": 0, "trial_requires_card": false,
		"sort_order": 0, "limits": map[string]any{"members": 5, "whatsapp_numbers": -1},
	})
	expect(t, r, http.StatusCreated)
	expect(t, staff.do(http.MethodPost, "/v1/admin/billing/plans/"+r.body["id"].(string)+"/deactivate", nil),
		http.StatusNoContent)
	expect(t, staff.do(http.MethodGet, "/v1/admin/workspaces?search=Handler", nil), http.StatusOK)
	expect(t, staff.do(http.MethodPost, "/v1/admin/workspaces/end-grant", map[string]any{"workspace_id": ws.String()}),
		http.StatusNotFound, "grant_not_found")
}
