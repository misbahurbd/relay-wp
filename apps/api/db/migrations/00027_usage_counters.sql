-- +goose Up
-- How much of each monthly plan resource a workspace used, per calendar month
-- (UTC). A new month is a new row, never a reset (ADR 0008).
CREATE TABLE usage_counters (
    workspace_id uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    resource     text        NOT NULL,
    period_start date        NOT NULL,
    used         integer     NOT NULL DEFAULT 0 CHECK (used >= 0),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, resource, period_start)
);

-- +goose Down
DROP TABLE usage_counters;
