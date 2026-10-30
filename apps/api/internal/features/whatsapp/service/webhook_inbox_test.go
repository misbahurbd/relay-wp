package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
)

type fakeSink struct {
	messages []model.InboundMessage
	statuses []model.StatusUpdate
	err      error
}

func (f *fakeSink) InboundMessage(_ context.Context, m model.InboundMessage) error {
	f.messages = append(f.messages, m)
	return f.err
}

func (f *fakeSink) MessageStatus(_ context.Context, s model.StatusUpdate) error {
	f.statuses = append(f.statuses, s)
	return nil
}

// A "messages" change value as Meta sends it.
const inboundValue = `{
  "messaging_product": "whatsapp",
  "metadata": {"display_phone_number": "15550001111", "phone_number_id": "1001"},
  "contacts": [{"profile": {"name": "Daniel"}, "wa_id": "14155550142"}],
  "messages": [
    {"from": "14155550142", "id": "wamid.T", "timestamp": "1761800000", "type": "text",
     "text": {"body": "Hi, is the blue one in stock?"},
     "referral": {"source_url": "https://fb.me/abc", "source_id": "1203", "source_type": "ad",
                  "headline": "Summer sale", "body": "20% off", "media_type": "image", "ctwa_clid": "x"}},
    {"from": "14155550142", "id": "wamid.I", "timestamp": "1761800001", "type": "image",
     "context": {"from": "15550001111", "id": "wamid.OUT1"},
     "image": {"caption": "This one", "mime_type": "image/jpeg", "sha256": "abc=", "id": "media.1"}},
    {"from": "14155550142", "id": "wamid.B", "timestamp": "1761800002", "type": "interactive",
     "interactive": {"type": "button_reply", "button_reply": {"id": "yes", "title": "Yes please"}}},
    {"from": "14155550142", "id": "wamid.R", "timestamp": "1761800003", "type": "reaction",
     "reaction": {"message_id": "wamid.OUT1", "emoji": "❤️"}},
    {"from": "14155550142", "id": "wamid.L", "timestamp": "1761800004", "type": "location",
     "location": {"latitude": 23.81, "longitude": 90.41, "name": "Office", "address": "Dhaka"}},
    {"from": "14155550142", "id": "wamid.Q", "timestamp": "1761800005", "type": "button",
     "button": {"text": "Stop promos", "payload": "STOP"}},
    {"from": "14155550142", "id": "wamid.U", "timestamp": "1761800006", "type": "unsupported",
     "errors": [{"code": 131051, "title": "Message type unknown"}]}
  ],
  "statuses": [
    {"id": "wamid.OUT1", "status": "delivered", "timestamp": "1761800010", "recipient_id": "14155550142"},
    {"id": "wamid.OUT2", "status": "failed", "timestamp": "1761800011", "recipient_id": "14155550142",
     "errors": [{"code": 131047, "title": "Re-engagement message"}]},
    {"id": "wamid.TEST", "status": "read", "timestamp": "1761800012", "recipient_id": "14155550142"},
    {"id": "wamid.OUT3", "status": "deleted", "timestamp": "1761800013", "recipient_id": "14155550142"}
  ]
}`

func TestHandleMessagesHandsOffToSink(t *testing.T) {
	sink := &fakeSink{}
	s := &Service{inbound: sink}
	acc := account{ID: uuid.New(), WorkspaceID: uuid.New()}
	numberID := uuid.New()
	var tested []string
	db := messageLookups{
		number: func(_ context.Context, a account, phoneNumberID string) (uuid.UUID, bool, error) {
			if a != acc || phoneNumberID != "1001" {
				t.Fatalf("number lookup %v %q", a, phoneNumberID)
			}
			return numberID, true, nil
		},
		testStatus: func(_ context.Context, _ account, st metaStatus) (bool, error) {
			tested = append(tested, st.ID)
			return st.ID == "wamid.TEST", nil
		},
	}
	if err := s.handleMessages(t.Context(), acc, json.RawMessage(inboundValue), db); err != nil {
		t.Fatal(err)
	}

	if len(sink.messages) != 7 {
		t.Fatalf("got %d messages", len(sink.messages))
	}
	for _, m := range sink.messages {
		if m.WorkspaceID != acc.WorkspaceID || m.NumberID != numberID || m.PhoneNumberID != "1001" ||
			m.From != "+14155550142" || m.ProfileName != "Daniel" || len(m.Raw) == 0 {
			t.Fatalf("envelope %+v", m)
		}
	}
	text, img, btn, react, loc, quick, unsup := sink.messages[0], sink.messages[1], sink.messages[2],
		sink.messages[3], sink.messages[4], sink.messages[5], sink.messages[6]

	if text.WAMID != "wamid.T" || text.Type != "text" || text.Text != "Hi, is the blue one in stock?" ||
		!text.Timestamp.Equal(time.Unix(1761800000, 0)) {
		t.Errorf("text %+v", text)
	}
	if r := text.Referral; r == nil || *r != (model.Referral{SourceType: "ad", SourceID: "1203", SourceURL: "https://fb.me/abc", Headline: "Summer sale", Body: "20% off"}) {
		t.Errorf("referral %+v", text.Referral)
	}
	if img.Text != "This one" || img.ContextWAMID != "wamid.OUT1" || img.Media == nil ||
		*img.Media != (model.InboundMedia{ID: "media.1", MimeType: "image/jpeg", SHA256: "abc="}) {
		t.Errorf("image %+v %+v", img, img.Media)
	}
	if btn.Text != "Yes please" || btn.Interactive == nil ||
		*btn.Interactive != (model.InteractiveReply{Kind: "button_reply", ID: "yes", Title: "Yes please"}) {
		t.Errorf("interactive %+v", btn.Interactive)
	}
	if react.Reaction == nil || *react.Reaction != (model.Reaction{WAMID: "wamid.OUT1", Emoji: "❤️"}) || react.Text != "" {
		t.Errorf("reaction %+v", react.Reaction)
	}
	if loc.Location == nil || *loc.Location != (model.Location{Latitude: 23.81, Longitude: 90.41, Name: "Office", Address: "Dhaka"}) {
		t.Errorf("location %+v", loc.Location)
	}
	if quick.Text != "Stop promos" || quick.Interactive == nil ||
		*quick.Interactive != (model.InteractiveReply{Kind: "button", ID: "STOP", Title: "Stop promos"}) {
		t.Errorf("quick reply %+v", quick.Interactive)
	}
	if unsup.Type != "unsupported" || unsup.Text != "" {
		t.Errorf("unsupported %+v", unsup)
	}
	var raw map[string]any
	if err := json.Unmarshal(unsup.Raw, &raw); err != nil || raw["errors"] == nil {
		t.Errorf("raw %s", unsup.Raw)
	}

	// The "deleted" status is skipped; the test message's stops at testStatus.
	if want := []string{"wamid.OUT1", "wamid.OUT2", "wamid.TEST"}; len(tested) != 3 || tested[0] != want[0] || tested[2] != want[2] {
		t.Errorf("test-status checks %v; want %v", tested, want)
	}
	if len(sink.statuses) != 2 {
		t.Fatalf("got %d statuses", len(sink.statuses))
	}
	want := []model.StatusUpdate{
		{
			WorkspaceID: acc.WorkspaceID, NumberID: numberID, WAMID: "wamid.OUT1", Status: "delivered",
			Timestamp: time.Unix(1761800010, 0).UTC(), RecipientID: "14155550142",
		},
		{
			WorkspaceID: acc.WorkspaceID, NumberID: numberID, WAMID: "wamid.OUT2", Status: "failed",
			Timestamp: time.Unix(1761800011, 0).UTC(), RecipientID: "14155550142", ErrorCode: 131047, ErrorTitle: "Re-engagement message",
		},
	}
	for i := range want {
		if sink.statuses[i] != want[i] {
			t.Errorf("status %d\n got %+v\nwant %+v", i, sink.statuses[i], want[i])
		}
	}
}

func TestHandleMessagesSinkErrorRetries(t *testing.T) {
	boom := errors.New("db down")
	s := &Service{inbound: &fakeSink{err: boom}}
	db := messageLookups{
		number:     func(context.Context, account, string) (uuid.UUID, bool, error) { return uuid.New(), true, nil },
		testStatus: func(context.Context, account, metaStatus) (bool, error) { return false, nil },
	}
	if err := s.handleMessages(t.Context(), account{}, json.RawMessage(inboundValue), db); !errors.Is(err, boom) {
		t.Fatalf("err = %v; want the sink's error so the job retries", err)
	}
}

func TestHandleMessagesUnknownNumber(t *testing.T) {
	sink := &fakeSink{}
	s := &Service{inbound: sink}
	tested := 0
	db := messageLookups{
		number: func(context.Context, account, string) (uuid.UUID, bool, error) { return uuid.Nil, false, nil },
		testStatus: func(context.Context, account, metaStatus) (bool, error) {
			tested++
			return false, nil
		},
	}
	if err := s.handleMessages(t.Context(), account{}, json.RawMessage(inboundValue), db); err != nil {
		t.Fatal(err)
	}
	if len(sink.messages) != 0 || len(sink.statuses) != 0 {
		t.Fatalf("sink got %d messages, %d statuses for an unknown number", len(sink.messages), len(sink.statuses))
	}
	if tested != 3 {
		t.Fatalf("test messages still get their statuses: %d checks", tested)
	}
}
