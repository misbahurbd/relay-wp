package app

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	automationsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
)

// dripEnv is a store workspace with a tag and a two-step sequence on it.
type dripEnv struct {
	*storeEnv
	tag string
}

func newDripEnv(t *testing.T, stopOnReply bool) *dripEnv {
	t.Helper()
	s := newStoreEnv(t)
	s.e.exec(`UPDATE message_templates SET components = $1 WHERE id = $2`,
		`{"header":{"type":"none"},"body":{"text":"Hi {{1}}, welcome","examples":["a"]},"footer":{"text":""},"buttons":[],"auth":{}}`, s.template)
	tag := s.owner.do("POST", "/v1/workspace/contact-tags", map[string]any{"name": "New subscriber"})
	expect(t, tag, http.StatusCreated)
	d := &dripEnv{storeEnv: s, tag: tag.body["id"].(string)}
	d.turnOn("drip", map[string]any{
		"tag_id": d.tag, "stop_on_reply": stopOnReply,
		"steps": []map[string]any{{"day": 0, "template_id": s.template.String()}, {"day": 2, "template_id": s.template.String()}},
	})
	return d
}

func (d *dripEnv) contact(phone, optIn string) uuid.UUID {
	d.e.t.Helper()
	id := uuid.Must(uuid.NewV7())
	d.e.exec(`INSERT INTO contacts (id, workspace_id, name, phone, source, opt_in) VALUES ($1, $2, 'Ayesha Rahman', $3, 'inbox', $4)`, id, d.wsID, phone, optIn)
	return id
}

func (d *dripEnv) tagContacts(ids ...uuid.UUID) resp {
	d.e.t.Helper()
	return d.owner.do("POST", "/v1/workspace/contacts/bulk", map[string]any{"action": "add_tag", "contact_ids": ids, "tag_id": d.tag})
}

func TestDripRunsItsStepsThenEnds(t *testing.T) {
	d := newDripEnv(t, true)
	c := d.contact("+13125550111", "in")
	expect(t, d.tagContacts(c), http.StatusOK)
	d.due()
	if d.automated() != 1 || d.runs(automationsmodel.StateScheduled, "") != 1 {
		t.Fatalf("step 1: automated = %d", d.automated())
	}
	if d.count(`SELECT count(*) FROM messages WHERE workspace_id = $1 AND body = 'Hi Ayesha, welcome'`, d.wsID) != 1 {
		t.Fatal("first name not filled")
	}
	gap := d.count(`SELECT round(extract(epoch FROM next_at - now()) / 86400)::int FROM automation_runs WHERE workspace_id = $1`, d.wsID)
	if gap != 2 {
		t.Fatalf("next step in %d days", gap)
	}
	card := items(d.owner.do("GET", "/v1/workspace/automations", nil))[5]
	if card["active"].(float64) != 1 {
		t.Fatalf("card: %v", card)
	}
	d.due()
	d.due()
	if d.automated() != 2 || d.runs(automationsmodel.StateDone, "") != 1 {
		t.Fatalf("end: automated = %d", d.automated())
	}
}

func TestDripEnrolsOnce(t *testing.T) {
	d := newDripEnv(t, true)
	c := d.contact("+13125550111", "in")
	d.tagContacts(c)
	expect(t, d.owner.do("POST", "/v1/workspace/contacts/bulk", map[string]any{"action": "remove_tag", "contact_ids": []uuid.UUID{c}, "tag_id": d.tag}), http.StatusOK)
	d.tagContacts(c)
	if d.count(`SELECT count(*) FROM automation_runs WHERE workspace_id = $1`, d.wsID) != 1 {
		t.Fatal("enrolled twice")
	}
}

func TestDripLeavesContactsTaggedBeforeItWasOn(t *testing.T) {
	s := newStoreEnv(t)
	tag := s.owner.do("POST", "/v1/workspace/contact-tags", map[string]any{"name": "Earlier"})
	d := &dripEnv{storeEnv: s, tag: tag.body["id"].(string)}
	d.tagContacts(d.contact("+13125550111", "in"))
	d.turnOn("drip", map[string]any{"tag_id": d.tag, "stop_on_reply": true, "steps": []map[string]any{{"day": 0, "template_id": s.template.String()}}})
	if d.count(`SELECT count(*) FROM automation_runs WHERE workspace_id = $1`, d.wsID) != 0 {
		t.Fatal("swept in an earlier contact")
	}
}

func TestDripConsent(t *testing.T) {
	d := newDripEnv(t, true)
	d.tagContacts(d.contact("+13125550111", "pending"))
	d.due()
	if d.automated() != 0 || d.runs(automationsmodel.StateSkipped, automationsmodel.ReasonNoConsent) != 1 {
		t.Fatalf("automated = %d", d.automated())
	}
}

func TestDripStopsOnReply(t *testing.T) {
	for _, stop := range []bool{true, false} {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			d := newDripEnv(t, stop)
			d.inbound("wamid.1", "15551230001", "hello")
			var contact uuid.UUID
			if err := d.e.a.db.QueryRow(context.Background(), `SELECT id FROM contacts WHERE workspace_id = $1`, d.wsID).Scan(&contact); err != nil {
				t.Fatal(err)
			}
			d.e.exec(`UPDATE contacts SET opt_in = 'in' WHERE id = $1`, contact)
			d.runAutomations()
			d.tagContacts(contact)
			d.due()
			d.inbound("wamid.2", "15551230001", "thanks")
			d.runAutomations()
			if stop && d.runs(automationsmodel.StateCancelled, automationsmodel.ReasonReplied) != 1 {
				t.Fatal("should stop")
			}
			if !stop && d.runs(automationsmodel.StateScheduled, "") != 1 {
				t.Fatal("should keep going")
			}
		})
	}
}

func TestPausedDripHoldsAndResumesShifted(t *testing.T) {
	d := newDripEnv(t, true)
	d.tagContacts(d.contact("+13125550111", "in"))
	base := "/v1/workspace/automations/drip/enabled"
	expect(t, d.owner.do("PUT", base, map[string]any{"enabled": false}), http.StatusOK)
	d.due()
	if d.automated() != 0 || d.runs(automationsmodel.StateScheduled, "") != 1 {
		t.Fatalf("paused: automated = %d", d.automated())
	}
	// The drip was paused a day ago.
	d.e.exec(`UPDATE automations SET paused_at = $1 WHERE workspace_id = $2 AND kind = 'drip'`, time.Now().Add(-24*time.Hour), d.wsID)
	before := d.count(`SELECT extract(epoch FROM next_at)::int FROM automation_runs WHERE workspace_id = $1`, d.wsID)
	expect(t, d.owner.do("PUT", base, map[string]any{"enabled": true}), http.StatusOK)
	after := d.count(`SELECT extract(epoch FROM next_at)::int FROM automation_runs WHERE workspace_id = $1`, d.wsID)
	if diff := after - before; diff < 86300 || diff > 86500 {
		t.Fatalf("shifted %d seconds", diff)
	}
	d.due()
	if d.automated() != 1 {
		t.Fatalf("resumed: automated = %d", d.automated())
	}
}

func TestBulkTagging500(t *testing.T) {
	d := newDripEnv(t, true)
	ids := make([]uuid.UUID, 500)
	for i := range ids {
		ids[i] = d.contact(fmt.Sprintf("+1312555%04d", i), "in")
	}
	expect(t, d.tagContacts(ids...), http.StatusOK)
	if d.count(`SELECT count(*) FROM automation_runs WHERE workspace_id = $1 AND kind = 'drip'`, d.wsID) != 500 {
		t.Fatal("not all enrolled")
	}
}
