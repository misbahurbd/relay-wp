package app

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	automationsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
	automationsservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/service"
	inboxmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
	inboxservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/service"
)

// automationsHook tells automations about each customer message.
type automationsHook struct{ automations *automationsservice.Service }

func (h automationsHook) OnInbound(ctx context.Context, tx pgx.Tx, ev inboxmodel.InboundEvent) error {
	return h.automations.OnInbound(ctx, tx, automationsmodel.Inbound{
		WorkspaceID: ev.WorkspaceID, ConversationID: ev.ConversationID, ContactID: ev.ContactID, MessageID: ev.MessageID,
		Text: ev.Text, NewConversation: ev.NewConversation, Returning: ev.Returning,
	})
}

// automationsInbox lets automations read conversations and send.
type automationsInbox struct{ inbox *inboxservice.Service }

func (i automationsInbox) Conversation(ctx context.Context, workspaceID, conversationID uuid.UUID) (automationsmodel.Conversation, error) {
	c, err := i.inbox.AutomationConversation(ctx, workspaceID, conversationID)
	if err != nil {
		return automationsmodel.Conversation{}, err
	}
	return automationsmodel.Conversation{ContactName: c.ContactName, OptIn: c.OptIn, LastAgentAt: c.LastAgentAt}, nil
}

func (i automationsInbox) SendAutomated(ctx context.Context, tx pgx.Tx, in automationsmodel.AutomatedSend) error {
	err := i.inbox.SendAutomated(ctx, tx, inboxmodel.AutomatedText{
		WorkspaceID: in.WorkspaceID, ConversationID: in.ConversationID, RunID: in.RunID, Text: in.Text,
	})
	switch {
	case errors.Is(err, inboxmodel.ErrWindowClosed):
		return automationsmodel.ErrWindowClosed
	case errors.Is(err, inboxmodel.ErrContactOptedOut):
		return automationsmodel.ErrOptedOut
	}
	return err
}

func (i automationsInbox) NoteOptOut(ctx context.Context, workspaceID, conversationID uuid.UUID) error {
	return i.inbox.NoteOptOut(ctx, workspaceID, conversationID)
}
