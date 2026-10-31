-- +goose Up
-- Store webhook deliveries already handled. It holds no customer data: a redelivery
-- finds its row and does nothing.
CREATE TABLE commerce_events (
    connection_id uuid        NOT NULL REFERENCES integration_connections (id) ON DELETE CASCADE,
    delivery_id   text        NOT NULL,
    workspace_id  uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    topic         text        NOT NULL,
    received_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (connection_id, delivery_id)
);

-- An order without a usable phone leaves a skipped run with no contact.
ALTER TABLE automation_runs ALTER COLUMN contact_id DROP NOT NULL;

-- A template can start a chat with someone who never wrote in.
ALTER TABLE conversations DROP CONSTRAINT conversations_source_check;
ALTER TABLE conversations ADD CONSTRAINT conversations_source_check
    CHECK (source IN ('organic', 'ctwa_ad', 'template', 'automation'));

-- +goose Down
ALTER TABLE conversations DROP CONSTRAINT conversations_source_check;
UPDATE conversations SET source = 'template' WHERE source = 'automation';
ALTER TABLE conversations ADD CONSTRAINT conversations_source_check
    CHECK (source IN ('organic', 'ctwa_ad', 'template'));
DELETE FROM automation_runs WHERE contact_id IS NULL;
ALTER TABLE automation_runs ALTER COLUMN contact_id SET NOT NULL;
DROP TABLE commerce_events;
