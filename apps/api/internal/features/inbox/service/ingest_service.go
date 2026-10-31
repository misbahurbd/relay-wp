package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// errDuplicate rolls back an ingest whose external ID is already stored.
var errDuplicate = errors.New("inbox: message already stored")

// MessageReceived is the data of a message.received Integrations event.
type MessageReceived struct {
	ConversationID uuid.UUID `json:"conversation_id"`
	MessageID      uuid.UUID `json:"message_id"`
	ContactID      uuid.UUID `json:"contact_id"`
	// Channel is the channel type; SenderID is the sender's ID on it (the E.164 phone on WhatsApp).
	Channel  string `json:"channel"`
	SenderID string `json:"sender_id"`
	// Phone is the sender's E.164 number, when the channel has one.
	Phone string `json:"phone,omitempty"`
	Type  string `json:"type"`
	Text  string `json:"text"`
}

// MessageStatusChanged is the data of a message.status Integrations event.
type MessageStatusChanged struct {
	ConversationID uuid.UUID `json:"conversation_id"`
	MessageID      uuid.UUID `json:"message_id"`
	ContactID      uuid.UUID `json:"contact_id"`
	Channel        string    `json:"channel"`
	Status         string    `json:"status"`
	ErrorCode      string    `json:"error_code,omitempty"`
	ErrorTitle     string    `json:"error_title,omitempty"`
}

// InboundMessage stores a customer's message (AD-2, AD-3): the contact, the
// conversation, the message, its media job, the Integrations event and the
// live-update signal, all in one transaction. An external ID already stored
// changes nothing, so a redelivered webhook is harmless. The channel's window
// (its Capabilities) sets how long free-form replies are allowed.
func (s *Service) InboundMessage(ctx context.Context, in model.Inbound) error {
	if in.ExternalID == "" || in.WorkspaceID == uuid.Nil || in.ChannelID == uuid.Nil || in.Sender.ExternalID == "" {
		return fmt.Errorf("inbox: inbound message without external id, workspace, channel or sender")
	}
	ch, ok := s.channel(in.ChannelType)
	if !ok {
		return fmt.Errorf("inbox: inbound message on unregistered channel %q", in.ChannelType)
	}
	window := ch.Capabilities().Window
	at := in.Timestamp.UTC()
	if at.IsZero() || at.After(s.now().Add(5*time.Minute)) {
		at = s.now()
	}
	if in.Type == model.TypeReaction {
		return s.inboundReaction(ctx, in)
	}
	contact, _, err := s.deps.Contacts.FindOrCreateByIdentity(ctx, in.WorkspaceID, in.ChannelType, in.Sender.ExternalID,
		in.Sender.Name, in.Sender.Phone)
	if err != nil {
		return fmt.Errorf("inbox: find contact: %w", err)
	}
	content := model.Content{
		Location: in.Location, Contacts: in.Contacts, Interactive: in.Interactive, Referral: in.Referral,
	}
	if in.Media != nil {
		content.Media = &model.Media{ExternalID: in.Media.ID, Mime: in.Media.Mime, SHA256: in.Media.SHA256, Filename: in.Media.Filename}
	}
	raw, err := encodeContent(content)
	if err != nil {
		return err
	}
	source := model.SourceOrganic
	if in.Referral != nil {
		source = model.SourceCTWA
	}
	convID, err := newID()
	if err != nil {
		return err
	}
	msgID, err := newID()
	if err != nil {
		return err
	}
	var windowExpiresAt *time.Time
	if window > 0 {
		windowExpiresAt = ptr(at.Add(window))
	}
	ws := in.WorkspaceID
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		conv, err := r.UpsertInboundConversation(ctx, gen.UpsertInboundConversationParams{
			ID: convID, WorkspaceID: ws, ContactID: contact.ID, ChannelType: in.ChannelType, ChannelID: in.ChannelID,
			Source: source, At: at, Preview: preview(in.Type, in.Text), WindowExpiresAt: windowExpiresAt,
		})
		if err != nil {
			return fmt.Errorf("upsert conversation: %w", err)
		}
		var replyTo *uuid.UUID
		if in.ContextExternalID != "" {
			q, err := r.MessageByExternalID(ctx, gen.MessageByExternalIDParams{WorkspaceID: ws, ExternalID: &in.ContextExternalID})
			if err == nil && q.ConversationID == conv.ID {
				replyTo = &q.ID
			} else if err != nil && !repository.IsNoRows(err) {
				return fmt.Errorf("quoted message: %w", err)
			}
		}
		ids, err := r.InsertMessage(ctx, gen.InsertMessageParams{
			ID: msgID, WorkspaceID: ws, ConversationID: conv.ID, Direction: model.DirectionIn, Kind: model.KindMessage,
			Type: msgType(in.Type), Body: in.Text, Content: raw, ExternalID: &in.ExternalID, ReplyToID: replyTo,
			Mentions: []uuid.UUID{}, CreatedAt: at,
		})
		if err != nil {
			return fmt.Errorf("insert message: %w", err)
		}
		if len(ids) == 0 {
			return errDuplicate
		}
		if err := s.deps.Contacts.SetLastMessage(ctx, tx, ws, contact.ID, at); err != nil {
			return err
		}
		// A reply to a chat whose window had closed reopens it (F5).
		if window > 0 && !conv.Created && !at.Before(conv.PrevWindowExpiresAt) {
			if err := s.event(ctx, r, ws, conv.ID, model.EventContent{Action: model.ActionWindowReopened}); err != nil {
				return err
			}
		}
		if content.Media != nil && content.Media.ExternalID != "" {
			if _, err := s.queue.InsertTx(ctx, tx, model.FetchMediaArgs{WorkspaceID: ws, MessageID: msgID}, nil); err != nil {
				return fmt.Errorf("enqueue media fetch: %w", err)
			}
		}
		if err := s.publish(ctx, tx, ws, model.EventMessageReceived, MessageReceived{
			ConversationID: conv.ID, MessageID: msgID, ContactID: contact.ID, Channel: in.ChannelType,
			SenderID: in.Sender.ExternalID, Phone: in.Sender.Phone, Type: msgType(in.Type), Text: in.Text,
		}); err != nil {
			return err
		}
		if s.hook != nil {
			if err := s.hook.OnInbound(ctx, tx, model.InboundEvent{
				WorkspaceID: ws, ConversationID: conv.ID, ContactID: contact.ID, MessageID: msgID, Text: in.Text,
				NewConversation: conv.Created, Returning: window > 0 && !conv.Created && !at.Before(conv.PrevWindowExpiresAt),
			}); err != nil {
				return err
			}
		}
		return signal(ctx, tx, ws, model.SignalMessage, conv.ID, conv.TeamID, conv.AssigneeID, in.ChannelType, in.ChannelID)
	})
	if errors.Is(err, errDuplicate) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inbox: ingest %s: %w", in.ExternalID, err)
	}
	s.log.Debug("inbox: message stored", "workspace_id", ws, "external_id", in.ExternalID, "message_id", msgID)
	return nil
}

// msgType keeps the channel's type within the column's limit.
func msgType(t string) string {
	t = strings.TrimSpace(t)
	switch {
	case t == "":
		return "unsupported"
	case len(t) > 32:
		return t[:32]
	}
	return t
}

// inboundReaction puts the customer's emoji on the message it reacts to (AD-4);
// a reaction to a message we don't have is ignored.
func (s *Service) inboundReaction(ctx context.Context, in model.Inbound) error {
	if in.Reaction == nil || in.Reaction.ExternalID == "" {
		return nil
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		target, err := r.MessageByExternalID(ctx, gen.MessageByExternalIDParams{WorkspaceID: in.WorkspaceID, ExternalID: &in.Reaction.ExternalID})
		if repository.IsNoRows(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reaction target: %w", err)
		}
		rows, err := r.SetReaction(ctx, gen.SetReactionParams{
			Emoji: truncate(in.Reaction.Emoji, model.MaxEmojiLen), Side: model.DirectionIn,
			WorkspaceID: in.WorkspaceID, ID: target.ID,
		})
		if err != nil || len(rows) == 0 {
			return err
		}
		c := rows[0]
		return signal(ctx, tx, in.WorkspaceID, model.SignalMessage, c.ConversationID, c.TeamID, c.AssigneeID, c.ChannelType, c.ChannelID)
	})
}

// statuses a channel reports.
var statuses = []string{model.MessageSent, model.MessageDelivered, model.MessageRead, model.MessageFailed}

// MessageStatus moves an outbound message's tick forward (AD-7). A status for a
// message we don't have, or one that would move backwards, changes nothing.
func (s *Service) MessageStatus(ctx context.Context, u model.StatusUpdate) error {
	if !slices.Contains(statuses, u.Status) || u.ExternalID == "" {
		return nil
	}
	at := u.Timestamp.UTC()
	if at.IsZero() {
		at = s.now()
	}
	p := gen.AdvanceMessageStatusParams{Status: u.Status, At: at, WorkspaceID: u.WorkspaceID, ExternalID: u.ExternalID}
	if u.Status == model.MessageFailed {
		p.ErrorCode, p.ErrorTitle = ptr(u.ErrorCode), ptr(u.ErrorTitle)
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		rows, err := repository.NewRepository(tx).AdvanceMessageStatus(ctx, p)
		if err != nil {
			return fmt.Errorf("inbox: advance status: %w", err)
		}
		for _, m := range rows {
			ev := MessageStatusChanged{
				ConversationID: m.ConversationID, MessageID: m.ID, ContactID: m.ContactID, Channel: m.ChannelType, Status: u.Status,
			}
			if u.Status == model.MessageFailed {
				ev.ErrorCode, ev.ErrorTitle = u.ErrorCode, u.ErrorTitle
			}
			if err := s.publish(ctx, tx, u.WorkspaceID, model.EventMessageStatus, ev); err != nil {
				return err
			}
			if err := signal(ctx, tx, u.WorkspaceID, model.SignalStatus, m.ConversationID, m.TeamID, m.AssigneeID, m.ChannelType, m.ChannelID); err != nil {
				return err
			}
		}
		return nil
	})
}
