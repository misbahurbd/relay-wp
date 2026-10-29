package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	onboardingjobs "github.com/misbahurbd/relay-wp/apps/api/internal/features/onboarding/jobs"
	onboardingmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/onboarding/model"
	whatsappjobs "github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/jobs"
	whatsappmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

const (
	testAppSecret   = "test-app-secret"
	testVerifyToken = "test-verify-token"
	testWABA        = "100200300400"
	testPhoneID     = "500600700800"
	testDisplay     = "+1 415-555-0142"
)

func testWhatsAppConfig() config.WhatsApp {
	return config.WhatsApp{
		Enabled: true, AppID: "1234567890", AppSecret: testAppSecret, ESConfigID: "777",
		GraphVersion: "v24.0", WebhookVerifyToken: testVerifyToken,
		TokenEncryptionKey: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32)),
	}
}

// fakeGraph stands in for Meta's Graph API.
type fakeGraph struct {
	srv   *httptest.Server
	mu    sync.Mutex
	calls []string
	pins  []string
}

func newFakeGraph(t *testing.T) *fakeGraph {
	g := &fakeGraph{}
	g.srv = httptest.NewServer(http.HandlerFunc(g.serve))
	t.Cleanup(g.srv.Close)
	return g
}

func (g *fakeGraph) serve(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v24.0")
	g.mu.Lock()
	g.calls = append(g.calls, r.Method+" "+path)
	g.mu.Unlock()

	reply := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	metaErr := func(status, code int) {
		reply(status, map[string]any{"error": map[string]any{"code": code, "message": "fake error", "type": "OAuthException"}})
	}
	if path != "/oauth/access_token" && r.Header.Get("Authorization") != "Bearer biz-token" {
		metaErr(http.StatusUnauthorized, 190)
		return
	}
	switch path {
	case "/oauth/access_token":
		if r.URL.Query().Get("code") != "good-code" || r.URL.Query().Get("client_secret") != testAppSecret {
			metaErr(http.StatusBadRequest, 100)
			return
		}
		reply(http.StatusOK, map[string]any{"access_token": "biz-token", "token_type": "bearer"})
	case "/" + testWABA:
		reply(http.StatusOK, map[string]any{"id": testWABA, "name": "Acme", "owner_business_info": map[string]any{"id": "42"}})
	case "/" + testPhoneID:
		reply(http.StatusOK, map[string]any{
			"display_phone_number": testDisplay, "verified_name": "Acme", "name_status": "PENDING_REVIEW",
			"quality_rating": "GREEN", "messaging_limit_tier": "TIER_250",
		})
	case "/" + testPhoneID + "/register":
		var body struct{ Pin string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		g.mu.Lock()
		g.pins = append(g.pins, body.Pin)
		g.mu.Unlock()
		reply(http.StatusOK, map[string]any{"success": true})
	case "/" + testWABA + "/subscribed_apps":
		reply(http.StatusOK, map[string]any{"success": true})
	case "/" + testWABA + "/message_templates":
		reply(http.StatusOK, map[string]any{"data": []any{map[string]any{"name": "hello_world"}}})
	case "/" + testPhoneID + "/messages":
		var body struct{ To string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.HasSuffix(body.To, "0000") {
			metaErr(http.StatusBadRequest, 131026)
			return
		}
		reply(http.StatusOK, map[string]any{"messages": []any{map[string]any{"id": "wamid." + body.To}}})
	default:
		metaErr(http.StatusNotFound, 803)
	}
}

// signedIn returns a browser signed in as a new verified user.
func (e *authEnv) signedIn() *browser {
	e.t.Helper()
	b := e.browser(chromeMac)
	b.login(e.verifiedUser())
	return b
}

func (e *authEnv) runSetupJob(workspaceID, numberID string) {
	e.t.Helper()
	err := whatsappjobs.NewSetupWorker(e.a.whatsapp).Work(e.t.Context(), &river.Job[whatsappmodel.SetupArgs]{
		JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 5},
		Args:   whatsappmodel.SetupArgs{WorkspaceID: uuid.MustParse(workspaceID), NumberID: uuid.MustParse(numberID)},
	})
	if err != nil {
		e.t.Fatal(err)
	}
}

// webhook posts a signed WhatsApp webhook, then works the queued job as the worker would.
func (e *authEnv) webhook(entry map[string]any) {
	e.t.Helper()
	payload, _ := json.Marshal(map[string]any{"object": "whatsapp_business_account", "entry": []any{entry}})
	mac := hmac.New(sha256.New, []byte(testAppSecret))
	mac.Write(payload)
	req := httptest.NewRequestWithContext(e.t.Context(), "POST", "/v1/webhooks/whatsapp", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		e.t.Fatalf("webhook: %d %s", rec.Code, rec.Body)
	}
	err := whatsappjobs.NewWebhookWorker(e.a.whatsapp).Work(e.t.Context(), &river.Job[whatsappmodel.WebhookArgs]{
		JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 25}, Args: whatsappmodel.WebhookArgs{Payload: payload},
	})
	if err != nil {
		e.t.Fatal(err)
	}
}

func statusChange(field string, value map[string]any) map[string]any {
	return map[string]any{"id": testWABA, "changes": []any{map[string]any{"field": field, "value": value}}}
}

func TestOnboardingFullJourney(t *testing.T) {
	e := newAuthEnv(t)
	b := e.signedIn()

	list := b.do("GET", "/v1/workspaces", nil)
	expect(t, list, http.StatusOK)
	if ws := list.body["workspaces"].([]any); len(ws) != 0 {
		t.Fatalf("new user has workspaces: %v", ws)
	}

	// Welcome screen.
	expect(t, b.do("POST", "/v1/workspaces", map[string]any{"name": "Ada's workspace", "first_goal": "nope", "start_mode": "setup"}),
		http.StatusUnprocessableEntity)
	created := b.do("POST", "/v1/workspaces", map[string]any{"name": "Ada's workspace", "first_goal": "offers", "start_mode": "setup"})
	expect(t, created, http.StatusCreated)
	wsID := created.body["id"].(string)
	if created.body["current_step"] != "business" || created.body["completed"] != false || created.body["is_owner"] != true {
		t.Fatalf("created = %v", created.body)
	}
	expect(t, b.do("POST", "/v1/workspaces", map[string]any{"name": "Second", "first_goal": "offers", "start_mode": "setup"}),
		http.StatusConflict, "workspace_limit_reached")

	base := "/v1/workspaces/" + wsID
	ws := b.do("GET", base, nil)
	expect(t, ws, http.StatusOK)
	if ws.body["plan"] != "growth" || ws.body["trial_ends_at"] == nil {
		t.Errorf("workspace = %v", ws.body)
	}

	// Strangers can't see it, and can't tell it exists.
	stranger := e.signedIn()
	expect(t, stranger.do("GET", base, nil), http.StatusNotFound, "workspace_not_found")
	expect(t, stranger.do("GET", base+"/onboarding", nil), http.StatusNotFound, "workspace_not_found")
	expect(t, stranger.do("GET", "/v1/workspaces/not-a-uuid", nil), http.StatusNotFound, "workspace_not_found")

	// Step 1: business.
	expect(t, b.do("POST", base+"/onboarding/skip-whatsapp", nil), http.StatusConflict, "onboarding_step_invalid")
	business := map[string]any{
		"name": "Acme Store", "industry": "retail_ecommerce", "website": "not a site", "country": "bd",
		"team_size": "2_5", "goals": []string{"marketing", "sales"},
	}
	expect(t, b.do("PUT", base+"/onboarding/business", business), http.StatusUnprocessableEntity, "invalid_input")
	business["website"] = "acme.example"
	saved := b.do("PUT", base+"/onboarding/business", business)
	expect(t, saved, http.StatusOK)
	if saved.body["name"] != "Acme Store" || saved.body["country"] != "BD" || saved.body["website"] != "https://acme.example" {
		t.Errorf("saved = %v", saved.body)
	}
	ob := b.do("GET", base+"/onboarding", nil)
	expect(t, ob, http.StatusOK)
	if ob.body["current_step"] != "whatsapp" {
		t.Fatalf("onboarding = %v", ob.body)
	}

	// Step 2: WhatsApp.
	cfg := b.do("GET", "/v1/whatsapp/signup-config", nil)
	expect(t, cfg, http.StatusOK)
	if cfg.body["enabled"] != true || cfg.body["config_id"] != "777" {
		t.Errorf("signup config = %v", cfg.body)
	}
	connect := map[string]any{"code": "bad-code", "waba_id": testWABA, "phone_number_id": testPhoneID, "number_type": "new"}
	expect(t, b.do("POST", base+"/whatsapp/connections", connect), http.StatusUnprocessableEntity, "whatsapp_code_exchange_failed")
	connect["code"] = "good-code"
	num := b.do("POST", base+"/whatsapp/connections", connect)
	expect(t, num, http.StatusAccepted)
	numberID := num.body["id"].(string)
	if num.body["status"] != "setting_up" || num.body["display_phone_number"] != testDisplay {
		t.Fatalf("number = %v", num.body)
	}
	if strings.Contains(num.raw, "biz-token") {
		t.Fatal("number response leaked the business token")
	}
	// Another workspace can't take the same WABA.
	stranger.do("POST", "/v1/workspaces", map[string]any{"name": "Eve's", "first_goal": "support", "start_mode": "setup"})
	strangerWS := stranger.do("GET", "/v1/workspaces", nil).body["workspaces"].([]any)[0].(map[string]any)["id"].(string)
	expect(t, stranger.do("POST", "/v1/workspaces/"+strangerWS+"/whatsapp/connections", connect),
		http.StatusConflict, "whatsapp_already_connected")

	e.runSetupJob(wsID, numberID)
	nums := b.do("GET", base+"/whatsapp/numbers", nil)
	expect(t, nums, http.StatusOK)
	n := nums.body["numbers"].([]any)[0].(map[string]any)
	tasks := n["setup_tasks"].(map[string]any)
	if n["status"] != "connected" || n["wa_link"] != "wa.me/14155550142" || tasks["register"] != "done" || tasks["import_profile"] != "done" {
		t.Fatalf("number after setup = %v", n)
	}
	if len(e.graph.pins) != 1 || len(e.graph.pins[0]) != 6 {
		t.Errorf("register pins = %v", e.graph.pins)
	}
	var stored []byte
	if err := e.a.db.QueryRow(context.Background(), `SELECT access_token FROM whatsapp_accounts WHERE waba_id = $1`, testWABA).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stored, []byte("biz-token")) {
		t.Fatal("business token stored in plain text")
	}

	ob = b.do("GET", base+"/onboarding", nil)
	if ob.body["current_step"] != "test_message" || ob.body["checklist"].([]any)[1].(map[string]any)["done"] != true {
		t.Fatalf("onboarding after connect = %v", ob.body)
	}

	// Step 3: test message.
	send := base + "/whatsapp/numbers/" + numberID + "/test-messages"
	expect(t, b.do("POST", send, map[string]any{"to": "+1 (415) 555-0142"}), http.StatusUnprocessableEntity, "same_number")
	failed := b.do("POST", send, map[string]any{"to": "+8801700000000"})
	expect(t, failed, http.StatusAccepted)
	if failed.body["status"] != "failed" || failed.body["error_code"] != "meta_131026" {
		t.Errorf("failed message = %v", failed.body)
	}
	msg := b.do("POST", send, map[string]any{"to": "+880 1712-345678"})
	expect(t, msg, http.StatusAccepted)
	if msg.body["status"] != "sent" || msg.body["to_phone"] != "+8801712345678" {
		t.Fatalf("message = %v", msg.body)
	}

	// Meta reports delivery through the webhook; a bad signature is refused.
	req := httptest.NewRequestWithContext(t.Context(), "POST", "/v1/webhooks/whatsapp", strings.NewReader(`{}`))
	req.Header.Set("X-Hub-Signature-256", "sha256=00")
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unsigned webhook: %d", rec.Code)
	}
	e.webhook(statusChange("messages", map[string]any{
		"messaging_product": "whatsapp",
		"statuses":          []any{map[string]any{"id": "wamid.8801712345678", "status": "delivered"}},
	}))
	// A late "sent" doesn't move it back.
	e.webhook(statusChange("messages", map[string]any{
		"statuses": []any{map[string]any{"id": "wamid.8801712345678", "status": "sent"}},
	}))
	got := b.do("GET", base+"/whatsapp/test-messages/"+msg.body["id"].(string), nil)
	expect(t, got, http.StatusOK)
	if got.body["status"] != "delivered" {
		t.Errorf("test message = %v", got.body)
	}
	var sentAt *string
	if err := e.a.db.QueryRow(context.Background(),
		`SELECT test_message_sent_at::text FROM workspace_onboarding WHERE workspace_id = $1`, wsID).Scan(&sentAt); err != nil || sentAt == nil {
		t.Errorf("test_message_sent_at not set: %v", err)
	}

	done := b.do("POST", base+"/onboarding/complete", nil)
	expect(t, done, http.StatusOK)
	if done.body["current_step"] != "done" || done.body["completed"] != true {
		t.Fatalf("completed = %v", done.body)
	}
	summary := b.do("GET", "/v1/workspaces", nil).body["workspaces"].([]any)[0].(map[string]any)
	if summary["current_step"] != "done" || summary["completed"] != true {
		t.Errorf("summary = %v", summary)
	}

	// Per-member dashboard state.
	me := b.do("PATCH", base+"/onboarding/me", map[string]any{"tour_seen": true, "checklist_hidden": true})
	expect(t, me, http.StatusOK)
	if me.body["tour_seen"] != true || me.body["checklist_hidden"] != true {
		t.Errorf("member state = %v", me.body)
	}
	me = b.do("PATCH", base+"/onboarding/me", map[string]any{"checklist_hidden": false})
	if me.body["tour_seen"] != true || me.body["checklist_hidden"] != false {
		t.Errorf("member state after unhide = %v", me.body)
	}

	// Webhook handshake, then Meta reports our access was removed.
	hs := b.do("GET", "/v1/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token="+testVerifyToken+"&hub.challenge=abc123", nil)
	if hs.code != http.StatusOK || hs.raw != "abc123" {
		t.Errorf("handshake: %d %q", hs.code, hs.raw)
	}
	expect(t, b.do("GET", "/v1/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=x", nil), http.StatusForbidden)
	e.webhook(statusChange("account_update", map[string]any{"event": "PARTNER_REMOVED"}))
	n = b.do("GET", base+"/whatsapp/numbers", nil).body["numbers"].([]any)[0].(map[string]any)
	if n["status"] != "disconnected" {
		t.Errorf("number after PARTNER_REMOVED = %v", n)
	}
	expect(t, b.do("POST", send, map[string]any{"to": "+8801712345679"}), http.StatusConflict, "whatsapp_disconnected")
}

func TestOnboardingSampleModeAndSkip(t *testing.T) {
	e := newAuthEnv(t)
	b := e.signedIn()
	created := b.do("POST", "/v1/workspaces", map[string]any{"name": "Ada's workspace", "first_goal": "support", "start_mode": "sample"})
	expect(t, created, http.StatusCreated)
	if created.body["completed"] != true || created.body["current_step"] != "business" {
		t.Fatalf("sample workspace = %v", created.body)
	}
	base := "/v1/workspaces/" + created.body["id"].(string)
	ob := b.do("GET", base+"/onboarding", nil)
	if ob.body["sample_data"] != true || ob.body["first_goal"] != "support" {
		t.Errorf("onboarding = %v", ob.body)
	}
	expect(t, b.do("PUT", base+"/onboarding/business", map[string]any{
		"name": "Ada Café", "industry": "food_restaurants", "country": "GB", "team_size": "just_me", "goals": []string{},
	}), http.StatusOK)
	expect(t, b.do("POST", base+"/onboarding/complete", nil), http.StatusConflict, "whatsapp_not_connected")
	skipped := b.do("POST", base+"/onboarding/skip-whatsapp", nil)
	expect(t, skipped, http.StatusOK)
	steps := skipped.body["steps"].([]any)
	if skipped.body["whatsapp_skipped"] != true || steps[0].(map[string]any)["status"] != "done" ||
		steps[1].(map[string]any)["status"] != "skipped" || steps[2].(map[string]any)["status"] != "skipped" {
		t.Errorf("after skip = %v", skipped.body)
	}
}

func TestMarkActivatedOnce(t *testing.T) {
	e := newAuthEnv(t)
	b := e.signedIn()
	created := b.do("POST", "/v1/workspaces", map[string]any{"name": "Ada's workspace", "first_goal": "offers", "start_mode": "setup"})
	wsID := uuid.MustParse(created.body["id"].(string))
	for i, want := range []bool{true, false} {
		got, err := e.a.directory.MarkActivated(t.Context(), wsID)
		if err != nil || got != want {
			t.Fatalf("call %d: MarkActivated = %v, %v; want %v", i+1, got, err, want)
		}
	}
	if b.do("GET", "/v1/workspaces", nil).body["workspaces"].([]any)[0].(map[string]any)["activated"] != true {
		t.Error("workspace not activated")
	}
	// Activated workspaces don't count toward the one-trial limit.
	expect(t, b.do("POST", "/v1/workspaces", map[string]any{"name": "Second", "first_goal": "offers", "start_mode": "setup"}), http.StatusCreated)
}

func TestOnboardingReminders(t *testing.T) {
	e := newAuthEnv(t)
	b := e.signedIn()
	created := b.do("POST", "/v1/workspaces", map[string]any{"name": "Ada's workspace", "first_goal": "offers", "start_mode": "setup"})
	expect(t, created, http.StatusCreated)
	wsID := uuid.MustParse(created.body["id"].(string))

	var setup, trial int
	if err := e.a.db.QueryRow(context.Background(), `SELECT
		count(*) FILTER (WHERE kind = 'onboarding_reminder'), count(*) FILTER (WHERE kind = 'trial_reminder')
		FROM river_job WHERE args->>'workspace_id' = $1`, wsID.String()).Scan(&setup, &trial); err != nil {
		t.Fatal(err)
	}
	if setup != 2 || trial != 2 {
		t.Fatalf("scheduled reminders: setup=%d trial=%d", setup, trial)
	}

	var owner string
	if err := e.a.db.QueryRow(context.Background(),
		`SELECT u.email FROM users u JOIN workspace_members m ON m.user_id = u.id WHERE m.workspace_id = $1`, wsID).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	work := func(n int) {
		err := onboardingjobs.NewSetupReminderWorker(e.a.onboarding).Work(t.Context(), &river.Job[onboardingmodel.SetupReminderArgs]{
			JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 25}, Args: onboardingmodel.SetupReminderArgs{WorkspaceID: wsID, N: n},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	work(1)
	if subject, _ := e.lastEmail(owner); subject != "Finish connecting WhatsApp to Relay" {
		t.Errorf("reminder subject = %q", subject)
	}
	work(2)
	work(3) // at most two
	var count int
	if err := e.a.db.QueryRow(context.Background(),
		`SELECT count(*) FROM river_job WHERE kind = 'send_email' AND args->'message'->'to' ? $1 AND args->'message'->>'subject' LIKE 'Finish connecting%'`,
		owner).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("setup reminder emails = %d, want 2", count)
	}

	err := onboardingjobs.NewTrialReminderWorker(e.a.onboarding).Work(t.Context(), &river.Job[onboardingmodel.TrialReminderArgs]{
		JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 25}, Args: onboardingmodel.TrialReminderArgs{WorkspaceID: wsID, DaysLeft: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if subject, _ := e.lastEmail(owner); subject != "Your Relay trial ends tomorrow" {
		t.Errorf("trial reminder subject = %q", subject)
	}
}
