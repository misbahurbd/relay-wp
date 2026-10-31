-- +goose Up
-- Automations (docs/PHASE-8-AUTOMATIONS.md, ADR 0012): a workspace's six preset
-- automations. A kind has no row until someone saves or toggles it; until then
-- the service serves its defaults, turned off.
CREATE TABLE automations (
    id           uuid        PRIMARY KEY,
    workspace_id uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    kind         text        NOT NULL CHECK (kind IN ('welcome', 'away', 'keyword', 'cart', 'order', 'drip')),
    enabled      boolean     NOT NULL DEFAULT false,
    -- The kind's typed settings (model.Settings); validated by the service.
    settings     jsonb       NOT NULL,
    updated_by   uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, kind)
);
CREATE TRIGGER automations_set_updated_at BEFORE UPDATE ON automations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE automations;
