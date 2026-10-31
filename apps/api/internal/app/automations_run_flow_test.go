package app

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	automationsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
)

// runAutomations works queued automation jobs until none is left, as River would.
func (v *inboxEnv) runAutomations() {
	v.e.t.Helper()
	ctx := context.Background()
	for range 5 {
		rows, err := v.e.a.db.Query(ctx, `UPDATE river_job SET state = 'completed', finalized_at = now()
			WHERE kind IN ('automations_inbound', 'automations_process_run') AND state = 'available' RETURNING kind, args`)
		if err != nil {
			v.e.t.Fatal(err)
		}
		type job struct {
			kind string
			raw  []byte
		}
		var jobs []job
		for rows.Next() {
			var j job
			if err := rows.Scan(&j.kind, &j.raw); err != nil {
				v.e.t.Fatal(err)
			}
			jobs = append(jobs, j)
		}
		rows.Close()
		if len(jobs) == 0 {
			return
		}
		for _, j := range jobs {
			var err error
			if j.kind == "automations_inbound" {
				var a automationsmodel.InboundArgs
				_ = json.Unmarshal(j.raw, &a)
				err = v.e.a.automations.HandleInbound(ctx, a.Inbound)
			} else {
				var a automationsmodel.ProcessRunArgs
				_ = json.Unmarshal(j.raw, &a)
				err = v.e.a.automations.ProcessRun(ctx, a.WorkspaceID, a.RunID)
			}
			if err != nil {
				v.e.t.Fatal(err)
			}
		}
	}
}

func (v *inboxEnv) count(query string, args ...any) int {
	v.e.t.Helper()
	var n int
	if err := v.e.a.db.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		v.e.t.Fatal(err)
	}
	return n
}

func (v *inboxEnv) runs(state, reason string) int {
	v.e.t.Helper()
	return v.count(`SELECT count(*) FROM automation_runs WHERE workspace_id = $1 AND state = $2 AND coalesce(reason, '') = $3`, v.wsID, state, reason)
}

func (v *inboxEnv) automated() int {
	v.e.t.Helper()
	return v.count(`SELECT count(*) FROM messages WHERE workspace_id = $1 AND automation_run_id IS NOT NULL`, v.wsID)
}

func (v *inboxEnv) turnOn(kind string, settings map[string]any) {
	v.e.t.Helper()
	base := "/v1/workspace/automations/" + kind
	expect(v.e.t, v.owner.do("PUT", base, map[string]any{kind: settings}), http.StatusOK)
	expect(v.e.t, v.owner.do("PUT", base+"/enabled", map[string]any{"enabled": true}), http.StatusOK)
}

func newAutomationsEnv(t *testing.T) *inboxEnv {
	t.Helper()
	v := newInboxEnv(t)
	includeAutomations(v.e)
	clearJobs(v)
	return v
}

// clearJobs finishes this workspace's jobs after the test, because the queue
// table is shared with the other flow tests.
func clearJobs(v *inboxEnv) {
	v.e.t.Cleanup(func() {
		v.e.exec(`UPDATE river_job SET state = 'completed', finalized_at = now()
			WHERE state = 'available' AND args->>'workspace_id' = $1`, v.wsID.String())
	})
}

var welcome = map[string]any{"message": "Hi {{name}}!", "delay_minutes": 0, "first_time_only": true}

func TestWelcomeSendsOnceAndKeepsTheChatWaiting(t *testing.T) {
	v := newAutomationsEnv(t)
	v.turnOn("welcome", welcome)
	v.inbound("wamid.1", "15551230001", "hello")
	v.runAutomations()
	if v.automated() != 1 || v.runs(automationsmodel.StateDone, "") != 1 {
		t.Fatalf("automated = %d", v.automated())
	}
	if n := v.count(`SELECT count(*) FROM messages WHERE workspace_id = $1 AND body = 'Hi Grace!' AND author_id IS NULL`, v.wsID); n != 1 {
		t.Fatal("reply not personalized")
	}
	if v.count(`SELECT count(*) FROM conversations WHERE workspace_id = $1 AND waiting_since IS NOT NULL`, v.wsID) != 1 {
		t.Fatal("an automated reply answered the customer")
	}
	if v.runSends(true) != 1 {
		t.Fatal("not delivered")
	}

	// A redelivered webhook and a second message change nothing.
	v.inbound("wamid.1", "15551230001", "hello")
	v.inbound("wamid.2", "15551230001", "hello again")
	v.runAutomations()
	if v.automated() != 1 {
		t.Fatalf("automated = %d", v.automated())
	}
	list := v.owner.do("GET", "/v1/workspace/automations", nil)
	if items(list)[0]["sent"].(float64) != 1 {
		t.Fatalf("stat: %s", list.raw)
	}
}

func TestProcessRunTwiceSendsOnce(t *testing.T) {
	v := newAutomationsEnv(t)
	v.turnOn("welcome", welcome)
	v.inbound("wamid.1", "15551230001", "hello")
	rows, err := v.e.a.db.Query(context.Background(), `UPDATE river_job SET state = 'completed', finalized_at = now()
		WHERE kind = 'automations_inbound' RETURNING args`)
	if err != nil {
		t.Fatal(err)
	}
	var a automationsmodel.InboundArgs
	for rows.Next() {
		var raw []byte
		_ = rows.Scan(&raw)
		_ = json.Unmarshal(raw, &a)
	}
	rows.Close()
	if err := v.e.a.automations.HandleInbound(context.Background(), a.Inbound); err != nil {
		t.Fatal(err)
	}
	var run uuid.UUID
	if err := v.e.a.db.QueryRow(context.Background(), `SELECT id FROM automation_runs WHERE workspace_id = $1`, v.wsID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := v.e.a.automations.ProcessRun(context.Background(), v.wsID, run); err != nil {
			t.Fatal(err)
		}
	}
	if v.automated() != 1 {
		t.Fatalf("automated = %d", v.automated())
	}
}

func TestOneReplyPerMessage(t *testing.T) {
	v := newAutomationsEnv(t)
	v.turnOn("welcome", welcome)
	v.turnOn("keyword", map[string]any{"rules": []map[string]any{{"keywords": "price", "match": "any", "reply": "See the catalog"}}})
	v.inbound("wamid.1", "15551230001", "price?")
	v.runAutomations()
	if v.automated() != 1 || v.runs(automationsmodel.StateSkipped, automationsmodel.ReasonLowerPriority) != 1 {
		t.Fatalf("automated = %d", v.automated())
	}
	if v.count(`SELECT count(*) FROM messages WHERE workspace_id = $1 AND body = 'See the catalog'`, v.wsID) != 1 {
		t.Fatal("keyword should win")
	}
}

func TestAgentReplySilencesAutomations(t *testing.T) {
	v := newAutomationsEnv(t)
	v.inbound("wamid.1", "15551230001", "hello")
	conv := v.only(t, v.owner, "")["id"].(string)
	expect(v.e.t, v.owner.do("POST", "/v1/workspace/inbox/conversations/"+conv+"/messages", map[string]any{"type": "text", "text": "Hi, I'm here"}), http.StatusCreated)
	v.turnOn("keyword", map[string]any{"rules": []map[string]any{{"keywords": "price", "match": "any", "reply": "See the catalog"}}})
	v.inbound("wamid.2", "15551230001", "price?")
	v.runAutomations()
	if v.automated() != 0 || v.runs(automationsmodel.StateSkipped, automationsmodel.ReasonAgentActive) != 1 {
		t.Fatalf("automated = %d", v.automated())
	}
}

func TestDelayedWelcomeSkipsIfAnAgentAnswersFirst(t *testing.T) {
	v := newAutomationsEnv(t)
	delayed := map[string]any{"message": "Hi", "delay_minutes": 5, "first_time_only": true}
	v.turnOn("welcome", delayed)
	v.inbound("wamid.1", "15551230001", "hello")
	v.runAutomations()
	if v.runs(automationsmodel.StateScheduled, "") != 1 || v.automated() != 0 {
		t.Fatal("should wait")
	}
	conv := v.only(t, v.owner, "")["id"].(string)
	expect(v.e.t, v.owner.do("POST", "/v1/workspace/inbox/conversations/"+conv+"/messages", map[string]any{"type": "text", "text": "Hi"}), http.StatusCreated)
	v.e.exec(`UPDATE automation_runs SET next_at = now() - interval '1 minute' WHERE workspace_id = $1`, v.wsID)
	if err := v.e.a.automations.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	v.runAutomations()
	if v.automated() != 0 || v.runs(automationsmodel.StateSkipped, automationsmodel.ReasonAgentActive) != 1 {
		t.Fatalf("automated = %d", v.automated())
	}
}

func TestStopOptsOutEvenWithEverythingOff(t *testing.T) {
	v := newInboxEnv(t) // No automations on the plan either.
	clearJobs(v)
	v.inbound("wamid.1", "15551230001", "hello")
	v.inbound("wamid.2", "15551230001", "stop")
	v.runAutomations()
	if v.count(`SELECT count(*) FROM contacts WHERE workspace_id = $1 AND opt_in = 'out' AND opt_in_source = 'whatsapp_reply'`, v.wsID) != 1 {
		t.Fatal("not opted out")
	}
	if v.count(`SELECT count(*) FROM messages WHERE workspace_id = $1 AND content->'event'->>'action' = 'opted_out'`, v.wsID) != 1 {
		t.Fatal("no event in the thread")
	}
	if v.automated() != 0 {
		t.Fatal("STOP must not get a reply")
	}
	includeAutomations(v.e)
	v.turnOn("keyword", map[string]any{"rules": []map[string]any{{"keywords": "price", "match": "any", "reply": "x"}}})
	v.inbound("wamid.3", "15551230001", "price")
	v.runAutomations()
	if v.automated() != 0 || v.runs(automationsmodel.StateSkipped, automationsmodel.ReasonOptedOut) != 1 {
		t.Fatalf("automated = %d", v.automated())
	}
}

func TestClosedWindowSkipsTheRun(t *testing.T) {
	v := newAutomationsEnv(t)
	v.turnOn("welcome", map[string]any{"message": "Hi", "delay_minutes": 5, "first_time_only": true})
	v.inbound("wamid.1", "15551230001", "hello")
	v.runAutomations()
	v.e.exec(`UPDATE conversations SET window_expires_at = $1 WHERE workspace_id = $2`, time.Now().Add(-time.Hour), v.wsID)
	v.e.exec(`UPDATE automation_runs SET next_at = now() - interval '1 minute' WHERE workspace_id = $1`, v.wsID)
	if err := v.e.a.automations.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	v.runAutomations()
	if v.runs(automationsmodel.StateSkipped, automationsmodel.ReasonWindowClosed) != 1 {
		t.Fatal("expected window_closed")
	}
}
