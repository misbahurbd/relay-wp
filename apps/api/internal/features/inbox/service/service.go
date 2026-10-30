// Package service holds the inbox's business rules (no HTTP, no SQL): ingesting
// customer messages and statuses, sending through an outbox, notes and mentions,
// assignment and status actions, quick replies, media and live-update signals
// (docs/PHASE-7-INBOX.md, slice A). It is channel-neutral: each channel plugs in
// through the Channel registry and its Capabilities (ADR 0010).
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/realtime"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/storage"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// ContactDirectory finds the customer a message is from. Implemented by features/contacts.
type ContactDirectory interface {
	// FindOrCreateByIdentity returns the contact a channel's sender is (ADR 0010):
	// by its identity, else by phone, creating it (source inbox) if new.
	FindOrCreateByIdentity(ctx context.Context, workspaceID uuid.UUID, channelType, externalID, name, phone string) (model.Contact, bool, error)
	// SetLastMessage records a message to or from the contact, in tx.
	SetLastMessage(ctx context.Context, tx pgx.Tx, workspaceID, contactID uuid.UUID, at time.Time) error
}

// Channel is one messaging channel (ADR 0010): WhatsApp today, Messenger and
// Instagram later. internal/app implements it on top of the channel's feature.
// channelID is the provider's account row the conversation is on, and to is the
// customer's ID on the channel. Sends return the channel's message ID; a refusal
// that must not be retried comes back as *model.SendFailure with Permanent set.
type Channel interface {
	Capabilities() model.Capabilities
	// Accounts names the workspace's accounts on this channel, by channel ID.
	Accounts(ctx context.Context, workspaceID uuid.UUID) (map[uuid.UUID]model.ChannelAccount, error)
	SendText(ctx context.Context, workspaceID, channelID uuid.UUID, to, body, replyTo string) (string, error)
	SendMedia(ctx context.Context, workspaceID, channelID uuid.UUID, to string, m model.OutboundMedia, replyTo string) (string, error)
	SendTemplate(ctx context.Context, workspaceID, channelID uuid.UUID, to string, t model.OutboundTemplate) (string, error)
	SendReaction(ctx context.Context, workspaceID, channelID uuid.UUID, to, targetExternalID, emoji string) (string, error)
	MarkRead(ctx context.Context, workspaceID, channelID uuid.UUID, externalID string) error
	UploadMedia(ctx context.Context, workspaceID, channelID uuid.UUID, contentType, filename string, data io.Reader, size int64) (string, error)
	FetchMedia(ctx context.Context, workspaceID, channelID uuid.UUID, mediaID string) (io.ReadCloser, string, int64, error)
}

// Channels is the channel registry, keyed by channel type and filled once at
// startup in internal/app (an approved factory point, ADR 0010). A conversation's
// channel_type picks its adapter.
type Channels map[string]Channel

// TemplateCatalog lists the workspace's approved templates. Implemented by features/templates.
type TemplateCatalog interface {
	ApprovedTemplates(ctx context.Context, workspaceID uuid.UUID) ([]model.Template, error)
	// ApprovedTemplate returns model.ErrTemplateNotApproved unless id is an approved template.
	ApprovedTemplate(ctx context.Context, workspaceID, id uuid.UUID) (model.Template, error)
}

// EventPublisher records an Integrations webhook event in tx. Implemented by features/integrations.
type EventPublisher interface {
	Publish(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, eventType string, data any) error
}

// MemberDirectory resolves another member's access, for mentions and assignment.
// Implemented by features/team.
type MemberDirectory interface {
	ResolveAccess(ctx context.Context, workspaceID, userID uuid.UUID) (a authctx.Access, suspended, ok bool, err error)
}

// Subscriber hands out live-update subscriptions. Implemented by platform/realtime.Hub.
type Subscriber interface {
	Subscribe(workspaceID uuid.UUID, filter func(realtime.Event) bool) (<-chan realtime.Event, func())
}

// Enqueuer inserts background jobs in a transaction.
type Enqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Deps are the service's collaborators, wired in internal/app.
type Deps struct {
	Contacts  ContactDirectory
	Channels  Channels
	Templates TemplateCatalog
	Events    EventPublisher
	Members   MemberDirectory
}

// Service runs the team inbox.
type Service struct {
	pool  *pgxpool.Pool
	queue Enqueuer
	store storage.Storage
	deps  Deps
	live  Subscriber
	log   *slog.Logger
	now   func() time.Time
}

// New builds the service.
func New(pool *pgxpool.Pool, queue Enqueuer, store storage.Storage, deps Deps, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		pool: pool, queue: queue, store: store, deps: deps, log: log,
		now: func() time.Time { return time.Now().UTC() },
	}
}

// SetSubscriber turns on live updates: the API process passes its realtime hub.
func (s *Service) SetSubscriber(live Subscriber) { s.live = live }

// channel returns the adapter for a channel type, or false when none is registered.
func (s *Service) channel(channelType string) (Channel, bool) {
	c, ok := s.deps.Channels[channelType]
	return c, ok && c != nil
}

// ---------- Scope (AD-10, PHASE-3 §5) ----------

// numbersOf is the numbers filter for a member: nil means every number. The
// owner sees every number, and so does a member with none ticked ("none ticked
// means every number their teams use"; their scope still limits the chats).
func numbersOf(a authctx.Access) []uuid.UUID {
	if a.Owner || len(a.Numbers) == 0 {
		return nil
	}
	return a.Numbers
}

// onNumbers reports whether a chat on this channel passes a numbers filter (nil:
// every number). Until team access covers channels, a member's numbers are their
// WhatsApp channels, and other channel types aren't restricted (ADR 0010).
func onNumbers(nums []uuid.UUID, channelType string, channelID uuid.UUID) bool {
	return nums == nil || channelType != model.ChannelWhatsApp || slices.Contains(nums, channelID)
}

// canSee reports whether a member may open a chat with these team, assignee and
// channel. Chats assigned to the member are always in their scope.
func canSee(a authctx.Access, teamID, assigneeID *uuid.UUID, channelType string, channelID uuid.UUID) bool {
	if a.Owner {
		return true
	}
	if !onNumbers(numbersOf(a), channelType, channelID) {
		return false
	}
	if assigneeID != nil && *assigneeID == a.UserID {
		return true
	}
	switch a.Scope {
	case authctx.ScopeAll:
		return true
	case authctx.ScopeTeam:
		return teamID != nil && slices.Contains(a.Teams, *teamID)
	default:
		return false
	}
}

// canSeeEvent applies canSee to a live-update signal; a mention reaches only its member.
func canSeeEvent(a authctx.Access, ev realtime.Event) bool {
	if ev.Kind == model.SignalMention {
		return ev.UserID != nil && *ev.UserID == a.UserID
	}
	if ev.ChannelID == nil {
		return a.Owner
	}
	return canSee(a, ev.TeamID, ev.AssigneeID, ev.ChannelType, *ev.ChannelID)
}

// hasInboxRole reports whether a member works in the inbox at all (not a Viewer).
func hasInboxRole(a authctx.Access) bool {
	return a.Can(authctx.PermInboxReply) || a.Can(authctx.PermInboxNotes) ||
		a.Can(authctx.PermInboxSendTemplate) || a.Can(authctx.PermInboxCloseReassign)
}

// ---------- Signals ----------

// signal sends a live-update signal in tx (AD-8): ids only, never content.
func signal(ctx context.Context, tx pgx.Tx, ws uuid.UUID, kind string, conversationID uuid.UUID, teamID, assigneeID *uuid.UUID,
	channelType string, channelID uuid.UUID,
) error {
	return realtime.Notify(ctx, tx, realtime.InboxChannel, realtime.Event{
		WorkspaceID: ws, Kind: kind, ConversationID: &conversationID,
		TeamID: teamID, AssigneeID: assigneeID, ChannelType: channelType, ChannelID: &channelID,
	})
}

// publish records an Integrations event in tx.
func (s *Service) publish(ctx context.Context, tx pgx.Tx, ws uuid.UUID, eventType string, data any) error {
	if s.deps.Events == nil {
		return nil
	}
	if err := s.deps.Events.Publish(ctx, tx, ws, eventType, data); err != nil {
		return fmt.Errorf("publish %s: %w", eventType, err)
	}
	return nil
}

// ---------- Small helpers ----------

func newID() (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("new id: %w", err)
	}
	return id, nil
}

// person builds a member's name from a LEFT JOIN of users and user_profiles.
func person(id *uuid.UUID, display, first, last, email *string) *model.Person {
	if id == nil {
		return nil
	}
	p := &model.Person{ID: *id}
	switch {
	case display != nil && strings.TrimSpace(*display) != "":
		p.Name = strings.TrimSpace(*display)
	case first != nil && strings.TrimSpace(*first+" "+deref(last)) != "":
		p.Name = strings.TrimSpace(*first + " " + deref(last))
	default:
		p.Name = deref(email)
	}
	return p
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func decodeContent(raw []byte) (model.Content, error) {
	var c model.Content
	if len(raw) == 0 {
		return c, nil
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, fmt.Errorf("decode message content: %w", err)
	}
	return c, nil
}

func encodeContent(c model.Content) ([]byte, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("encode message content: %w", err)
	}
	return b, nil
}

// truncate cuts s to at most n characters.
func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

// preview is the list's one-line summary of a message (AD-2).
func preview(msgType, text string) string {
	if t := strings.Join(strings.Fields(text), " "); t != "" {
		return truncate(t, model.MaxPreview)
	}
	switch msgType {
	case model.TypeImage:
		return "Photo"
	case model.TypeVideo:
		return "Video"
	case model.TypeAudio:
		return "Audio"
	case model.TypeDocument:
		return "Document"
	case model.TypeSticker:
		return "Sticker"
	case "location":
		return "Location"
	case "contacts":
		return "Contact card"
	case model.TypeTemplate:
		return "Template"
	default:
		return "Message"
	}
}

func ptr[T any](v T) *T { return &v }
