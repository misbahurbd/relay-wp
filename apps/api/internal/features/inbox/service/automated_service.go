package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// InboundHook hears about each customer message, in the ingest transaction.
// Implemented in internal/app on top of features/automations.
type InboundHook interface {
	OnInbound(ctx context.Context, tx pgx.Tx, ev model.InboundEvent) error
}

// SetInboundHook connects the hook, which is built after this service.
func (s *Service) SetInboundHook(h InboundHook) { s.hook = h }

// AutomationConversation is what automations read before they send.
func (s *Service) AutomationConversation(ctx context.Context, workspaceID, id uuid.UUID) (model.AutomationConversation, error) {
	c, err := repository.NewRepository(s.pool).AutomationConversation(ctx, gen.AutomationConversationParams{WorkspaceID: workspaceID, ID: id})
	if repository.IsNoRows(err) {
		return model.AutomationConversation{}, model.ErrConversationNotFound
	}
	if err != nil {
		return model.AutomationConversation{}, fmt.Errorf("automation conversation: %w", err)
	}
	out := model.AutomationConversation{ContactName: c.ContactName, OptIn: c.OptIn}
	if c.LastAgentAt.Unix() > 0 {
		out.LastAgentAt = &c.LastAgentAt
	}
	return out, nil
}

// SendAutomated queues what an automation wrote, in tx, so it commits with the
// run. It has no author, needs a contact who hasn't opted out and, for text, an
// open window, and leaves the chat waiting for a person. Without a conversation
// it uses the contact's latest, or starts one.
func (s *Service) SendAutomated(ctx context.Context, tx pgx.Tx, in model.AutomatedText) error {
	r := repository.NewRepository(tx)
	ws := in.WorkspaceID
	convID, err := s.automationConversation(ctx, r, in)
	if err != nil {
		return err
	}
	c, err := r.LockConversation(ctx, gen.LockConversationParams{WorkspaceID: ws, ID: convID})
	if err != nil {
		return fmt.Errorf("lock conversation: %w", err)
	}
	info, err := r.AutomationConversation(ctx, gen.AutomationConversationParams{WorkspaceID: ws, ID: convID})
	if err != nil {
		return fmt.Errorf("automation conversation: %w", err)
	}
	if info.OptIn == "out" {
		return model.ErrContactOptedOut
	}
	ch, ok := s.channel(c.ChannelType)
	if !ok {
		return model.ErrChannelUnavailable
	}
	caps, now := ch.Capabilities(), s.now()
	out := outbound{msgType: model.TypeText, body: in.Text}
	if in.TemplateID != nil {
		if !caps.Templates {
			return model.ErrTemplatesUnsupported
		}
		out, err = s.checkTemplate(ctx, authctx.Access{WorkspaceID: ws}, caps, model.SendInput{
			Type: model.SendTemplate, TemplateID: in.TemplateID, Params: model.TemplateParams{Body: in.Params},
		})
		if err != nil {
			return err
		}
	} else if !windowOpen(caps, c.WindowExpiresAt, now) {
		return model.ErrWindowClosed
	}
	id, err := newID()
	if err != nil {
		return err
	}
	raw, err := encodeContent(out.content)
	if err != nil {
		return err
	}
	if err := r.InsertAutomatedMessage(ctx, gen.InsertAutomatedMessageParams{
		ID: id, WorkspaceID: ws, ConversationID: convID, Type: out.msgType, Body: out.body, Content: raw,
		AutomationRunID: &in.RunID, CreatedAt: now,
	}); err != nil {
		return fmt.Errorf("insert automated message: %w", err)
	}
	row, err := r.TouchAutomated(ctx, gen.TouchAutomatedParams{
		At: now, Preview: preview(out.msgType, out.body), ToPending: out.template != nil, WorkspaceID: ws, ID: convID,
	})
	if err != nil {
		return fmt.Errorf("touch conversation: %w", err)
	}
	if err := s.deps.Contacts.SetLastMessage(ctx, tx, ws, c.ContactID, now); err != nil {
		return err
	}
	if _, err := s.queue.InsertTx(ctx, tx, model.SendArgs{WorkspaceID: ws, MessageID: id}, nil); err != nil {
		return fmt.Errorf("enqueue send: %w", err)
	}
	return signal(ctx, tx, ws, model.SignalMessage, convID, row.TeamID, row.AssigneeID, row.ChannelType, row.ChannelID)
}

func (s *Service) automationConversation(ctx context.Context, r repository.Repository, in model.AutomatedText) (uuid.UUID, error) {
	if in.ConversationID != nil {
		return *in.ConversationID, nil
	}
	id, err := r.LatestContactConversation(ctx, gen.LatestContactConversationParams{WorkspaceID: in.WorkspaceID, ContactID: *in.ContactID})
	if err == nil {
		return id, nil
	}
	if !repository.IsNoRows(err) {
		return uuid.Nil, fmt.Errorf("latest conversation: %w", err)
	}
	channelType, channelID, ok, err := s.deps.Accounts.DefaultAccount(ctx, in.WorkspaceID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("default account: %w", err)
	}
	if !ok {
		return uuid.Nil, model.ErrChannelUnavailable
	}
	if id, err = newID(); err != nil {
		return uuid.Nil, err
	}
	id, err = r.InsertAutomationConversation(ctx, gen.InsertAutomationConversationParams{
		ID: id, WorkspaceID: in.WorkspaceID, ContactID: *in.ContactID, ChannelType: channelType, ChannelID: channelID,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("start conversation: %w", err)
	}
	return id, nil
}

// NoteOptOut adds a line to the thread saying the customer opted out.
func (s *Service) NoteOptOut(ctx context.Context, workspaceID, conversationID uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if err := s.event(ctx, r, workspaceID, conversationID, model.EventContent{Action: model.ActionOptedOut}); err != nil {
			return err
		}
		c, err := r.LockConversation(ctx, gen.LockConversationParams{WorkspaceID: workspaceID, ID: conversationID})
		if err != nil {
			return fmt.Errorf("lock conversation: %w", err)
		}
		return signal(ctx, tx, workspaceID, model.SignalMessage, conversationID, c.TeamID, c.AssigneeID, c.ChannelType, c.ChannelID)
	})
}
