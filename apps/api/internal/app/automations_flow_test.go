package app

import (
	"net/http"
	"testing"
)

func includeAutomations(e *authEnv) {
	e.exec(`UPDATE plans SET limits = limits || '{"automations": 1}'`)
}

func TestAutomationsNeedThePlan(t *testing.T) {
	e := newAuthEnv(t)
	w := newTeamWorkspace(e)
	expect(t, w.owner.do("GET", "/v1/workspace/automations", nil), http.StatusPaymentRequired, "plan_required")

	includeAutomations(e)
	list := w.owner.do("GET", "/v1/workspace/automations", nil)
	expect(t, list, http.StatusOK)
	got := items(list)
	want := []string{"welcome", "away", "keyword", "cart", "order", "drip"}
	if len(got) != len(want) {
		t.Fatalf("list: %s", list.raw)
	}
	for i, k := range want {
		if got[i]["kind"] != k || got[i]["enabled"] != false {
			t.Fatalf("item %d: %v", i, got[i])
		}
	}
}

func TestAutomationPermissions(t *testing.T) {
	e := newAuthEnv(t)
	includeAutomations(e)
	w := newTeamWorkspace(e)
	base := "/v1/workspace/automations"
	welcome := map[string]any{"welcome": map[string]any{"message": "Hi", "delay_minutes": 1, "first_time_only": true}}

	viewer, _ := w.join("viewer")
	expect(t, viewer.do("GET", base, nil), http.StatusOK)
	expect(t, viewer.do("PUT", base+"/welcome", welcome), http.StatusForbidden, "permission_denied")
	expect(t, viewer.do("PUT", base+"/welcome/enabled", map[string]any{"enabled": true}), http.StatusForbidden, "permission_denied")

	saved := w.owner.do("PUT", base+"/welcome", welcome)
	expect(t, saved, http.StatusOK)
	expect(t, w.owner.do("PUT", base+"/welcome", map[string]any{"away": map[string]any{}}), http.StatusUnprocessableEntity)
	expect(t, w.owner.do("PUT", base+"/keyword/enabled", map[string]any{"enabled": true}), http.StatusConflict, "automation_not_ready")
	on := w.owner.do("PUT", base+"/welcome/enabled", map[string]any{"enabled": true})
	expect(t, on, http.StatusOK)
	if on.body["enabled"] != true {
		t.Fatalf("enable: %s", on.raw)
	}

	// Another workspace can't see this one's settings.
	other := newTeamWorkspace(e)
	other.owner.workspace = w.id
	expect(t, other.owner.do("GET", base, nil), http.StatusNotFound, "workspace_not_found")
}
