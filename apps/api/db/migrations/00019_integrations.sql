-- +goose Up
-- Integrations (docs/PHASE-5-INTEGRATIONS.md): API keys, outgoing webhooks and
-- store/app connections.

-- An API key. It acts as its creator, in this workspace only (ADR 0011).
CREATE TABLE api_keys (
    id           uuid        PRIMARY KEY,
    workspace_id uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name         text        NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
    -- The no-code app the key was made for, so its card shows connected.
    app          text        CHECK (app IN ('zapier', 'make', 'n8n')),
    prefix       text        NOT NULL,
    last4        text        NOT NULL,
    -- Secret: SHA-256 of the key. Compared only in GetAPIKeyByHash's WHERE.
    token_hash   bytea       NOT NULL UNIQUE,
    created_by   uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    last_used_at timestamptz,
    revoked_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX api_keys_workspace_idx ON api_keys (workspace_id, created_at DESC);

-- Where events are POSTed.
CREATE TABLE webhook_endpoints (
    id                   uuid        PRIMARY KEY,
    workspace_id         uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    url                  text        NOT NULL CHECK (length(url) <= 2048),
    description          text        NOT NULL DEFAULT '' CHECK (length(description) <= 200),
    events               text[]      NOT NULL CHECK (cardinality(events) > 0),
    enabled              boolean     NOT NULL DEFAULT true,
    -- Secret: the signing secret, AES-256-GCM with TOKEN_ENCRYPTION_KEY. Read only by GetWebhookSecret.
    secret               bytea       NOT NULL,
    consecutive_failures int         NOT NULL DEFAULT 0,
    disabled_reason      text,
    created_by           uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX webhook_endpoints_workspace_idx ON webhook_endpoints (workspace_id);
CREATE TRIGGER webhook_endpoints_set_updated_at BEFORE UPDATE ON webhook_endpoints
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One event sent to one endpoint, with the outcome of its latest attempt. Kept 30 days.
CREATE TABLE webhook_deliveries (
    id              uuid        PRIMARY KEY,
    workspace_id    uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    endpoint_id     uuid        NOT NULL REFERENCES webhook_endpoints (id) ON DELETE CASCADE,
    event_id        uuid        NOT NULL,
    event_type      text        NOT NULL,
    payload         jsonb       NOT NULL,
    status          text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'succeeded', 'failed')),
    attempts        int         NOT NULL DEFAULT 0,
    response_status int,
    response_ms     int,
    error           text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    delivered_at    timestamptz
);
CREATE INDEX webhook_deliveries_endpoint_idx ON webhook_deliveries (endpoint_id, created_at DESC, id DESC);
CREATE INDEX webhook_deliveries_created_idx ON webhook_deliveries (created_at);

-- A connected store or app. One row per workspace and provider.
CREATE TABLE integration_connections (
    id              uuid        PRIMARY KEY,
    workspace_id    uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    provider        text        NOT NULL CHECK (provider IN ('shopify', 'woocommerce', 'google_sheets')),
    status          text        NOT NULL CHECK (status IN ('connected', 'error', 'disconnected')),
    -- What the page shows: the shop domain, store URL or Google account.
    account_label   text        NOT NULL DEFAULT '',
    config          jsonb       NOT NULL DEFAULT '{}',
    -- Secret: provider credentials as JSON, AES-256-GCM. Read only by GetConnectionCredentials.
    credentials     bytea,
    last_checked_at timestamptz,
    last_error      text,
    connected_by    uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, provider)
);
CREATE TRIGGER integration_connections_set_updated_at BEFORE UPDATE ON integration_connections
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE integration_connections;
DROP TABLE webhook_deliveries;
DROP TABLE webhook_endpoints;
DROP TABLE api_keys;
