-- +goose Up
-- Broadcasts: one approved template sent to many contacts (ADR 0013).
CREATE TABLE broadcasts (
    id               uuid PRIMARY KEY,
    workspace_id     uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name             text        NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    number_id        uuid,
    template_id      uuid,
    -- {"tag_ids": [...]}: opted-in contacts with any of the tags; none = every opted-in contact.
    audience         jsonb       NOT NULL DEFAULT '{}',
    -- {"header": {...}, "body": [{"source": "text|name|phone|attribute", "value": "", "fallback": ""}]}
    variables        jsonb       NOT NULL DEFAULT '{}',
    status           text        NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'scheduled', 'sending', 'completed', 'cancelled', 'failed')),
    scheduled_at     timestamptz,
    started_at       timestamptz,
    snapshot_at      timestamptz,
    completed_at     timestamptz,
    recipients_total integer     NOT NULL DEFAULT 0,
    -- The month the plan's broadcasts unit was taken from, so a cancel gives it back there.
    usage_period     date,
    failure_reason   text,
    created_by       uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, id)
);
CREATE INDEX broadcasts_list_idx ON broadcasts (workspace_id, id DESC);
CREATE INDEX broadcasts_active_idx ON broadcasts (status, scheduled_at) WHERE status IN ('scheduled', 'sending');

-- One row per contact per broadcast: who gets it, and the record that they got
-- it once. contact_id has no foreign key, so a deleted contact stays in reports.
CREATE TABLE broadcast_recipients (
    workspace_id uuid        NOT NULL,
    broadcast_id uuid        NOT NULL,
    contact_id   uuid        NOT NULL,
    phone        text        NOT NULL,
    name         text        NOT NULL DEFAULT '',
    state        text        NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'queued', 'skipped', 'failed')),
    reason       text,
    message_id   uuid UNIQUE,
    queued_at    timestamptz,
    PRIMARY KEY (broadcast_id, contact_id),
    FOREIGN KEY (workspace_id, broadcast_id) REFERENCES broadcasts (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX broadcast_recipients_state_idx ON broadcast_recipients (broadcast_id, state);

ALTER TABLE conversations DROP CONSTRAINT conversations_source_check;
ALTER TABLE conversations ADD CONSTRAINT conversations_source_check
    CHECK (source IN ('organic', 'ctwa_ad', 'template', 'automation', 'broadcast'));

-- +goose Down
ALTER TABLE conversations DROP CONSTRAINT conversations_source_check;
UPDATE conversations SET source = 'template' WHERE source = 'broadcast';
ALTER TABLE conversations ADD CONSTRAINT conversations_source_check
    CHECK (source IN ('organic', 'ctwa_ad', 'template', 'automation'));
DROP TABLE broadcast_recipients;
DROP TABLE broadcasts;
