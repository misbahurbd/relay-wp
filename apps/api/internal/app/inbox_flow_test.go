package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	inboxmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
	inboxservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/service"
	whatsappmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/realtime"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/storage"
)

// fakeWhatsApp stands in for WhatsApp's sends: it records them and answers with
// new wamids. Capabilities and Accounts are the real WhatsApp channel's.
type fakeWhatsApp struct {
	whatsappChannel
	mu    sync.Mutex
	sent  []string // "text:+44…:body", "template:+44…:name", …
	reads []string
	n     int
	// fail, when set, is returned by the next send.
	fail error
}

func (f *fakeWhatsApp) record(s string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		err := f.fail
		f.fail = nil
		return "", err
	}
	f.n++
	f.sent = append(f.sent, s)
	return fmt.Sprintf("wamid.out.%d", f.n), nil
}

func (f *fakeWhatsApp) failNext(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = err
}

func (f *fakeWhatsApp) last() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) == 0 {
		return ""
	}
	return f.sent[len(f.sent)-1]
}

func (f *fakeWhatsApp) SendText(_ context.Context, _, _ uuid.UUID, to, body, _ string) (string, error) {
	return f.record("text:" + to + ":" + body)
}

func (f *fakeWhatsApp) SendMedia(_ context.Context, _, _ uuid.UUID, to string, m inboxmodel.OutboundMedia, _ string) (string, error) {
	return f.record("media:" + to + ":" + m.Kind + ":" + m.MediaID)
}

func (f *fakeWhatsApp) SendTemplate(_ context.Context, _, _ uuid.UUID, to string, t inboxmodel.OutboundTemplate) (string, error) {
	return f.record("template:" + to + ":" + t.Name + ":" + strings.Join(t.BodyParams, ","))
}

func (f *fakeWhatsApp) SendReaction(_ context.Context, _, _ uuid.UUID, to, target, emoji string) (string, error) {
	return f.record("reaction:" + to + ":" + target + ":" + emoji)
}

func (f *fakeWhatsApp) MarkRead(_ context.Context, _, _ uuid.UUID, wamid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads = append(f.reads, wamid)
	return nil
}

func (f *fakeWhatsApp) UploadMedia(_ context.Context, _, _ uuid.UUID, _, filename string, data io.Reader, size int64) (string, error) {
	b, _ := io.ReadAll(data)
	if int64(len(b)) != size {
		return "", fmt.Errorf("read %d bytes, want %d", len(b), size)
	}
	return "meta-media-" + filename, nil
}

func (f *fakeWhatsApp) FetchMedia(context.Context, uuid.UUID, uuid.UUID, string) (io.ReadCloser, string, int64, error) {
	return io.NopCloser(strings.NewReader("jpeg-bytes")), "image/jpeg", 10, nil
}

// inboxEnv is a team workspace with a connected number, a fake WhatsApp and a live hub.
type inboxEnv struct {
	*teamWorkspace
	wsID     uuid.UUID
	numberID uuid.UUID
	channel  *fakeWhatsApp
	srv      *httptest.Server
	seq      int
}

func newInboxEnv(t *testing.T) *inboxEnv {
	t.Helper()
	e := newAuthEnv(t)
	store, err := storage.New(t.Context(), config.Storage{Driver: "memory"}, false)
	if err != nil {
		t.Fatal(err)
	}
	channel := &fakeWhatsApp{whatsappChannel: whatsappChannel{wa: e.a.whatsapp}}
	e.a.storage = store
	e.a.inboxChannelsOverride = inboxservice.Channels{inboxmodel.ChannelWhatsApp: channel}
	e.a.buildInbox()
	// The test pool is direct, so the hub can LISTEN on it.
	hub := realtime.New(e.a.db, e.a.log, realtime.InboxChannel)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = hub.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	e.a.live = hub
	e.a.inbox.SetSubscriber(hub)
	if e.h, err = e.a.Handler(); err != nil {
		t.Fatal(err)
	}
	w := newTeamWorkspace(e)
	env := &inboxEnv{teamWorkspace: w, wsID: uuid.MustParse(w.id), channel: channel, numberID: uuid.Must(uuid.NewV7())}
	account := uuid.Must(uuid.NewV7())
	e.exec(`INSERT INTO whatsapp_accounts (id, workspace_id, waba_id, access_token) VALUES ($1, $2, $3, '\x00')`,
		account, env.wsID, "waba-"+account.String())
	e.exec(`INSERT INTO whatsapp_numbers (id, workspace_id, account_id, phone_number_id, display_phone_number,
		verified_name, number_type, status) VALUES ($1, $2, $3, $4, '+44 20 7946 0000', 'Acme Support', 'new', 'connected')`,
		env.numberID, env.wsID, account, "pn-"+env.numberID.String())
	env.srv = httptest.NewServer(e.h)
	t.Cleanup(env.srv.Close)
	return env
}

// inbound delivers a customer's message through the whatsapp sink, as the webhook job would.
func (v *inboxEnv) inbound(wamid, from, text string) {
	v.e.t.Helper()
	err := inboxSink{inbox: v.e.a.inbox}.InboundMessage(context.Background(), whatsappmodel.InboundMessage{
		WorkspaceID: v.wsID, NumberID: v.numberID, PhoneNumberID: "pn", WAMID: wamid, From: from,
		ProfileName: "Grace Hopper", Timestamp: time.Now().UTC(), Type: "text", Text: text,
	})
	if err != nil {
		v.e.t.Fatal(err)
	}
}

func (v *inboxEnv) status(wamid, status string) {
	v.e.t.Helper()
	err := inboxSink{inbox: v.e.a.inbox}.MessageStatus(context.Background(), whatsappmodel.StatusUpdate{
		WorkspaceID: v.wsID, NumberID: v.numberID, WAMID: wamid, Status: status, Timestamp: time.Now().UTC(),
	})
	if err != nil {
		v.e.t.Fatal(err)
	}
}

// runSends works every queued inbox_send job once, as River would.
func (v *inboxEnv) runSends(final bool) int {
	v.e.t.Helper()
	rows, err := v.e.a.db.Query(context.Background(),
		`UPDATE river_job SET state = 'completed', finalized_at = now()
		 WHERE kind = 'inbox_send' AND state = 'available' RETURNING args`)
	if err != nil {
		v.e.t.Fatal(err)
	}
	var jobs []inboxmodel.SendArgs
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			v.e.t.Fatal(err)
		}
		var j inboxmodel.SendArgs
		_ = json.Unmarshal(raw, &j)
		jobs = append(jobs, j)
	}
	rows.Close()
	for _, j := range jobs {
		if err := v.e.a.inbox.Deliver(context.Background(), j.WorkspaceID, j.MessageID, final); err != nil && final {
			v.e.t.Fatal(err)
		}
	}
	return len(jobs)
}

func (v *inboxEnv) jobs(kind string) int {
	v.e.t.Helper()
	var n int
	if err := v.e.a.db.QueryRow(context.Background(),
		`SELECT count(id) FROM river_job WHERE kind = $1 AND args->>'workspace_id' = $2`, kind, v.id).Scan(&n); err != nil {
		v.e.t.Fatal(err)
	}
	return n
}

func (v *inboxEnv) phone() string {
	v.seq++
	return fmt.Sprintf("+4477009%05d", v.seq)
}

func (v *inboxEnv) only(t *testing.T, b *browser, query string) map[string]any {
	t.Helper()
	r := b.do("GET", v.base+"/inbox/conversations?"+query, nil)
	expect(t, r, http.StatusOK)
	list := items(r)
	if len(list) != 1 {
		t.Fatalf("want one conversation for %q: %s", query, r.raw)
	}
	return list[0]
}

func thread(t *testing.T, b *browser, base, conv string) []map[string]any {
	t.Helper()
	r := b.do("GET", base+"/inbox/conversations/"+conv+"/messages", nil)
	expect(t, r, http.StatusOK)
	return items(r)
}

func TestInboxIngestSendAndStatuses(t *testing.T) {
	v := newInboxEnv(t)
	owner, base := v.owner, v.base+"/inbox"
	phone := v.phone()

	// A webhook delivered twice is stored once (AD-3).
	v.inbound("wamid.in.1", phone, "Hi, is my order shipped?")
	v.inbound("wamid.in.1", phone, "Hi, is my order shipped?")
	conv := v.only(t, owner, "tab=open&q="+url.QueryEscape("order shipped"))
	id := conv["id"].(string)
	if conv["status"] != "open" || conv["unread_count"].(float64) != 1 || conv["last_message_direction"] != "in" ||
		conv["waiting_since"] == nil || conv["window_expires_at"] == nil {
		t.Fatalf("conversation: %v", conv)
	}
	if msgs := thread(t, owner, v.base, id); len(msgs) != 1 || msgs[0]["body"] != "Hi, is my order shipped?" {
		t.Fatalf("thread: %v", msgs)
	}
	contact := (&contactsWorkspace{teamWorkspace: v.teamWorkspace}).contactByPhone(t, phone)
	if contact["source"] != "inbox" || contact["name"] != "Grace Hopper" || contact["last_message_at"] == nil {
		t.Fatalf("contact: %v", contact)
	}
	list := owner.do("GET", base+"/conversations", nil)
	if c := list.body["counts"].(map[string]any); c["open"].(float64) != 1 || c["mine"].(float64) != 0 {
		t.Fatalf("counts: %s", list.raw)
	}

	// A reply inside the window goes out through the job.
	sent := owner.do("POST", base+"/conversations/"+id+"/messages", map[string]any{"type": "text", "text": "Yes, it left today."})
	expect(t, sent, http.StatusCreated)
	if sent.body["status"] != "pending" || sent.body["author"] == nil {
		t.Fatalf("sent: %s", sent.raw)
	}
	if n := v.runSends(false); n != 1 {
		t.Fatalf("ran %d sends", n)
	}
	if got := v.channel.last(); got != "text:"+phone+":Yes, it left today." {
		t.Fatalf("channel got %q", got)
	}
	msgs := thread(t, owner, v.base, id)
	if msgs[0]["status"] != "sent" {
		t.Fatalf("after send: %v", msgs[0])
	}
	got := owner.do("GET", base+"/conversations/"+id, nil)
	expect(t, got, http.StatusOK)
	if got.body["waiting_since"] != nil || got.body["last_message_direction"] != "out" || got.body["window_open"] != true {
		t.Fatalf("after reply: %s", got.raw)
	}
	// The chat names its channel and says what it allows (ADR 0010).
	ch := got.body["channel"].(map[string]any)
	caps := got.body["capabilities"].(map[string]any)
	if ch["type"] != "whatsapp" || ch["id"] != v.numberID.String() || ch["name"] != "Acme Support" ||
		ch["handle"] != "+44 20 7946 0000" || caps["window_hours"].(float64) != 24 || caps["templates"] != true ||
		caps["reactions"] != true || caps["read_receipts"] != true || caps["max_text_length"].(float64) != 4096 ||
		len(caps["media"].([]any)) != 4 {
		t.Fatalf("channel and capabilities: %s", got.raw)
	}
	if conv["channel"].(map[string]any)["name"] != "Acme Support" {
		t.Fatalf("list item channel: %v", conv)
	}

	// Statuses only move forward (AD-7): read, then a late delivered, stays read.
	v.status("wamid.out.1", "read")
	v.status("wamid.out.1", "delivered")
	v.status("wamid.unknown", "delivered") // ignored
	if msgs := thread(t, owner, v.base, id); msgs[0]["status"] != "read" {
		t.Fatalf("status went backwards: %v", msgs[0])
	}

	// A permanent Meta refusal fails the message without a retry.
	expect(t, owner.do("POST", base+"/conversations/"+id+"/messages", map[string]any{"type": "text", "text": "Again"}), http.StatusCreated)
	v.channel.failNext(&inboxmodel.SendFailure{Code: "131047", Title: "Re-engagement message", Permanent: true})
	v.runSends(false)
	if m := thread(t, owner, v.base, id)[0]; m["status"] != "failed" || m["error_code"] != "131047" {
		t.Fatalf("failed send: %v", m)
	}
	// A transport error retries, and fails only on the last attempt.
	expect(t, owner.do("POST", base+"/conversations/"+id+"/messages", map[string]any{"type": "text", "text": "Third"}), http.StatusCreated)
	v.channel.failNext(fmt.Errorf("connection reset"))
	v.runSends(true)
	if m := thread(t, owner, v.base, id)[0]; m["status"] != "failed" || m["error_code"] != "send_failed" {
		t.Fatalf("final attempt: %v", m)
	}

	// Mark read clears the unread count and queues a read receipt.
	expect(t, owner.do("POST", base+"/conversations/"+id+"/read", nil), http.StatusNoContent)
	if v.jobs("inbox_mark_read") != 1 {
		t.Fatal("no read receipt queued")
	}
	if c := v.only(t, owner, "tab=open"); c["unread_count"].(float64) != 0 {
		t.Fatalf("unread: %v", c)
	}

	// After the window, free-form sends are refused; a template is allowed.
	v.e.exec(`UPDATE conversations SET window_expires_at = now() - interval '1 minute' WHERE id = $1`, id)
	expect(t, owner.do("POST", base+"/conversations/"+id+"/messages", map[string]any{"type": "text", "text": "Late"}),
		http.StatusConflict, "window_closed")

	tplID := uuid.Must(uuid.NewV7())
	v.e.exec(`INSERT INTO message_templates (id, workspace_id, name, language, category, status, components)
		VALUES ($1, $2, 'order_update', 'en_US', 'utility', 'approved', $3)`, tplID, v.wsID,
		`{"header":{"type":"none"},"body":{"text":"Hi {{1}}, order {{2}} shipped.","examples":["a","b"]},"footer":{"text":""},"buttons":[]}`)
	tpls := owner.do("GET", base+"/templates", nil)
	expect(t, tpls, http.StatusOK)
	if ts := items(tpls); len(ts) != 1 || ts[0]["body_variables"].(float64) != 2 {
		t.Fatalf("templates: %s", tpls.raw)
	}
	expect(t, owner.do("POST", base+"/conversations/"+id+"/status", map[string]any{"status": "closed"}), http.StatusOK)
	expect(t, owner.do("POST", base+"/conversations/"+id+"/messages", map[string]any{
		"type": "template", "template_id": tplID, "params": map[string]any{"body": []string{"Grace"}},
	}), http.StatusUnprocessableEntity, "template_params_missing")
	tpl := owner.do("POST", base+"/conversations/"+id+"/messages", map[string]any{
		"type": "template", "template_id": tplID, "params": map[string]any{"body": []string{"Grace", "#42"}},
	})
	expect(t, tpl, http.StatusCreated)
	if tpl.body["body"] != "Hi Grace, order #42 shipped." || tpl.body["type"] != "template" {
		t.Fatalf("template message: %s", tpl.raw)
	}
	got = owner.do("GET", base+"/conversations/"+id, nil)
	if got.body["status"] != "pending" || got.body["template_since_inbound"] != true {
		t.Fatalf("template to a closed chat: %s", got.raw)
	}
	v.runSends(false)
	if got := v.channel.last(); got != "template:"+phone+":order_update:Grace,#42" {
		t.Fatalf("channel got %q", got)
	}

	// The customer answers: the chat reopens and the window restarts.
	v.inbound("wamid.in.2", phone, "Thanks!")
	got = owner.do("GET", base+"/conversations/"+id, nil)
	if got.body["status"] != "open" || got.body["window_open"] != true || got.body["unread_count"].(float64) != 1 {
		t.Fatalf("after the answer: %s", got.raw)
	}
	// The thread says why the chat moved: the template restarted it, and the reply reopened the window.
	page := owner.do("GET", base+"/conversations/"+id+"/messages", nil)
	expect(t, page, http.StatusOK)
	var actions []string
	for _, m := range items(page) {
		if m["kind"] == "event" {
			actions = append(actions, m["content"].(map[string]any)["event"].(map[string]any)["action"].(string))
		}
	}
	if !slices.Contains(actions, "template_pending") || !slices.Contains(actions, "window_reopened") {
		t.Fatalf("events: %v", actions)
	}
}

func TestInboxNotesAssignAndSnooze(t *testing.T) {
	v := newInboxEnv(t)
	owner, base := v.owner, v.base+"/inbox"
	agent, agentID := v.join("agent")
	viewer, viewerID := v.join("viewer")
	// The viewer reads every chat, so their refusals below are about permissions, not scope.
	v.e.exec(`UPDATE workspace_members SET chat_scope = 'all' WHERE workspace_id = $1 AND user_id = $2`, v.wsID, viewerID)
	v.inbound("wamid.n.1", v.phone(), "Hello")
	id := v.only(t, owner, "tab=open")["id"].(string)
	conv := base + "/conversations/" + id

	// Notes: a mention must be a member who can see the chat (AD-12).
	expect(t, owner.do("POST", conv+"/notes", map[string]any{"text": "Who?", "mentions": []string{uuid.NewString()}}),
		http.StatusUnprocessableEntity, "invalid_mention")
	expect(t, viewer.do("POST", conv+"/notes", map[string]any{"text": "Hi"}), http.StatusForbidden, "permission_denied")
	// The agent's team scope doesn't reach an unassigned chat, so they can't be mentioned yet.
	expect(t, owner.do("POST", conv+"/notes", map[string]any{"text": "@agent look", "mentions": []string{agentID}}),
		http.StatusUnprocessableEntity, "invalid_mention")

	// Assign to the agent: now they see it and can be mentioned.
	assigned := owner.do("POST", conv+"/assign", map[string]any{"user_id": agentID})
	expect(t, assigned, http.StatusOK)
	if a := assigned.body["assignee"].(map[string]any); a["id"] != agentID {
		t.Fatalf("assigned: %s", assigned.raw)
	}
	note := owner.do("POST", conv+"/notes", map[string]any{"text": "@agent look", "mentions": []string{agentID}})
	expect(t, note, http.StatusCreated)
	if note.body["kind"] != "note" {
		t.Fatalf("note: %s", note.raw)
	}
	mentions := agent.do("GET", base+"/mentions", nil)
	expect(t, mentions, http.StatusOK)
	if mentions.body["unseen"].(float64) != 1 {
		t.Fatalf("mentions: %s", mentions.raw)
	}
	seen := agent.do("POST", base+"/mentions/seen", map[string]any{})
	expect(t, seen, http.StatusOK)
	if seen.body["updated"].(float64) != 1 {
		t.Fatalf("seen: %s", seen.raw)
	}
	if c := agent.do("GET", base+"/conversations?tab=mine", nil); len(items(c)) != 1 {
		t.Fatalf("mine: %s", c.raw)
	}
	msgs := thread(t, owner, v.base, id)
	if msgs[1]["kind"] != "event" || msgs[1]["content"].(map[string]any)["event"].(map[string]any)["action"] != "assigned" {
		t.Fatalf("assign event: %v", msgs[1])
	}

	// Close, reopen, snooze; then the snooze ends.
	closed := owner.do("POST", conv+"/status", map[string]any{"status": "closed"})
	expect(t, closed, http.StatusOK)
	if closed.body["status"] != "closed" || closed.body["closed_at"] == nil {
		t.Fatalf("closed: %s", closed.raw)
	}
	expect(t, owner.do("POST", conv+"/status", map[string]any{"status": "open"}), http.StatusOK)
	expect(t, owner.do("POST", conv+"/status", map[string]any{"snooze_until": time.Now().Add(-time.Minute)}),
		http.StatusUnprocessableEntity, "invalid_input")
	snoozed := owner.do("POST", conv+"/status", map[string]any{"snooze_until": time.Now().Add(time.Hour)})
	expect(t, snoozed, http.StatusOK)
	if snoozed.body["status"] != "pending" || snoozed.body["snoozed_until"] == nil {
		t.Fatalf("snoozed: %s", snoozed.raw)
	}
	v.e.exec(`UPDATE conversations SET snoozed_until = now() - interval '1 second' WHERE id = $1`, id)
	if n, err := v.e.a.inbox.Unsnooze(context.Background()); err != nil || n != 1 {
		t.Fatalf("unsnooze: %d, %v", n, err)
	}
	if got := owner.do("GET", conv, nil); got.body["status"] != "open" || got.body["snoozed_until"] != nil {
		t.Fatalf("after unsnooze: %s", got.raw)
	}

	// Unassign; a viewer can't take a chat, an agent may take an unassigned one.
	expect(t, owner.do("POST", conv+"/assign", map[string]any{"user_id": nil}), http.StatusOK)
	expect(t, viewer.do("POST", conv+"/assign", map[string]any{"user_id": agentID}), http.StatusForbidden, "permission_denied")

	// Quick replies: everyone reads, inbox.reply writes.
	qr := agent.do("POST", base+"/quick-replies", map[string]any{"shortcut": "/thanks", "title": "Thanks", "body": "Thank you!"})
	expect(t, qr, http.StatusCreated)
	if qr.body["shortcut"] != "thanks" {
		t.Fatalf("quick reply: %s", qr.raw)
	}
	expect(t, agent.do("POST", base+"/quick-replies", map[string]any{"shortcut": "thanks", "title": "x", "body": "y"}),
		http.StatusConflict, "quick_reply_exists")
	expect(t, viewer.do("POST", base+"/quick-replies", map[string]any{"shortcut": "hi", "title": "x", "body": "y"}),
		http.StatusForbidden, "permission_denied")
	if l := viewer.do("GET", base+"/quick-replies", nil); len(items(l)) != 1 {
		t.Fatalf("quick replies: %s", l.raw)
	}
	qid := qr.body["id"].(string)
	expect(t, agent.do("PATCH", base+"/quick-replies/"+qid, map[string]any{"body": "Thanks a lot!"}), http.StatusOK)
	expect(t, agent.do("DELETE", base+"/quick-replies/"+qid, nil), http.StatusNoContent)
}

func TestInboxMediaUploadAndFetch(t *testing.T) {
	v := newInboxEnv(t)
	owner, base := v.owner, v.base+"/inbox"
	phone := v.phone()
	err := inboxSink{inbox: v.e.a.inbox}.InboundMessage(context.Background(), whatsappmodel.InboundMessage{
		WorkspaceID: v.wsID, NumberID: v.numberID, WAMID: "wamid.img.1", From: phone, Timestamp: time.Now().UTC(),
		Type: "image", Text: "my receipt", Media: &whatsappmodel.InboundMedia{ID: "meta-1", MimeType: "image/jpeg"},
	})
	if err != nil {
		t.Fatal(err)
	}
	id := v.only(t, owner, "tab=open")["id"].(string)
	msg := thread(t, owner, v.base, id)[0]
	media := base + "/conversations/" + id + "/messages/" + msg["id"].(string) + "/media"
	expect(t, owner.do("GET", media, nil), http.StatusNotFound, "media_not_ready")
	if err := v.e.a.inbox.FetchMedia(context.Background(), v.wsID, uuid.MustParse(msg["id"].(string))); err != nil {
		t.Fatal(err)
	}
	got := owner.do("GET", media, nil)
	expect(t, got, http.StatusOK)
	if got.raw != "jpeg-bytes" || got.header.Get("Content-Type") != "image/jpeg" ||
		!strings.HasPrefix(got.header.Get("Content-Disposition"), "inline") {
		t.Fatalf("media: %q %v", got.raw, got.header)
	}

	// Upload and send a document.
	bad := owner.upload(base+"/media", "virus.exe", []byte("MZ"))
	expect(t, bad, http.StatusUnprocessableEntity, "media_type_not_allowed")
	up := owner.upload(base+"/media", "invoice 42.pdf", []byte("%PDF-1.7"))
	expect(t, up, http.StatusCreated)
	if up.body["kind"] != "document" || up.body["filename"] != "invoice_42.pdf" {
		t.Fatalf("upload: %s", up.raw)
	}
	sent := owner.do("POST", base+"/conversations/"+id+"/messages", map[string]any{
		"type": "media", "media_key": up.body["key"], "caption": "Your invoice",
	})
	expect(t, sent, http.StatusCreated)
	if sent.body["type"] != "document" {
		t.Fatalf("media message: %s", sent.raw)
	}
	v.runSends(false)
	if got := v.channel.last(); got != "media:"+phone+":document:meta-media-invoice_42.pdf" {
		t.Fatalf("channel got %q", got)
	}
	// Another workspace's key is refused.
	foreign := strings.Replace(up.body["key"].(string), v.id, uuid.NewString(), 1)
	expect(t, owner.do("POST", base+"/conversations/"+id+"/messages", map[string]any{"type": "media", "media_key": foreign}),
		http.StatusUnprocessableEntity, "invalid_input")
}

// sse is one open event stream.
type sse struct {
	events chan string // "kind conversation_id"
	cancel context.CancelFunc
	header http.Header
}

func (v *inboxEnv) stream(t *testing.T, b *browser) *sse {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.srv.URL+v.base+"/inbox/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", testOrigin)
	req.Header.Set("X-Workspace-ID", b.workspace)
	for _, c := range b.cookies {
		req.AddCookie(c)
	}
	res, err := http.DefaultClient.Do(req) //nolint:bodyclose // the reader goroutine closes it
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("events: status %d", res.StatusCode)
	}
	s := &sse{events: make(chan string, 16), cancel: cancel, header: res.Header}
	go func() {
		defer func() { _ = res.Body.Close() }()
		sc := bufio.NewScanner(res.Body)
		kind := ""
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				kind = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				var ev realtime.Event
				_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev)
				conv := ""
				if ev.ConversationID != nil {
					conv = ev.ConversationID.String()
				}
				s.events <- kind + " " + conv
			}
		}
	}()
	t.Cleanup(cancel)
	return s
}

// next waits for an event, or returns "" after wait.
func (s *sse) next(wait time.Duration) string {
	select {
	case ev := <-s.events:
		return ev
	case <-time.After(wait):
		return ""
	}
}

func TestInboxScopeAndLiveEvents(t *testing.T) {
	v := newInboxEnv(t)
	owner, base := v.owner, v.base+"/inbox"
	agent, agentID := v.join("agent")
	v.e.exec(`UPDATE workspace_members SET chat_scope = 'assigned' WHERE workspace_id = $1 AND user_id = $2`, v.wsID, agentID)

	ownerLive, agentLive := v.stream(t, owner), v.stream(t, agent)
	if ct := ownerLive.header.Get("Content-Type"); ct != "text/event-stream" || ownerLive.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("headers: %v", ownerLive.header)
	}

	// The hub connects in the background; the first inbound proves it is listening.
	phone := v.phone()
	var id string
	deadline := time.Now().Add(10 * time.Second)
	for i := 0; ; i++ {
		v.inbound(fmt.Sprintf("wamid.live.%d", i), phone, "Anyone there?")
		if ev := ownerLive.next(500 * time.Millisecond); ev != "" {
			if !strings.HasPrefix(ev, "message ") {
				t.Fatalf("owner event: %q", ev)
			}
			id = strings.TrimPrefix(ev, "message ")
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("owner got no live event")
		}
	}
	// The agent sees only chats assigned to them: no event, 404 on the chat, an empty list.
	if ev := agentLive.next(time.Second); ev != "" {
		t.Fatalf("agent got an event for a chat outside their scope: %q", ev)
	}
	expect(t, agent.do("GET", base+"/conversations/"+id, nil), http.StatusNotFound, "conversation_not_found")
	expect(t, agent.do("POST", base+"/conversations/"+id+"/messages", map[string]any{"type": "text", "text": "x"}),
		http.StatusNotFound, "conversation_not_found")
	if l := agent.do("GET", base+"/conversations", nil); len(items(l)) != 0 || l.body["counts"].(map[string]any)["open"].(float64) != 0 {
		t.Fatalf("agent list: %s", l.raw)
	}

	// Assigned to the agent: they hear it and can open it.
	expect(t, owner.do("POST", base+"/conversations/"+id+"/assign", map[string]any{"user_id": agentID}), http.StatusOK)
	if ev := agentLive.next(5 * time.Second); ev != "conversation "+id {
		t.Fatalf("agent event after assign: %q", ev)
	}
	expect(t, agent.do("GET", base+"/conversations/"+id, nil), http.StatusOK)
	if l := agent.do("GET", base+"/conversations", nil); len(items(l)) != 1 {
		t.Fatalf("agent list after assign: %s", l.raw)
	}
}

func TestInboxTeamScopeMergesTeamsAndAssigned(t *testing.T) {
	v := newInboxEnv(t)
	owner, base := v.owner, v.base+"/inbox"
	agent, agentID := v.join("agent") // the Agent role's scope is team
	team := uuid.Must(uuid.NewV7())
	v.e.exec(`INSERT INTO teams (id, workspace_id, name) VALUES ($1, $2, 'Support')`, team, v.wsID)
	v.e.exec(`INSERT INTO team_members (workspace_id, team_id, user_id) VALUES ($1, $2, $3)`, v.wsID, team, agentID)

	ids := map[string]string{}
	for _, name := range []string{"team", "assigned", "hidden"} {
		v.inbound("wamid.scope."+name, v.phone(), name)
		ids[name] = v.only(t, owner, "q="+name)["id"].(string)
	}
	v.e.exec(`UPDATE conversations SET team_id = $1 WHERE id = $2`, team, ids["team"])
	expect(t, owner.do("POST", base+"/conversations/"+ids["assigned"]+"/assign", map[string]any{"user_id": agentID}), http.StatusOK)

	// A new agent session picks up the team.
	first := agent.do("GET", base+"/conversations?limit=1", nil)
	expect(t, first, http.StatusOK)
	if len(items(first)) != 1 || first.body["next_cursor"] == nil {
		t.Fatalf("first page: %s", first.raw)
	}
	if c := first.body["counts"].(map[string]any); c["open"].(float64) != 2 || c["mine"].(float64) != 1 {
		t.Fatalf("counts: %s", first.raw)
	}
	second := agent.do("GET", base+"/conversations?limit=1&cursor="+first.body["next_cursor"].(string), nil)
	expect(t, second, http.StatusOK)
	if len(items(second)) != 1 || second.body["next_cursor"] != nil {
		t.Fatalf("second page: %s", second.raw)
	}
	seen := map[string]bool{items(first)[0]["id"].(string): true, items(second)[0]["id"].(string): true}
	if !seen[ids["team"]] || !seen[ids["assigned"]] {
		t.Fatalf("pages: %v", seen)
	}
	expect(t, agent.do("GET", base+"/conversations/"+ids["hidden"], nil), http.StatusNotFound, "conversation_not_found")
	waiting := agent.do("GET", base+"/conversations?sort=waiting", nil)
	expect(t, waiting, http.StatusOK)
	if list := items(waiting); len(list) != 2 || list[0]["id"] != ids["team"] {
		t.Fatalf("waiting longest first: %s", waiting.raw)
	}
}

// A WhatsApp sender whose phone matches a contact added by hand reaches that
// contact, and still reaches it (and the same chat) after its phone is edited:
// the sender is found by their identity, not the phone (ADR 0010).
func TestInboxSenderIdentityLinksContact(t *testing.T) {
	v := newInboxEnv(t)
	owner, base := v.owner, v.base+"/inbox"
	phone := v.phone()
	added := owner.do("POST", v.base+"/contacts", map[string]any{"name": "Ada Lovelace", "phone": phone})
	expect(t, added, http.StatusCreated)
	contactID := added.body["id"].(string)

	v.inbound("wamid.id.1", phone, "Hello from Ada")
	conv := v.only(t, owner, "tab=open")
	if c := conv["contact"].(map[string]any); c["id"] != contactID || c["name"] != "Ada Lovelace" {
		t.Fatalf("contact: %v", conv)
	}
	var contacts, identities int
	count := func() {
		t.Helper()
		if err := v.e.a.db.QueryRow(context.Background(), `SELECT
			(SELECT count(id) FROM contacts WHERE workspace_id = $1),
			(SELECT count(contact_id) FROM contact_identities WHERE workspace_id = $1 AND contact_id = $2 AND channel_type = 'whatsapp' AND external_id = $3)`,
			v.wsID, contactID, phone).Scan(&contacts, &identities); err != nil {
			t.Fatal(err)
		}
	}
	if count(); contacts != 1 || identities != 1 {
		t.Fatalf("contacts %d, identities %d", contacts, identities)
	}

	expect(t, owner.do("PATCH", v.base+"/contacts/"+contactID, map[string]any{"phone": "+447700900999"}), http.StatusOK)
	v.inbound("wamid.id.2", phone, "Still me")
	again := v.only(t, owner, "tab=open")
	if again["id"] != conv["id"] || again["unread_count"].(float64) != 2 {
		t.Fatalf("after the phone edit: %v", again)
	}
	if count(); contacts != 1 || identities != 1 {
		t.Fatalf("after the phone edit: contacts %d, identities %d", contacts, identities)
	}

	// A reply goes to the number the customer writes from, not the edited phone.
	expect(t, owner.do("POST", base+"/conversations/"+conv["id"].(string)+"/messages",
		map[string]any{"type": "text", "text": "Hi Ada"}), http.StatusCreated)
	v.runSends(false)
	if got := v.channel.last(); got != "text:"+phone+":Hi Ada" {
		t.Fatalf("channel got %q", got)
	}
}

func TestSeedInboxForLocalDevelopment(t *testing.T) {
	v := newInboxEnv(t)
	v.e.a.cfg.AppEnv = "development" // seeding refuses to run anywhere else
	n, err := v.e.a.SeedInbox(t.Context(), v.wsID)
	if err != nil || n != 7 {
		t.Fatalf("seed: %d, %v", n, err)
	}
	// Seeding again stores nothing new.
	if _, err := v.e.a.SeedInbox(t.Context(), v.wsID); err != nil {
		t.Fatal(err)
	}
	r := v.owner.do("GET", v.base+"/inbox/conversations?tab=open", nil)
	expect(t, r, http.StatusOK)
	if got := len(items(r)); got != 6 {
		t.Fatalf("open chats: %d\n%s", got, r.raw)
	}
	// Lucía's two messages are one chat with 2 unread; Jonas's window has closed.
	for _, c := range items(r) {
		switch c["contact"].(map[string]any)["name"] {
		case "Lucía Fernández":
			if c["unread_count"].(float64) != 2 {
				t.Fatalf("lucía: %v", c)
			}
		case "Jonas Weber":
			exp, err := time.Parse(time.RFC3339Nano, c["window_expires_at"].(string))
			if err != nil || !exp.Before(time.Now()) {
				t.Fatalf("jonas: %v", c)
			}
		}
	}
}
