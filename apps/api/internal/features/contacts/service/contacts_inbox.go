package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/repository"
)

// maxExternalIDLen matches contact_identities.external_id's check.
const maxExternalIDLen = 128

// FindOrCreateByIdentity returns the contact a channel's sender is (ADR 0010),
// and whether it was created now. A known identity wins, so a later edit of the
// contact's phone doesn't lose the chat. An unknown sender with a phone links to
// the contact with that phone, or creates one (source inbox, opt-in pending); a
// sender without a phone fails with ErrIdentityNeedsPhone until contacts may
// have no phone. Concurrent calls for one new sender agree on one contact and
// one identity. For features/inbox, through an interface it declares.
func (s *Service) FindOrCreateByIdentity(ctx context.Context, workspaceID uuid.UUID, channelType, externalID, name, phone string) (model.Contact, bool, error) {
	channelType, externalID = strings.TrimSpace(channelType), strings.TrimSpace(externalID)
	if channelType == "" || externalID == "" || len(externalID) > maxExternalIDLen {
		return model.Contact{}, false, model.ErrInvalidIdentity
	}
	r := repository.NewRepository(s.pool)
	key := gen.ContactIDByIdentityParams{WorkspaceID: workspaceID, ChannelType: channelType, ExternalID: externalID}
	id, err := r.ContactIDByIdentity(ctx, key)
	switch {
	case err == nil:
		return s.inboxContact(ctx, workspaceID, id, false)
	case !repository.IsNoRows(err):
		return model.Contact{}, false, fmt.Errorf("contact by identity: %w", err)
	}
	if strings.TrimSpace(phone) == "" {
		return model.Contact{}, false, model.ErrIdentityNeedsPhone
	}
	contactID, created, err := s.findOrCreateByPhone(ctx, r, workspaceID, phone, name)
	if err != nil {
		return model.Contact{}, false, err
	}
	if err := r.InsertContactIdentity(ctx, gen.InsertContactIdentityParams{
		WorkspaceID: workspaceID, ContactID: contactID, ChannelType: channelType, ExternalID: externalID,
	}); err != nil {
		return model.Contact{}, false, fmt.Errorf("insert contact identity: %w", err)
	}
	// Another call may have linked the sender first; its link is the truth.
	linked, err := r.ContactIDByIdentity(ctx, key)
	if err != nil {
		return model.Contact{}, false, fmt.Errorf("contact by identity: %w", err)
	}
	return s.inboxContact(ctx, workspaceID, linked, created && linked == contactID)
}

// findOrCreateByPhone returns the contact with this phone, creating it when the
// customer is new. Two concurrent calls for one phone: one inserts, both read it.
func (s *Service) findOrCreateByPhone(ctx context.Context, r repository.Repository, workspaceID uuid.UUID, phone, name string) (uuid.UUID, bool, error) {
	e164, ok := NormalizePhone(phone)
	if !ok {
		return uuid.Nil, false, model.ErrInvalidPhone
	}
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > model.MaxNameLen {
		name = string([]rune(name)[:model.MaxNameLen])
	}
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("contact id: %w", err)
	}
	inserted, err := r.InsertInboxContact(ctx, gen.InsertInboxContactParams{ID: id, WorkspaceID: workspaceID, Name: name, Phone: e164})
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("insert inbox contact: %w", err)
	}
	if len(inserted) == 1 {
		return id, true, nil
	}
	if id, err = r.ContactIDByPhone(ctx, gen.ContactIDByPhoneParams{WorkspaceID: workspaceID, Phone: e164}); err != nil {
		return uuid.Nil, false, fmt.Errorf("contact by phone: %w", err)
	}
	return id, false, nil
}

func (s *Service) inboxContact(ctx context.Context, workspaceID, id uuid.UUID, created bool) (model.Contact, bool, error) {
	d, err := s.get(ctx, s.pool, workspaceID, id)
	if err != nil {
		return model.Contact{}, false, err
	}
	return d.Contact, created, nil
}

// SetLastMessage records a message to or from the contact at at, in tx. An
// older time never replaces a newer one.
func (s *Service) SetLastMessage(ctx context.Context, tx pgx.Tx, workspaceID, contactID uuid.UUID, at time.Time) error {
	err := repository.NewRepository(tx).SetContactLastMessage(ctx, gen.SetContactLastMessageParams{
		WorkspaceID: workspaceID, ID: contactID, At: at,
	})
	if err != nil {
		return fmt.Errorf("set contact last message: %w", err)
	}
	return nil
}

// SetOptOut records that a contact opted out; changed is false if they already
// had. For features/automations, through an interface it declares.
func (s *Service) SetOptOut(ctx context.Context, workspaceID, contactID uuid.UUID, source string) (bool, error) {
	n, err := repository.NewRepository(s.pool).SetContactOptOut(ctx, gen.SetContactOptOutParams{
		WorkspaceID: workspaceID, ID: contactID, Source: &source,
	})
	if err != nil {
		return false, fmt.Errorf("set opt out: %w", err)
	}
	return n > 0, nil
}
