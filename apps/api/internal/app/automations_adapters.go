package app

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	automationsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
	automationsservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/service"
	contactsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	contactsservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/service"
	inboxmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
	inboxservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/service"
	integrationsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
	whatsappmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
	whatsappservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/service"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
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
	_, err := i.inbox.SendAutomated(ctx, tx, inboxmodel.AutomatedText{
		WorkspaceID: in.WorkspaceID, ConversationID: in.ConversationID, ContactID: in.ContactID, RunID: &in.RunID,
		Text: in.Text, TemplateID: in.TemplateID, Params: in.Params,
	})
	switch {
	case errors.Is(err, inboxmodel.ErrWindowClosed):
		return automationsmodel.ErrWindowClosed
	case errors.Is(err, inboxmodel.ErrContactOptedOut):
		return automationsmodel.ErrOptedOut
	case errors.Is(err, inboxmodel.ErrChannelUnavailable), errors.Is(err, inboxmodel.ErrTemplatesUnsupported):
		return automationsmodel.ErrNoChannel
	case errors.Is(err, inboxmodel.ErrTemplateNotApproved):
		return automationsmodel.ErrTemplateUnavailable
	case apperr.IsKind(err, apperr.KindValidation):
		return automationsmodel.ErrMissingVariable
	}
	return err
}

func (i automationsInbox) NoteOptOut(ctx context.Context, workspaceID, conversationID uuid.UUID) error {
	return i.inbox.NoteOptOut(ctx, workspaceID, conversationID)
}

// automationsContacts maps contacts' errors.
type automationsContacts struct{ *contactsservice.Service }

func (c automationsContacts) FindOrCreateByPhone(ctx context.Context, workspaceID uuid.UUID, phone, name string) (uuid.UUID, error) {
	id, err := c.Service.FindOrCreateByPhone(ctx, workspaceID, phone, name)
	if errors.Is(err, contactsmodel.ErrInvalidPhone) {
		return uuid.Nil, automationsmodel.ErrInvalidPhone
	}
	return id, err
}

func (c automationsContacts) Contact(ctx context.Context, workspaceID, contactID uuid.UUID) (automationsmodel.Contact, error) {
	optIn, name, err := c.Consent(ctx, workspaceID, contactID)
	return automationsmodel.Contact{OptIn: optIn, Name: name}, err
}

// automationsCommerce passes store events to automations.
type automationsCommerce struct{ automations *automationsservice.Service }

func (c automationsCommerce) OnOrder(ctx context.Context, tx pgx.Tx, ev integrationsmodel.OrderEvent) error {
	return c.automations.OnOrder(ctx, tx, automationsmodel.OrderEvent(ev))
}

func (c automationsCommerce) OnCheckout(ctx context.Context, tx pgx.Tx, ev integrationsmodel.CheckoutEvent) error {
	return c.automations.OnCheckout(ctx, tx, automationsmodel.CheckoutEvent(ev))
}

func (c automationsCommerce) RedactCustomer(ctx context.Context, workspaceID uuid.UUID, phone string) error {
	return c.automations.RedactCustomer(ctx, workspaceID, phone)
}

func (c automationsCommerce) RedactStore(ctx context.Context, workspaceID uuid.UUID) error {
	return c.automations.RedactStore(ctx, workspaceID)
}

// inboxAccounts starts new chats from the workspace's first connected number.
type inboxAccounts struct{ wa *whatsappservice.Service }

func (a inboxAccounts) DefaultAccount(ctx context.Context, workspaceID uuid.UUID) (string, uuid.UUID, bool, error) {
	numbers, err := a.wa.Numbers(ctx, workspaceID)
	if err != nil {
		return "", uuid.Nil, false, err
	}
	for _, n := range numbers {
		if n.Status == whatsappmodel.StatusConnected {
			return inboxmodel.ChannelWhatsApp, n.ID, true, nil
		}
	}
	return "", uuid.Nil, false, nil
}
