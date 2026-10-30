package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	inboxmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
)

// Local development without a real WhatsApp number: WHATSAPP_FAKE=true swaps the
// inbox's WhatsApp channel for devWhatsApp, and `relay dev seed-inbox` fills the
// inbox with sample chats through the normal ingest. Neither runs outside development.

// devChannelNamespace derives each workspace's fake number ID.
var devChannelNamespace = uuid.MustParse("6f1c2b8e-3d0a-4c1e-9b7a-5e2f8d4c1a90")

// devChannelID is the fake WhatsApp number of a workspace.
func devChannelID(workspaceID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(devChannelNamespace, workspaceID[:])
}

// devWhatsApp has WhatsApp's capabilities but sends nowhere: every send succeeds
// and Meta's delivered and read statuses arrive a moment later.
type devWhatsApp struct {
	whatsappChannel
	app *App
}

func (f *devWhatsApp) Accounts(_ context.Context, workspaceID uuid.UUID) (map[uuid.UUID]inboxmodel.ChannelAccount, error) {
	return map[uuid.UUID]inboxmodel.ChannelAccount{
		devChannelID(workspaceID): {Name: "Relay demo (fake)", Handle: "+1 555 010 0100"},
	}, nil
}

func (f *devWhatsApp) sent(workspaceID, channelID uuid.UUID) (string, error) {
	id := "fake." + uuid.NewString()
	go f.playStatuses(workspaceID, channelID, id)
	return id, nil
}

// playStatuses reports delivered after 1 s and read after 3 s, as a phone would.
func (f *devWhatsApp) playStatuses(workspaceID, channelID uuid.UUID, externalID string) {
	for _, step := range []struct {
		after  time.Duration
		status string
	}{{time.Second, "delivered"}, {2 * time.Second, "read"}} {
		time.Sleep(step.after)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := f.app.inbox.MessageStatus(ctx, inboxmodel.StatusUpdate{
			WorkspaceID: workspaceID, ChannelType: inboxmodel.ChannelWhatsApp, ChannelID: channelID,
			ExternalID: externalID, Status: step.status, Timestamp: time.Now().UTC(),
		})
		cancel()
		if err != nil && f.app.log != nil {
			f.app.log.Warn("fake whatsapp: status", "err", err)
		}
	}
}

func (f *devWhatsApp) SendText(_ context.Context, ws, ch uuid.UUID, _, _, _ string) (string, error) {
	return f.sent(ws, ch)
}

func (f *devWhatsApp) SendMedia(_ context.Context, ws, ch uuid.UUID, _ string, _ inboxmodel.OutboundMedia, _ string) (string, error) {
	return f.sent(ws, ch)
}

func (f *devWhatsApp) SendTemplate(_ context.Context, ws, ch uuid.UUID, _ string, _ inboxmodel.OutboundTemplate) (string, error) {
	return f.sent(ws, ch)
}

func (f *devWhatsApp) SendReaction(_ context.Context, _, _ uuid.UUID, _, _, _ string) (string, error) {
	return "fake." + uuid.NewString(), nil
}

func (f *devWhatsApp) MarkRead(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }

func (f *devWhatsApp) UploadMedia(_ context.Context, _, _ uuid.UUID, _, _ string, data io.Reader, _ int64) (string, error) {
	if _, err := io.Copy(io.Discard, data); err != nil {
		return "", fmt.Errorf("fake upload: %w", err)
	}
	return "fake-media." + uuid.NewString(), nil
}

func (f *devWhatsApp) FetchMedia(context.Context, uuid.UUID, uuid.UUID, string) (io.ReadCloser, string, int64, error) {
	return nil, "", 0, errors.New("fake whatsapp has no media to download")
}

// SeedInbox adds sample WhatsApp chats to a workspace through the normal ingest:
// a few open ones with growing waits, one from an ad, a location, a reaction and
// one whose 24-hour window has closed. Running it again adds nothing new.
func (a *App) SeedInbox(ctx context.Context, workspaceID uuid.UUID) (int, error) {
	if !a.cfg.IsDevelopment() {
		return 0, errors.New("seed-inbox runs only in development")
	}
	now := time.Now().UTC()
	ch := devChannelID(workspaceID)
	type sample struct {
		key, name, phone, text string
		ago                    time.Duration
		edit                   func(*inboxmodel.Inbound)
	}
	samples := []sample{
		{"daniel-1", "Daniel Brooks", "+14155550101", "Hi! Where is my order #4821? It said 2 days 🙈", 4 * time.Minute, nil},
		{"lucia-1", "Lucía Fernández", "+34612345678", "Hola, do you have the green jacket in size M?", 47 * time.Minute, nil},
		{"lucia-2", "Lucía Fernández", "+34612345678", "And can I pay cash on delivery?", 45 * time.Minute, nil},
		{"omar-1", "Omar Haddad", "+971501234567", "I want to return the shoes, they are too small", 2*time.Hour + 13*time.Minute, nil},
		{"priya-1", "Priya Sharma", "+919812345678", "Here's my address for the delivery", 3 * time.Hour, func(m *inboxmodel.Inbound) {
			m.Type, m.Text = "location", ""
			m.Location = &inboxmodel.Location{Latitude: 23.7808, Longitude: 90.4074, Name: "Gulshan 2", Address: "Road 90, Dhaka"}
		}},
		{"mei-1", "Mei Chen", "+8613812345678", "Saw your winter collection ad! Is the wool coat back in stock?", 20 * time.Minute, func(m *inboxmodel.Inbound) {
			m.Referral = &inboxmodel.Referral{SourceType: "ad", SourceID: "120208", Headline: "Winter collection"}
		}},
		{"jonas-1", "Jonas Weber", "+4915112345678", "Thanks, the package arrived 👍", 30 * time.Hour, nil},
	}
	added := 0
	for _, s := range samples {
		m := inboxmodel.Inbound{
			WorkspaceID: workspaceID, ChannelType: inboxmodel.ChannelWhatsApp, ChannelID: ch,
			ExternalID: "seed." + workspaceID.String()[:8] + "." + s.key,
			Sender:     inboxmodel.Sender{ExternalID: s.phone, Phone: s.phone, Name: s.name},
			Timestamp:  now.Add(-s.ago), Type: "text", Text: s.text,
		}
		if s.edit != nil {
			s.edit(&m)
		}
		if err := a.inbox.InboundMessage(ctx, m); err != nil {
			return added, fmt.Errorf("seed %s: %w", s.key, err)
		}
		added++
	}
	// Daniel reacts to his own question, the way customers do.
	if err := a.inbox.InboundMessage(ctx, inboxmodel.Inbound{
		WorkspaceID: workspaceID, ChannelType: inboxmodel.ChannelWhatsApp, ChannelID: ch,
		ExternalID: "seed." + workspaceID.String()[:8] + ".daniel-react",
		Sender:     inboxmodel.Sender{ExternalID: "+14155550101", Phone: "+14155550101", Name: "Daniel Brooks"},
		Timestamp:  now.Add(-3 * time.Minute), Type: "reaction",
		Reaction: &inboxmodel.InboundReaction{ExternalID: "seed." + workspaceID.String()[:8] + ".daniel-1", Emoji: "🙏"},
	}); err != nil {
		return added, fmt.Errorf("seed reaction: %w", err)
	}
	return added, nil
}
