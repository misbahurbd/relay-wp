package app

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

const segmentsBase = "/v1/workspace/contact-segments"

func cond(field, op, value string) map[string]any {
	return map[string]any{"field": field, "op": op, "value": value}
}

func rules(groups ...map[string]any) map[string]any { return map[string]any{"groups": groups} }

func group(match string, conds ...map[string]any) map[string]any {
	return map[string]any{"match": match, "conditions": conds}
}

func TestSegmentRulesMatchContacts(t *testing.T) {
	b := newBroadcastsEnv(t)
	vip := b.tag("VIP")
	grace := b.contact("Grace", "+447700900041", "in", vip)
	b.contact("Alan", "+447700900042", "pending", vip)
	b.contact("Ada", "+15550100043", "in")
	b.e.exec(`UPDATE contacts SET last_message_at = now() - interval '40 days', language = 'bn' WHERE id = $1`, grace)

	// VIP and opted in, OR a US number.
	r := rules(
		group("all", cond("tag", "is", vip.String()), cond("opt_in", "is", "in")),
		group("any", cond("country_code", "starts_with", "+1")),
	)
	preview := b.owner.do("POST", segmentsBase+"/preview", map[string]any{"rules": r})
	expect(t, preview, http.StatusOK)
	count := preview.body["count"].(map[string]any)
	if count["matched"].(float64) != 2 || count["opted_in"].(float64) != 2 || len(preview.body["sample"].([]any)) != 2 {
		t.Fatalf("preview: %s", preview.raw)
	}

	// Last message more than 30 days ago and language bn: only Grace.
	quiet := b.owner.do("POST", segmentsBase+"/preview", map[string]any{"rules": rules(group("all",
		cond("last_message", "over", "30"), cond("language", "is", "BN"), map[string]any{"field": "opt_in", "op": "is_not", "value": "out"},
	))})
	expect(t, quiet, http.StatusOK)
	if quiet.body["count"].(map[string]any)["matched"].(float64) != 1 {
		t.Fatalf("quiet: %s", quiet.raw)
	}

	saved := b.owner.do("POST", segmentsBase, map[string]any{"name": "  VIP   or US ", "description": "Test", "rules": r})
	expect(t, saved, http.StatusCreated)
	if saved.body["name"] != "VIP or US" || saved.body["count"].(map[string]any)["matched"].(float64) != 2 {
		t.Fatalf("saved: %s", saved.raw)
	}
	expect(t, b.owner.do("POST", segmentsBase, map[string]any{"name": "vip or us", "rules": r}), http.StatusConflict, "segment_name_taken")
	list := b.owner.do("GET", segmentsBase, nil)
	expect(t, list, http.StatusOK)
	if len(items(list)) != 1 {
		t.Fatalf("list: %s", list.raw)
	}
}

func TestSegmentRulesAreCheckedAndValuesStayValues(t *testing.T) {
	b := newBroadcastsEnv(t)
	b.contact("Grace", "+447700900051", "in")

	bad := b.owner.do("POST", segmentsBase+"/preview", map[string]any{"rules": rules(group("all",
		cond("last_message", "within", "0"), cond("country_code", "starts_with", "44"), cond("opt_in", "is", "maybe"),
	))})
	expect(t, bad, http.StatusUnprocessableEntity, "invalid_segment")
	for _, f := range []string{"conditions[0]", "conditions[1]", "conditions[2]"} {
		if !strings.Contains(bad.raw, f) {
			t.Fatalf("missing %s: %s", f, bad.raw)
		}
	}
	expect(t, b.owner.do("POST", segmentsBase+"/preview", map[string]any{"rules": rules()}), http.StatusUnprocessableEntity, "invalid_segment")

	// SQL in a value is only a value: it matches nothing and breaks nothing.
	inject := b.owner.do("POST", segmentsBase+"/preview", map[string]any{"rules": rules(group("any",
		map[string]any{"field": "attribute", "op": "contains", "key": "x') OR true --", "value": "' OR 1=1 --"},
		cond("language", "is", "'; DROP TABLE contacts; --"),
	))})
	expect(t, inject, http.StatusOK)
	if inject.body["count"].(map[string]any)["matched"].(float64) != 0 {
		t.Fatalf("injection matched: %s", inject.raw)
	}
	if b.count(`SELECT count(*) FROM contacts WHERE workspace_id = $1`, b.wsID) != 1 {
		t.Fatal("contacts changed")
	}

	// Another workspace's tag can't be used.
	other := newTeamWorkspace(b.e)
	foreign := uuid.Must(uuid.NewV7())
	b.e.exec(`INSERT INTO contact_tags (id, workspace_id, name) VALUES ($1, $2, 'theirs')`, foreign, other.id)
	expect(t, b.owner.do("POST", segmentsBase, map[string]any{"name": "Theirs", "rules": rules(group("all", cond("tag", "is", foreign.String())))}),
		http.StatusUnprocessableEntity, "invalid_segment")

	viewer, _ := b.join("viewer")
	expect(t, viewer.do("GET", segmentsBase, nil), http.StatusOK)
	expect(t, viewer.do("POST", segmentsBase, map[string]any{"name": "No", "rules": rules(group("all", cond("opt_in", "is", "in")))}),
		http.StatusForbidden, "permission_denied")
}

func TestBroadcastToSegments(t *testing.T) {
	b := newBroadcastsEnv(t)
	vip := b.tag("VIP")
	b.contact("Grace", "+447700900061", "in", vip)
	b.contact("Alan", "+15550100062", "in")
	b.contact("Pat", "+15550100063", "pending")
	b.contact("Ada", "+33600000064", "in")

	us := b.owner.do("POST", segmentsBase, map[string]any{"name": "US", "rules": rules(group("all", cond("country_code", "starts_with", "+1")))})
	expect(t, us, http.StatusCreated)
	usID := us.body["id"].(string)

	// The US segment and the VIP tag, counted once each, opted-in only.
	preview := b.owner.do("POST", broadcastsBase+"/audience-preview", map[string]any{"tag_ids": []string{vip.String()}, "segment_ids": []string{usID}})
	expect(t, preview, http.StatusOK)
	if preview.body["included"].(float64) != 2 || preview.body["left_out"].(float64) != 1 {
		t.Fatalf("preview: %s", preview.raw)
	}
	res := b.owner.do("POST", broadcastsBase, map[string]any{
		"name": "US and VIP", "number_id": b.numberID, "template_id": b.templateID,
		"audience":  map[string]any{"tag_ids": []string{vip.String()}, "segment_ids": []string{usID}},
		"variables": map[string]any{"body": []map[string]any{{"source": "text", "value": "Hi"}, {"source": "text", "value": "X"}}},
	})
	expect(t, res, http.StatusCreated)
	id := res.body["id"].(string)
	expect(t, b.owner.do("POST", broadcastsBase+"/"+id+"/send", map[string]any{}), http.StatusOK)
	b.runBroadcasts()
	if n := b.count(`SELECT count(*) FROM broadcast_recipients WHERE broadcast_id = $1`, id); n != 2 {
		t.Fatalf("recipients = %d, want 2", n)
	}

	// A scheduled broadcast whose segment is deleted fails when it starts, saying why.
	later := b.owner.do("POST", broadcastsBase, map[string]any{
		"name": "Later", "number_id": b.numberID, "template_id": b.templateID,
		"audience":  map[string]any{"segment_ids": []string{usID}},
		"variables": map[string]any{"body": []map[string]any{{"source": "text", "value": "Hi"}, {"source": "text", "value": "X"}}},
	})
	expect(t, later, http.StatusCreated)
	laterID := later.body["id"].(string)
	expect(t, b.owner.do("POST", broadcastsBase+"/"+laterID+"/send", map[string]any{}), http.StatusOK)
	expect(t, b.owner.do("DELETE", segmentsBase+"/"+usID, nil), http.StatusNoContent)
	if err := b.e.a.broadcasts.Start(context.Background(), b.wsID, uuid.MustParse(laterID)); err != nil {
		t.Fatal(err)
	}
	got := b.get(laterID)
	if got["status"] != "failed" || got["failure_reason"] != "segment_deleted" {
		t.Fatalf("after delete: %v", got)
	}
	expect(t, b.owner.do("POST", broadcastsBase+"/audience-preview", map[string]any{"segment_ids": []string{usID}}),
		http.StatusUnprocessableEntity, "broadcast_segment_not_found")
}
