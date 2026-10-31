package app

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	broadcastsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/broadcasts/model"
)

const broadcastsBase = "/v1/workspace/broadcasts"

type broadcastsEnv struct {
	*inboxEnv
	templateID uuid.UUID
}

// newBroadcastsEnv is an inbox workspace with an approved two-variable template
// and a plan of two broadcasts a month.
func newBroadcastsEnv(t *testing.T) *broadcastsEnv {
	t.Helper()
	v := newInboxEnv(t)
	v.e.exec(`UPDATE plans SET limits = limits || '{"broadcasts": 2}'`)
	t.Cleanup(func() { v.e.exec(`UPDATE plans SET limits = limits - 'broadcasts'`) })
	clearJobs(v)
	t.Cleanup(func() {
		v.e.exec(`UPDATE broadcasts SET status = 'cancelled' WHERE workspace_id = $1 AND status IN ('scheduled', 'sending')`, v.wsID)
	})
	b := &broadcastsEnv{inboxEnv: v, templateID: uuid.Must(uuid.NewV7())}
	v.e.exec(`INSERT INTO message_templates (id, workspace_id, name, language, category, status, components)
		VALUES ($1, $2, 'spring_sale', 'en_US', 'marketing', 'approved', $3)`, b.templateID, v.wsID,
		`{"header":{"type":"none"},"body":{"text":"Hi {{1}}, use {{2}} today.","examples":["a","b"]},"footer":{"text":""},"buttons":[]}`)
	return b
}

func (b *broadcastsEnv) contact(name, phone, optIn string, tags ...uuid.UUID) uuid.UUID {
	b.e.t.Helper()
	id := uuid.Must(uuid.NewV7())
	b.e.exec(`INSERT INTO contacts (id, workspace_id, name, phone, source, opt_in, attributes) VALUES ($1, $2, $3, $4, 'manual', $5, '{"city":"Dhaka"}')`,
		id, b.wsID, name, phone, optIn)
	for _, tag := range tags {
		b.e.exec(`INSERT INTO contact_tag_links (workspace_id, contact_id, tag_id) VALUES ($1, $2, $3)`, b.wsID, id, tag)
	}
	return id
}

func (b *broadcastsEnv) tag(name string) uuid.UUID {
	b.e.t.Helper()
	id := uuid.Must(uuid.NewV7())
	b.e.exec(`INSERT INTO contact_tags (id, workspace_id, name) VALUES ($1, $2, $3)`, id, b.wsID, name)
	return id
}

// draft saves a ready broadcast to audience tags and returns its ID.
func (b *broadcastsEnv) draft(name string, tags ...uuid.UUID) string {
	b.e.t.Helper()
	audience := map[string]any{"all": true}
	if tags != nil {
		audience = map[string]any{"tag_ids": tags}
	}
	res := b.owner.do("POST", broadcastsBase, map[string]any{
		"name": name, "number_id": b.numberID, "template_id": b.templateID,
		"audience": audience,
		"variables": map[string]any{"body": []map[string]any{
			{"source": "name", "value": "", "fallback": "there"},
			{"source": "text", "value": "SAVE10"},
		}},
	})
	expect(b.e.t, res, http.StatusCreated)
	return res.body["id"].(string)
}

// runBroadcasts works queued broadcast jobs until none is left, as River would.
func (b *broadcastsEnv) runBroadcasts() {
	b.e.t.Helper()
	ctx := context.Background()
	for range 20 {
		rows, err := b.e.a.db.Query(ctx, `UPDATE river_job SET state = 'completed', finalized_at = now()
			WHERE kind IN ('broadcasts_start', 'broadcasts_batch') AND state = 'available' AND args->>'workspace_id' = $1
			RETURNING kind, args`, b.wsID.String())
		if err != nil {
			b.e.t.Fatal(err)
		}
		type job struct {
			kind string
			raw  []byte
		}
		var jobs []job
		for rows.Next() {
			var j job
			if err := rows.Scan(&j.kind, &j.raw); err != nil {
				b.e.t.Fatal(err)
			}
			jobs = append(jobs, j)
		}
		rows.Close()
		if len(jobs) == 0 {
			return
		}
		for _, j := range jobs {
			var err error
			if j.kind == "broadcasts_start" {
				var a broadcastsmodel.StartArgs
				_ = json.Unmarshal(j.raw, &a)
				err = b.e.a.broadcasts.Start(ctx, a.WorkspaceID, a.BroadcastID)
			} else {
				var a broadcastsmodel.BatchArgs
				_ = json.Unmarshal(j.raw, &a)
				err = b.e.a.broadcasts.Batch(ctx, a.WorkspaceID, a.BroadcastID, a.Seq)
			}
			if err != nil {
				b.e.t.Fatal(err)
			}
		}
	}
}

func (b *broadcastsEnv) get(id string) map[string]any {
	b.e.t.Helper()
	res := b.owner.do("GET", broadcastsBase+"/"+id, nil)
	expect(b.e.t, res, http.StatusOK)
	return res.body
}

func (b *broadcastsEnv) usedThisMonth() float64 {
	b.e.t.Helper()
	res := b.owner.do("GET", "/v1/workspace/billing/usage", nil)
	expect(b.e.t, res, http.StatusOK)
	for _, r := range res.body["resources"].([]any) {
		if m := r.(map[string]any); m["resource"] == "broadcasts" {
			return m["used"].(float64)
		}
	}
	b.e.t.Fatalf("usage: %s", res.raw)
	return 0
}

func stat(b map[string]any, key string) float64 { return b["stats"].(map[string]any)[key].(float64) }

func TestBroadcastSendsOnceToOptedInContacts(t *testing.T) {
	b := newBroadcastsEnv(t)
	grace := b.contact("Grace", "+447700900001", "in")
	b.contact("", "+447700900002", "in")
	b.contact("Alan", "+447700900003", "in")
	b.contact("Pending Pat", "+447700900004", "pending")
	b.contact("Out Olu", "+447700900005", "out")

	preview := b.owner.do("POST", broadcastsBase+"/audience-preview", map[string]any{"all": true, "number_id": b.numberID})
	expect(t, preview, http.StatusOK)
	if preview.body["included"].(float64) != 3 || preview.body["left_out"].(float64) != 2 {
		t.Fatalf("preview: %s", preview.raw)
	}

	id := b.draft("Spring sale")
	sent := b.owner.do("POST", broadcastsBase+"/"+id+"/send", map[string]any{})
	expect(t, sent, http.StatusOK)
	if sent.body["status"] != "sending" {
		t.Fatalf("send: %s", sent.raw)
	}
	b.runBroadcasts()
	if got := b.get(id); got["recipients_total"].(float64) != 3 || stat(got, "queued") != 3 {
		t.Fatalf("after batches: %v", got)
	}

	// A retried batch, and a second start, send nobody twice.
	ctx := context.Background()
	if err := b.e.a.broadcasts.Batch(ctx, b.wsID, uuid.MustParse(id), 0); err != nil {
		t.Fatal(err)
	}
	if err := b.e.a.broadcasts.Start(ctx, b.wsID, uuid.MustParse(id)); err != nil {
		t.Fatal(err)
	}
	if n := b.count(`SELECT count(*) FROM broadcast_recipients WHERE broadcast_id = $1 AND message_id IS NOT NULL`, id); n != 3 {
		t.Fatalf("messages = %d, want 3", n)
	}
	if n := b.count(`SELECT count(*) FROM river_job WHERE kind = 'inbox_send' AND queue = 'bulk' AND state = 'available' AND args->>'workspace_id' = $1`, b.wsID.String()); n != 3 {
		t.Fatalf("bulk sends queued = %d, want 3", n)
	}
	if b.runSends(true) != 3 {
		t.Fatal("not delivered")
	}
	if !slices.Contains(b.channel.sent, "template:+447700900001:spring_sale:Grace,SAVE10") ||
		!slices.Contains(b.channel.sent, "template:+447700900002:spring_sale:there,SAVE10") {
		t.Fatalf("sent: %v", b.channel.sent)
	}
	if n := b.count(`SELECT count(*) FROM conversations WHERE workspace_id = $1 AND source = 'broadcast'`, b.wsID); n != 3 {
		t.Fatalf("broadcast chats = %d", n)
	}

	// Grace reads it and replies; the tick then completes the broadcast.
	var wamid string
	if err := b.e.a.db.QueryRow(ctx, `SELECT m.external_id FROM broadcast_recipients r JOIN messages m ON m.id = r.message_id
		WHERE r.broadcast_id = $1 AND r.contact_id = $2`, id, grace).Scan(&wamid); err != nil {
		t.Fatal(err)
	}
	b.status(wamid, "read")
	b.inbound("wamid.reply.1", "447700900001", "Thanks!")
	if err := b.e.a.broadcasts.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	got := b.get(id)
	if got["status"] != "completed" || stat(got, "sent") != 3 || stat(got, "delivered") != 1 || stat(got, "read") != 1 || stat(got, "replied") != 1 {
		t.Fatalf("report: %v", got)
	}
	if b.usedThisMonth() != 1 {
		t.Fatal("a sent broadcast didn't use a plan unit")
	}
	if n := b.count(`SELECT count(*) FROM workspaces WHERE id = $1 AND activated_at IS NOT NULL`, b.wsID); n != 1 {
		t.Fatal("a broadcast that reached a customer didn't activate the workspace")
	}

	list := b.owner.do("GET", broadcastsBase, nil)
	expect(t, list, http.StatusOK)
	if rows := items(list); len(rows) != 1 || rows[0]["sent"].(float64) != 3 || rows[0]["template"].(map[string]any)["name"] != "spring_sale" {
		t.Fatalf("list: %s", list.raw)
	}
	read := b.owner.do("GET", broadcastsBase+"/"+id+"/recipients?filter=read", nil)
	expect(t, read, http.StatusOK)
	if rows := items(read); len(rows) != 1 || rows[0]["name"] != "Grace" || rows[0]["replied"] != true {
		t.Fatalf("read recipients: %s", read.raw)
	}
	expect(t, b.owner.do("PUT", broadcastsBase+"/"+id, map[string]any{"name": "Too late"}), http.StatusConflict, "broadcast_not_editable")
}

func TestBroadcastTagsAndOptOutDuringSending(t *testing.T) {
	b := newBroadcastsEnv(t)
	vip, other := b.tag("vip"), b.tag("wholesale")
	b.contact("Grace", "+447700900011", "in", vip)
	late := b.contact("Alan", "+447700900012", "in", vip, other)
	b.contact("Ada", "+447700900013", "in", other)
	b.contact("Not tagged", "+447700900014", "in")

	id := b.draft("VIP only", vip)
	expect(t, b.owner.do("POST", broadcastsBase+"/"+id+"/send", map[string]any{}), http.StatusOK)
	// Take the audience, then Alan opts out before his batch runs.
	ctx := context.Background()
	if err := b.e.a.broadcasts.Start(ctx, b.wsID, uuid.MustParse(id)); err != nil {
		t.Fatal(err)
	}
	b.e.exec(`UPDATE contacts SET opt_in = 'out' WHERE id = $1`, late)
	b.runBroadcasts()
	b.runSends(true)
	if err := b.e.a.broadcasts.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	got := b.get(id)
	if got["recipients_total"].(float64) != 2 || stat(got, "sent") != 1 || stat(got, "skipped") != 1 || got["status"] != "completed" {
		t.Fatalf("report: %v", got)
	}
	failures := got["failures"].([]any)
	if len(failures) != 1 || failures[0].(map[string]any)["reason"] != "opted_out" {
		t.Fatalf("failures: %v", failures)
	}
}

func TestBroadcastPlanLimitScheduleAndCancel(t *testing.T) {
	b := newBroadcastsEnv(t)
	b.contact("Grace", "+447700900021", "in")
	first, second, third := b.draft("One"), b.draft("Two"), b.draft("Three")

	expect(t, b.owner.do("POST", broadcastsBase+"/"+first+"/send", map[string]any{}), http.StatusOK)
	at := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	scheduled := b.owner.do("POST", broadcastsBase+"/"+second+"/send", map[string]any{"scheduled_at": at})
	expect(t, scheduled, http.StatusOK)
	if scheduled.body["status"] != "scheduled" {
		t.Fatalf("schedule: %s", scheduled.raw)
	}
	expect(t, b.owner.do("POST", broadcastsBase+"/"+third+"/send", map[string]any{}), http.StatusConflict, "plan_broadcast_limit")
	if b.usedThisMonth() != 2 {
		t.Fatal("used != 2")
	}

	// A scheduled broadcast can still change, and the tick doesn't start it early.
	edit := b.owner.do("PUT", broadcastsBase+"/"+second, map[string]any{
		"name": "Two (edited)", "number_id": b.numberID, "template_id": b.templateID, "audience": map[string]any{"all": true},
		"variables": map[string]any{"body": []map[string]any{{"source": "text", "value": "Hi"}, {"source": "text", "value": "X"}}},
	})
	expect(t, edit, http.StatusOK)
	if err := b.e.a.broadcasts.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if b.get(second)["status"] != "scheduled" {
		t.Fatal("started before its time")
	}

	// Cancelling it before anything was queued gives the unit back.
	cancelled := b.owner.do("POST", broadcastsBase+"/"+second+"/cancel", nil)
	expect(t, cancelled, http.StatusOK)
	if cancelled.body["status"] != "cancelled" || b.usedThisMonth() != 1 {
		t.Fatalf("cancel: %s", cancelled.raw)
	}
	expect(t, b.owner.do("POST", broadcastsBase+"/"+second+"/cancel", nil), http.StatusConflict, "broadcast_not_cancellable")
	expect(t, b.owner.do("POST", broadcastsBase+"/"+third+"/send", map[string]any{}), http.StatusOK)

	// A start job for the cancelled one does nothing.
	if err := b.e.a.broadcasts.Start(context.Background(), b.wsID, uuid.MustParse(second)); err != nil {
		t.Fatal(err)
	}
	if n := b.count(`SELECT count(*) FROM broadcast_recipients WHERE broadcast_id = $1`, second); n != 0 {
		t.Fatalf("cancelled broadcast took %d recipients", n)
	}
}

func TestBroadcastNeedsEverythingBeforeSending(t *testing.T) {
	b := newBroadcastsEnv(t)
	empty := b.owner.do("POST", broadcastsBase, map[string]any{"name": "Draft", "audience": map[string]any{"tag_ids": []string{}}, "variables": map[string]any{"body": []any{}}})
	expect(t, empty, http.StatusCreated)
	notReady := b.owner.do("POST", broadcastsBase+"/"+empty.body["id"].(string)+"/send", map[string]any{})
	expect(t, notReady, http.StatusUnprocessableEntity, "broadcast_not_ready")
	if !strings.Contains(notReady.raw, "number_id") || !strings.Contains(notReady.raw, "template_id") {
		t.Fatalf("details: %s", notReady.raw)
	}

	// A name variable needs a fallback; nobody opted in is an empty audience.
	res := b.owner.do("POST", broadcastsBase, map[string]any{
		"name": "No fallback", "number_id": b.numberID, "template_id": b.templateID, "audience": map[string]any{"all": true},
		"variables": map[string]any{"body": []map[string]any{{"source": "name", "value": ""}, {"source": "text", "value": "X"}}},
	})
	expect(t, res, http.StatusCreated)
	noFallback := b.owner.do("POST", broadcastsBase+"/"+res.body["id"].(string)+"/send", map[string]any{})
	expect(t, noFallback, http.StatusUnprocessableEntity, "broadcast_not_ready")
	if !strings.Contains(noFallback.raw, "variables.body[0].fallback") {
		t.Fatalf("details: %s", noFallback.raw)
	}
	expect(t, b.owner.do("POST", broadcastsBase+"/"+b.draft("Nobody")+"/send", map[string]any{}), http.StatusUnprocessableEntity, "broadcast_empty_audience")
	if b.usedThisMonth() != 0 {
		t.Fatal("a refused send used a plan unit")
	}
	expect(t, b.owner.do("POST", broadcastsBase+"/"+b.draft("Past")+"/send", map[string]any{"scheduled_at": time.Now().Add(-time.Hour)}),
		http.StatusUnprocessableEntity, "invalid_broadcast")
}

func TestBroadcastPermissions(t *testing.T) {
	b := newBroadcastsEnv(t)
	id := b.draft("Team view")
	agent, _ := b.join("agent")
	expect(t, agent.do("GET", broadcastsBase, nil), http.StatusOK)
	expect(t, agent.do("GET", broadcastsBase+"/"+id, nil), http.StatusOK)
	expect(t, agent.do("POST", broadcastsBase, map[string]any{"name": "x"}), http.StatusForbidden, "permission_denied")
	expect(t, agent.do("POST", broadcastsBase+"/"+id+"/send", map[string]any{}), http.StatusForbidden, "permission_denied")

	dup := b.owner.do("POST", broadcastsBase+"/"+id+"/duplicate", nil)
	expect(t, dup, http.StatusCreated)
	if dup.body["name"] != "Team view (copy)" || dup.body["status"] != "draft" {
		t.Fatalf("duplicate: %s", dup.raw)
	}
	expect(t, b.owner.do("DELETE", broadcastsBase+"/"+dup.body["id"].(string), nil), http.StatusNoContent)

	other := newTeamWorkspace(b.e)
	expect(t, other.owner.do("GET", broadcastsBase+"/"+id, nil), http.StatusNotFound, "broadcast_not_found")
}

func TestBroadcastSkipsRecentMarketing(t *testing.T) {
	b := newBroadcastsEnv(t)
	recent := b.contact("Recent Rita", "+447700900031", "in")
	b.contact("Fresh Fay", "+447700900032", "in")
	old := b.contact("Old Otto", "+447700900033", "in")
	// Rita got a marketing template an hour ago; Otto two days ago.
	for _, m := range []struct {
		contact uuid.UUID
		ago     string
	}{{recent, "1 hour"}, {old, "2 days"}} {
		conv := uuid.Must(uuid.NewV7())
		b.e.exec(`INSERT INTO conversations (id, workspace_id, contact_id, channel_type, channel_id, source) VALUES ($1, $2, $3, 'whatsapp', $4, 'template')`,
			conv, b.wsID, m.contact, b.numberID)
		b.e.exec(`INSERT INTO messages (id, workspace_id, conversation_id, direction, type, content, status, created_at)
			VALUES ($1, $2, $3, 'out', 'template', '{"template":{"category":"marketing"}}', 'read', now() - $4::interval)`,
			uuid.Must(uuid.NewV7()), b.wsID, conv, m.ago)
	}

	preview := b.owner.do("POST", broadcastsBase+"/audience-preview", map[string]any{"all": true, "skip_recent_marketing": true})
	expect(t, preview, http.StatusOK)
	if preview.body["included"].(float64) != 2 || preview.body["recent"].(float64) != 1 {
		t.Fatalf("preview: %s", preview.raw)
	}
	res := b.owner.do("POST", broadcastsBase, map[string]any{
		"name": "Gentle", "number_id": b.numberID, "template_id": b.templateID,
		"audience":  map[string]any{"all": true, "skip_recent_marketing": true},
		"variables": map[string]any{"body": []map[string]any{{"source": "text", "value": "Hi"}, {"source": "text", "value": "X"}}},
	})
	expect(t, res, http.StatusCreated)
	if res.body["audience"].(map[string]any)["skip_recent_marketing"] != true {
		t.Fatalf("saved audience: %s", res.raw)
	}
	id := res.body["id"].(string)
	expect(t, b.owner.do("POST", broadcastsBase+"/"+id+"/send", map[string]any{}), http.StatusOK)
	b.runBroadcasts()
	if n := b.count(`SELECT count(*) FROM broadcast_recipients WHERE broadcast_id = $1`, id); n != 2 {
		t.Fatalf("recipients = %d, want 2", n)
	}
	if n := b.count(`SELECT count(*) FROM broadcast_recipients WHERE broadcast_id = $1 AND contact_id = $2`, id, recent); n != 0 {
		t.Fatal("a contact sent marketing an hour ago got another")
	}

	// A draft with no source reaches nobody.
	none := b.owner.do("POST", broadcastsBase+"/audience-preview", map[string]any{})
	expect(t, none, http.StatusOK)
	if none.body["included"].(float64) != 0 {
		t.Fatalf("empty audience: %s", none.raw)
	}
}
