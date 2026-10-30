package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database/dbtest"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// fakeChannel is a channel with the given capabilities that records its sends.
type fakeChannel struct {
	caps model.Capabilities
	mu   sync.Mutex
	sent []string
}

func (f *fakeChannel) record(s string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, s)
	return fmt.Sprintf("mid.%d", len(f.sent)), nil
}

func (f *fakeChannel) Capabilities() model.Capabilities { return f.caps }

func (f *fakeChannel) Accounts(context.Context, uuid.UUID) (map[uuid.UUID]model.ChannelAccount, error) {
	return map[uuid.UUID]model.ChannelAccount{testPage: {Name: "Acme", Handle: "@acme"}}, nil
}

func (f *fakeChannel) SendText(_ context.Context, _, _ uuid.UUID, to, body, _ string) (string, error) {
	return f.record("text:" + to + ":" + body)
}

func (f *fakeChannel) SendMedia(_ context.Context, _, _ uuid.UUID, to string, m model.OutboundMedia, _ string) (string, error) {
	return f.record("media:" + to + ":" + m.Kind)
}

func (f *fakeChannel) SendTemplate(_ context.Context, _, _ uuid.UUID, to string, t model.OutboundTemplate) (string, error) {
	return f.record("template:" + to + ":" + t.Name)
}

func (f *fakeChannel) SendReaction(_ context.Context, _, _ uuid.UUID, to, target, emoji string) (string, error) {
	return f.record("reaction:" + to + ":" + target + ":" + emoji)
}

func (f *fakeChannel) MarkRead(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }

func (f *fakeChannel) UploadMedia(context.Context, uuid.UUID, uuid.UUID, string, string, io.Reader, int64) (string, error) {
	return "media-1", nil
}

func (f *fakeChannel) FetchMedia(context.Context, uuid.UUID, uuid.UUID, string) (io.ReadCloser, string, int64, error) {
	return io.NopCloser(strings.NewReader("")), "", 0, nil
}

// fakeContacts resolves every sender to one contact.
type fakeContacts struct{ id uuid.UUID }

func (c fakeContacts) FindOrCreateByIdentity(context.Context, uuid.UUID, string, string, string, string) (model.Contact, bool, error) {
	return model.Contact{ID: c.id}, false, nil
}

func (fakeContacts) SetLastMessage(context.Context, pgx.Tx, uuid.UUID, uuid.UUID, time.Time) error {
	return nil
}

// fakeQueue records the kinds of the jobs it is given.
type fakeQueue struct {
	mu    sync.Mutex
	kinds []string
}

func (q *fakeQueue) InsertTx(_ context.Context, _ pgx.Tx, args river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.kinds = append(q.kinds, args.Kind())
	return &rivertype.JobInsertResult{}, nil
}

func (q *fakeQueue) count(kind string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for _, k := range q.kinds {
		if k == kind {
			n++
		}
	}
	return n
}

var testPage = uuid.MustParse("01900000-0000-7000-8000-000000000001")

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	ae, ok := apperr.As(err)
	if !ok || ae.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}

const mib = 1 << 20

func whatsappLike() model.Capabilities {
	return model.Capabilities{
		Window: 24 * time.Hour, Templates: true, Reactions: true, ReadReceipts: true, MaxTextLen: 4096,
		Media: []model.MediaKind{
			{Kind: model.TypeImage, MaxBytes: 5 * mib, MimeTypes: []string{"image/jpeg", "image/png"}},
			{Kind: model.TypeDocument, MaxBytes: 100 * mib},
		},
	}
}

// A second channel type with no window, templates, reactions or read receipts:
// every rule that differed by channel follows its capabilities (ADR 0010).
func TestChannelCapabilitiesDriveTheRules(t *testing.T) {
	pool := dbtest.New(t)
	ctx := t.Context()
	ws, user, contact := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO workspaces (id, name) VALUES ($1, 'Channels')`, []any{ws}},
		{`INSERT INTO users (id, email) VALUES ($1, $2)`, []any{user, user.String() + "@example.test"}},
		{`INSERT INTO contacts (id, workspace_id, name, phone) VALUES ($1, $2, 'Ada', $3)`, []any{contact, ws, "+447700900111"}},
		{`INSERT INTO contact_identities (workspace_id, contact_id, channel_type, external_id) VALUES ($1, $2, 'messenger', 'psid-1')`, []any{ws, contact}},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	messenger := &fakeChannel{caps: model.Capabilities{
		MaxTextLen: 2000,
		Media:      []model.MediaKind{{Kind: model.TypeImage, MaxBytes: 8 * mib}},
	}}
	queue := &fakeQueue{}
	s := New(pool, queue, nil, Deps{
		Contacts: fakeContacts{id: contact},
		Channels: Channels{model.ChannelMessenger: messenger, model.ChannelWhatsApp: &fakeChannel{caps: whatsappLike()}},
	}, nil)
	owner := authctx.Access{WorkspaceID: ws, UserID: user, Owner: true}

	if err := s.InboundMessage(ctx, model.Inbound{
		WorkspaceID: ws, ChannelType: model.ChannelMessenger, ChannelID: testPage, ExternalID: "m.in.1",
		Sender: model.Sender{ExternalID: "psid-1", Name: "Ada"}, Timestamp: time.Now().UTC(), Type: "text", Text: "Hi",
	}); err != nil {
		t.Fatal(err)
	}
	page, err := s.List(ctx, owner, model.ListFilter{Tab: model.TabOpen})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("list: %+v %v", page.Items, err)
	}
	conv := page.Items[0]
	if conv.Channel != (model.ChannelRef{Type: model.ChannelMessenger, ID: testPage, Name: "Acme", Handle: "@acme"}) ||
		conv.WindowExpiresAt != nil {
		t.Fatalf("conversation: %+v", conv)
	}
	detail, err := s.Get(ctx, owner, conv.ID)
	if err != nil || detail.Capabilities.Window != 0 || detail.Capabilities.Templates {
		t.Fatalf("detail: %+v %v", detail.Capabilities, err)
	}

	// No window: a free-form message is allowed, and it goes to the sender's channel ID.
	msg, err := s.Send(ctx, owner, conv.ID, model.SendInput{Type: model.SendText, Text: "Hello"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Deliver(ctx, ws, msg.ID, false); err != nil {
		t.Fatal(err)
	}
	if len(messenger.sent) != 1 || messenger.sent[0] != "text:psid-1:Hello" {
		t.Fatalf("sent: %v", messenger.sent)
	}
	_, err = s.Send(ctx, owner, conv.ID, model.SendInput{Type: model.SendText, Text: strings.Repeat("a", 2001)})
	wantCode(t, err, "invalid_input")

	// No templates, no reactions, no read receipts.
	tpl := uuid.Must(uuid.NewV7())
	_, err = s.Send(ctx, owner, conv.ID, model.SendInput{Type: model.SendTemplate, TemplateID: &tpl})
	wantCode(t, err, "templates_unsupported")
	_, err = s.Templates(ctx, ws, model.ChannelMessenger)
	wantCode(t, err, "templates_unsupported")
	wantCode(t, s.React(ctx, owner, conv.ID, msg.ID, "👍"), "reactions_unsupported")
	if err := s.MarkRead(ctx, owner, conv.ID); err != nil {
		t.Fatal(err)
	}
	if n := queue.count(model.MarkReadArgs{}.Kind()); n != 0 {
		t.Fatalf("%d read receipts queued on a channel without them", n)
	}

	// A channel type with no adapter can't send.
	if _, err := pool.Exec(ctx, `UPDATE conversations SET channel_type = 'instagram' WHERE id = $1`, conv.ID); err != nil {
		t.Fatal(err)
	}
	_, err = s.Send(ctx, owner, conv.ID, model.SendInput{Type: model.SendText, Text: "Hello"})
	wantCode(t, err, "channel_unavailable")
}

func TestMediaLimitsComeFromChannels(t *testing.T) {
	messenger := model.Capabilities{Media: []model.MediaKind{{Kind: model.TypeImage, MaxBytes: 8 * mib}}}
	s := New(nil, nil, nil, Deps{Channels: Channels{
		model.ChannelWhatsApp: &fakeChannel{caps: whatsappLike()}, model.ChannelMessenger: &fakeChannel{caps: messenger},
	}}, nil)
	// An upload is checked against the largest limit of any channel …
	if n, ok := s.uploadLimit(model.TypeImage, "image/png"); !ok || n != 8*mib {
		t.Fatalf("image limit: %d %v", n, ok)
	}
	if _, ok := s.uploadLimit(model.TypeVideo, "video/mp4"); ok {
		t.Fatal("no channel takes video here")
	}
	// … and again against the conversation's channel at send.
	png := model.Upload{Kind: model.TypeImage, Mime: "image/png", Size: 6 * mib}
	wantCode(t, checkMedia(whatsappLike(), png), "media_too_large")
	if err := checkMedia(messenger, png); err != nil {
		t.Fatal(err)
	}
	wantCode(t, checkMedia(messenger, model.Upload{Kind: model.TypeDocument, Mime: "application/pdf", Size: 1}), "media_type_not_allowed")
	if err := checkMedia(whatsappLike(), model.Upload{Kind: model.TypeDocument, Mime: "application/pdf", Size: 50 * mib}); err != nil {
		t.Fatalf("any document type within 100 MB: %v", err)
	}
}
