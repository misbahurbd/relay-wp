package service

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/realtime"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

func TestCanSee(t *testing.T) {
	me, other := uuid.New(), uuid.New()
	team, otherTeam := uuid.New(), uuid.New()
	num, otherNum := uuid.New(), uuid.New()
	tests := []struct {
		name     string
		a        authctx.Access
		team     *uuid.UUID
		assignee *uuid.UUID
		channel  string
		number   uuid.UUID
		want     bool
	}{
		{"owner sees all", authctx.Access{UserID: me, Owner: true, Scope: authctx.ScopeAssigned, Numbers: []uuid.UUID{otherNum}}, nil, &other, "", num, true},
		{"all scope", authctx.Access{UserID: me, Scope: authctx.ScopeAll}, nil, nil, "", num, true},
		{"all scope, other number", authctx.Access{UserID: me, Scope: authctx.ScopeAll, Numbers: []uuid.UUID{otherNum}}, nil, nil, "", num, false},
		{"team scope, my team", authctx.Access{UserID: me, Scope: authctx.ScopeTeam, Teams: []uuid.UUID{team}}, &team, &other, "", num, true},
		{"team scope, other team", authctx.Access{UserID: me, Scope: authctx.ScopeTeam, Teams: []uuid.UUID{team}}, &otherTeam, nil, "", num, false},
		{"team scope, no team", authctx.Access{UserID: me, Scope: authctx.ScopeTeam, Teams: []uuid.UUID{team}}, nil, nil, "", num, false},
		{"team scope, assigned to me", authctx.Access{UserID: me, Scope: authctx.ScopeTeam}, &otherTeam, &me, "", num, true},
		{"assigned scope, mine", authctx.Access{UserID: me, Scope: authctx.ScopeAssigned}, nil, &me, "", num, true},
		{"assigned scope, someone else's", authctx.Access{UserID: me, Scope: authctx.ScopeAssigned}, &team, &other, "", num, false},
		{"assigned to me on another number", authctx.Access{UserID: me, Scope: authctx.ScopeAssigned, Numbers: []uuid.UUID{otherNum}}, nil, &me, "", num, false},
		// Numbers restrict only WhatsApp chats until team access covers channels (ADR 0010).
		{"numbers don't restrict another channel", authctx.Access{UserID: me, Scope: authctx.ScopeAll, Numbers: []uuid.UUID{otherNum}}, nil, nil, model.ChannelMessenger, num, true},
	}
	for _, tt := range tests {
		channel := tt.channel
		if channel == "" {
			channel = model.ChannelWhatsApp
		}
		if got := canSee(tt.a, tt.team, tt.assignee, channel, tt.number); got != tt.want {
			t.Errorf("%s: canSee = %v", tt.name, got)
		}
	}
}

func TestCanSeeEvent(t *testing.T) {
	me := uuid.New()
	num := uuid.New()
	a := authctx.Access{UserID: me, Scope: authctx.ScopeAll}
	if !canSeeEvent(a, realtime.Event{Kind: model.SignalMessage, ChannelType: model.ChannelWhatsApp, ChannelID: &num}) {
		t.Error("message in scope should pass")
	}
	other := uuid.New()
	if canSeeEvent(a, realtime.Event{Kind: model.SignalMention, ChannelType: model.ChannelWhatsApp, ChannelID: &num, UserID: &other}) {
		t.Error("someone else's mention must not pass")
	}
	if !canSeeEvent(a, realtime.Event{Kind: model.SignalMention, ChannelType: model.ChannelWhatsApp, ChannelID: &num, UserID: &me}) {
		t.Error("my mention should pass")
	}
	if canSeeEvent(a, realtime.Event{Kind: model.SignalMessage}) {
		t.Error("an event without a channel reaches only the owner")
	}
}

func TestPartsOf(t *testing.T) {
	me, t1, t2 := uuid.New(), uuid.New(), uuid.New()
	if p := partsOf(authctx.Access{Owner: true, Scope: authctx.ScopeAssigned}); len(p) != 1 || p[0].teamID != nil || p[0].assigneeID != nil {
		t.Errorf("owner: %+v", p)
	}
	p := partsOf(authctx.Access{UserID: me, Scope: authctx.ScopeTeam, Teams: []uuid.UUID{t1, t2}})
	if len(p) != 3 || *p[0].teamID != t1 || *p[1].teamID != t2 || *p[2].assigneeID != me || len(p[2].outsideTeams) != 2 {
		t.Errorf("team: %+v", p)
	}
	if p := partsOf(authctx.Access{UserID: me, Scope: authctx.ScopeAssigned}); len(p) != 1 || *p[0].assigneeID != me {
		t.Errorf("assigned: %+v", p)
	}
}

func TestCompareRowsWaitingPutsNotWaitingLast(t *testing.T) {
	old, recent := time.Now().Add(-time.Hour), time.Now()
	a := model.Conversation{ID: uuid.New(), WaitingSince: &recent}
	b := model.Conversation{ID: uuid.New(), WaitingSince: &old}
	c := model.Conversation{ID: uuid.New()}
	if compareRows(b, a, true) >= 0 || compareRows(a, c, true) >= 0 || compareRows(c, b, true) <= 0 {
		t.Error("waiting order: oldest first, not waiting last")
	}
	x := model.Conversation{ID: uuid.New(), LastMessageAt: recent}
	y := model.Conversation{ID: uuid.New(), LastMessageAt: old}
	if compareRows(x, y, false) >= 0 {
		t.Error("newest order: latest first")
	}
}

func TestTemplateVariables(t *testing.T) {
	if n := countVariables("Hi {{1}}, order {{2}} and {{ 2 }}"); n != 2 {
		t.Errorf("countVariables = %d", n)
	}
	if got := render("Hi {{1}}, order {{2}}", []string{"Ada", "#7"}); got != "Hi Ada, order #7" {
		t.Errorf("render = %q", got)
	}
	o := option(model.Template{
		HeaderType: "text", HeaderText: "Order {{1}}", Body: "Hi {{1}}",
		Buttons: []model.TemplateButton{{Type: "quick_reply", Text: "Ok"}, {Type: "url", Value: "https://x.test/{{1}}"}},
	})
	if !o.HeaderVariable || o.BodyVariables != 1 || len(o.URLButtons) != 1 || o.URLButtons[0] != 1 || o.HeaderMedia {
		t.Errorf("option = %+v", o)
	}
}

func TestUploadKeys(t *testing.T) {
	ws := uuid.New()
	key := uploadKey(ws, uuid.New(), 2048, safeName("../My Invoice (1).PDF"))
	up, ok := parseUploadKey(ws, key)
	if !ok || up.Kind != model.TypeDocument || up.Mime != "application/pdf" || up.Size != 2048 || up.Filename != "My_Invoice__1_.PDF" {
		t.Fatalf("parse %q: %+v %v", key, up, ok)
	}
	if _, ok := parseUploadKey(uuid.New(), key); ok {
		t.Error("another workspace's key must not parse")
	}
	if _, ok := parseUploadKey(ws, uploadKey(ws, uuid.New(), MaxUploadBytes+1, "file.pdf")); ok {
		t.Error("a key over the upload limit must not parse")
	}
	if _, ok := parseUploadKey(ws, strings.Replace(key, "/uploads/", "/other/", 1)); ok {
		t.Error("a key outside uploads must not parse")
	}
}

func TestPreview(t *testing.T) {
	if got := preview(model.TypeImage, ""); got != "Photo" {
		t.Errorf("image preview = %q", got)
	}
	long := strings.Repeat("ab ", 100)
	if got := preview(model.TypeText, long); len([]rune(got)) != model.MaxPreview {
		t.Errorf("preview length = %d", len([]rune(got)))
	}
	if got := preview(model.TypeText, "two\n\nlines"); got != "two lines" {
		t.Errorf("preview = %q", got)
	}
}
