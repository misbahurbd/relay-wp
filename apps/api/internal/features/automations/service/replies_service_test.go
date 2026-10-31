package service

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
)

func TestIsStop(t *testing.T) {
	for text, want := range map[string]bool{
		"STOP": true, "stop": true, " Stop! ": true, "unsubscribe": true,
		"please stop": false, "stop sending": false, "": false,
	} {
		if got := isStop(text); got != want {
			t.Errorf("isStop(%q) = %v", text, got)
		}
	}
}

func TestMatchRule(t *testing.T) {
	rules := []model.KeywordRule{
		{Keywords: "price, cost, how much", Match: model.MatchAny, Reply: "catalog"},
		{Keywords: "track my order", Match: model.MatchExact, Reply: "tracking"},
		{Keywords: "দাম", Match: model.MatchAny, Reply: "bangla"},
	}
	cases := []struct{ text, want string }{
		{"What's the PRICE?", "catalog"},
		{"how much is it", "catalog"},
		{"priceless art", ""},
		{"please track my order now", "tracking"},
		{"track the order", ""},
		{"এর দাম কত", "bangla"},
	}
	for _, c := range cases {
		r, ok := matchRule(rules, c.text)
		if ok != (c.want != "") || r.Reply != c.want {
			t.Errorf("%q: got %q, %v", c.text, r.Reply, ok)
		}
	}
}

func TestIsOpen(t *testing.T) {
	s := &model.AwaySettings{Timezone: "Asia/Dhaka", Hours: []model.WorkingDay{
		{Day: "sat", Open: true, From: "10:00", To: "20:00"},
		{Day: "sun", Open: true, From: "10:00", To: "20:00"},
		{Day: "mon", Open: true, From: "10:00", To: "20:00"},
		{Day: "tue", Open: true, From: "10:00", To: "20:00"},
		{Day: "wed", Open: true, From: "10:00", To: "20:00"},
		{Day: "thu", Open: true, From: "10:00", To: "20:00"},
		{Day: "fri"},
	}}
	dhaka := func(day, hour int) time.Time {
		return time.Date(2026, 10, day, hour, 0, 0, 0, time.FixedZone("BDT", 6*3600))
	}
	// 2026-10-28 is a Wednesday, 30 a Friday.
	if open, _ := isOpen(s, dhaka(28, 12)); !open {
		t.Error("Wednesday noon should be open")
	}
	_, night := isOpen(s, dhaka(28, 21))
	_, morning := isOpen(s, dhaka(29, 8))
	if night != morning || night != "2026-10-29" {
		t.Errorf("one closed night must be one period: %q, %q", night, morning)
	}
	if _, friday := isOpen(s, dhaka(30, 12)); friday != "2026-10-31" {
		t.Errorf("Friday waits for Saturday: %q", friday)
	}
}

func TestCandidatesInPriorityOrder(t *testing.T) {
	night := time.Date(2026, 10, 28, 18, 0, 0, 0, time.UTC)
	on := map[model.Kind]model.Settings{
		model.KindKeyword: &model.KeywordSettings{Rules: []model.KeywordRule{{Keywords: "price", Match: model.MatchAny, Reply: "k"}}},
		model.KindAway:    model.Default(model.KindAway),
		model.KindWelcome: model.Default(model.KindWelcome),
	}
	on[model.KindAway].(*model.AwaySettings).Timezone = "Asia/Dhaka"
	kinds := func(in model.Inbound, enabled map[model.Kind]model.Settings) []model.Kind {
		var out []model.Kind
		for _, c := range candidates(enabled, in, night) {
			out = append(out, c.kind)
		}
		return out
	}
	id := uuid.New()
	in := model.Inbound{ConversationID: id, MessageID: id, Text: "price?", NewConversation: true}
	if got := kinds(in, on); len(got) != 3 || got[0] != model.KindKeyword || got[1] != model.KindAway || got[2] != model.KindWelcome {
		t.Fatalf("got %v", got)
	}
	in.Text = "hello"
	if got := kinds(in, on); len(got) != 2 || got[0] != model.KindAway {
		t.Fatalf("no keyword: got %v", got)
	}
	delete(on, model.KindAway)
	in.NewConversation = false
	if got := kinds(in, on); len(got) != 0 {
		t.Fatalf("an old chat gets no welcome: %v", got)
	}
	in.Returning = true
	if got := kinds(in, on); len(got) != 0 {
		t.Fatalf("first-time only: %v", got)
	}
	on[model.KindWelcome].(*model.WelcomeSettings).FirstTimeOnly = false
	if got := kinds(in, on); len(got) != 1 {
		t.Fatalf("returning contact: %v", got)
	}
}
