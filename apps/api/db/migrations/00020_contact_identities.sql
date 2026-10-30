-- +goose Up
-- Who a contact is on each channel (ADR 0010, docs/PHASE-7-INBOX.md section 11).
-- Inbound messages find their contact by (channel_type, external_id): for WhatsApp
-- the external ID is the E.164 phone, for Messenger and Instagram a page-scoped ID.
-- A later edit of contacts.phone doesn't lose the chat, because the identity stays.

-- Lets identities reference a contact of the same workspace.
ALTER TABLE contacts ADD CONSTRAINT contacts_workspace_id_id_key UNIQUE (workspace_id, id);

CREATE TABLE contact_identities (
    workspace_id uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    contact_id   uuid        NOT NULL,
    channel_type text        NOT NULL CHECK (channel_type IN ('whatsapp', 'messenger', 'instagram')),
    -- The sender's ID on the channel: E.164 phone for WhatsApp, PSID or IGSID later.
    external_id  text        NOT NULL CHECK (char_length(external_id) BETWEEN 1 AND 128),
    -- What the channel calls the sender, for example an Instagram @username; may be empty.
    handle       text        NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, channel_type, external_id),
    FOREIGN KEY (workspace_id, contact_id) REFERENCES contacts (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX contact_identities_contact_idx ON contact_identities (workspace_id, contact_id);

-- +goose Down
DROP TABLE contact_identities;
ALTER TABLE contacts DROP CONSTRAINT contacts_workspace_id_id_key;
