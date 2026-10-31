-- +goose Up
-- One contact's pass through one automation (ADR 0012). next_at is the schedule,
-- and dedupe_key stops a retried trigger from sending twice.
CREATE TABLE automation_runs (
    id              uuid        PRIMARY KEY,
    workspace_id    uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    kind            text        NOT NULL CHECK (kind IN ('welcome', 'away', 'keyword', 'cart', 'order', 'drip')),
    contact_id      uuid        NOT NULL REFERENCES contacts (id) ON DELETE CASCADE,
    conversation_id uuid,
    dedupe_key      text        NOT NULL,
    state           text        NOT NULL CHECK (state IN ('scheduled', 'done', 'skipped', 'cancelled', 'failed')),
    -- Why a run ended without sending: agent_active, opted_out, paused, ...
    reason          text,
    step            int         NOT NULL DEFAULT 0,
    next_at         timestamptz,
    -- What the send needs, fixed when the run was made.
    context         jsonb       NOT NULL DEFAULT '{}',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, dedupe_key)
);
CREATE INDEX automation_runs_due_idx ON automation_runs (next_at) WHERE state = 'scheduled';
CREATE INDEX automation_runs_stats_idx ON automation_runs (workspace_id, created_at);
CREATE INDEX automation_runs_contact_idx ON automation_runs (workspace_id, contact_id) WHERE state = 'scheduled';
CREATE TRIGGER automation_runs_set_updated_at BEFORE UPDATE ON automation_runs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Set on messages an automation sent; they have no author.
ALTER TABLE messages ADD COLUMN automation_run_id uuid REFERENCES automation_runs (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE messages DROP COLUMN automation_run_id;
DROP TABLE automation_runs;
