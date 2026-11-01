-- +goose Up
-- Saved rules that group contacts on their own (ADR 0014).
CREATE TABLE contact_segments (
    id           uuid PRIMARY KEY,
    workspace_id uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name         citext      NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    description  text        NOT NULL DEFAULT '' CHECK (char_length(description) <= 200),
    -- {"groups": [{"match": "all|any", "conditions": [{"field", "op", "value", "key"}]}]}
    rules        jsonb       NOT NULL,
    created_by   uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT contact_segments_workspace_name_key UNIQUE (workspace_id, name)
);
CREATE INDEX contact_segments_list_idx ON contact_segments (workspace_id, name);

-- +goose Down
DROP TABLE contact_segments;
