-- +goose Up
-- WhatsApp senders with a username may arrive with only a business-scoped user ID
-- and no phone (ADR 0015). The inbox creates such a contact without a phone; its
-- WhatsApp identity is the user ID ("BD.2294857751239750"), and the identity's
-- handle is the username. Contacts added by hand, import, API or commerce still
-- need a phone (enforced by the service).
ALTER TABLE contacts ALTER COLUMN phone DROP NOT NULL;

-- +goose Down
-- Refuses while a contact has no phone, so a rollback never drops contacts.
ALTER TABLE contacts ALTER COLUMN phone SET NOT NULL;
