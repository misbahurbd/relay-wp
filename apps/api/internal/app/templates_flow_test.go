package app

import (
	"net/http"
	"testing"
)

func TestTemplatesFlow(t *testing.T) {
	e := newAuthEnv(t)
	w := newTeamWorkspace(e)
	owner, base := w.owner, w.base+"/templates"

	draft := map[string]any{
		"name": "order_update", "language": "en_US", "category": "utility",
		"components": map[string]any{
			"header":  map[string]any{"type": "text", "text": "Order {{1}}", "example": "#4821"},
			"body":    map[string]any{"text": "Hi {{1}}, your order {{2}} shipped.", "examples": []string{"Daniel", "#4821"}},
			"buttons": []any{map[string]any{"type": "url", "text": "Track", "value": "https://bit.ly/x"}},
		},
	}
	created := owner.do("POST", base, draft)
	expect(t, created, http.StatusCreated)
	id := created.body["id"].(string)
	if created.body["status"] != "draft" || created.body["meta_template_id"] != nil {
		t.Fatalf("created: %s", created.raw)
	}
	if ws := created.body["warnings"].([]any); len(ws) != 1 || ws[0].(map[string]any)["field"] != "buttons[0].value" {
		t.Fatalf("warnings: %s", created.raw)
	}
	expect(t, owner.do("POST", base, draft), http.StatusConflict, "template_exists")

	// Validation errors carry field locations the web maps to inputs.
	bad := owner.do("PATCH", base+"/"+id, map[string]any{
		"components": map[string]any{"body": map[string]any{"text": "{{1}} starts"}, "footer": map[string]any{"text": "Bye {{1}}"}},
	})
	expect(t, bad, http.StatusUnprocessableEntity, "invalid_template")
	fields := map[string]bool{}
	for _, d := range bad.body["details"].([]any) {
		fields[d.(map[string]any)["field"].(string)] = true
	}
	if !fields["body.text"] || !fields["footer.text"] {
		t.Fatalf("details: %s", bad.raw)
	}

	patched := owner.do("PATCH", base+"/"+id, map[string]any{"category": "marketing"})
	expect(t, patched, http.StatusOK)
	if patched.body["category"] != "marketing" || patched.body["components"].(map[string]any)["body"].(map[string]any)["text"] == "" {
		t.Fatalf("patched: %s", patched.raw)
	}

	list := owner.do("GET", base+"?category=marketing&q=ORDER", nil)
	expect(t, list, http.StatusOK)
	if len(list.body["items"].([]any)) != 1 || list.body["counts"].(map[string]any)["all"] != float64(1) {
		t.Fatalf("list: %s", list.raw)
	}
	expect(t, owner.do("GET", base+"?cursor=nope!", nil), http.StatusUnprocessableEntity, "invalid_cursor")

	// No WhatsApp account yet.
	expect(t, owner.do("POST", base+"/"+id+"/submit", nil), http.StatusConflict, "whatsapp_not_connected")
	expect(t, owner.do("POST", base+"/sync", nil), http.StatusConflict, "whatsapp_not_connected")

	// Viewers can read templates but not change them.
	viewer, _ := w.join("viewer")
	expect(t, viewer.do("GET", base+"/"+id, nil), http.StatusOK)
	expect(t, viewer.do("POST", base, map[string]any{"name": "x", "language": "en_US", "category": "utility"}), http.StatusForbidden)
	expect(t, viewer.do("DELETE", base+"/"+id, nil), http.StatusForbidden)

	// Another workspace can't see it.
	stranger := e.signedIn().in(w.id)
	expect(t, stranger.do("GET", base+"/"+id, nil), http.StatusNotFound, "workspace_not_found")

	expect(t, owner.do("DELETE", base+"/"+id, nil), http.StatusNoContent)
	expect(t, owner.do("GET", base+"/"+id, nil), http.StatusNotFound, "template_not_found")
}
