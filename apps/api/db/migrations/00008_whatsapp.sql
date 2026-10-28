-- +goose Up
-- A WhatsApp Business account (WABA) connected through Meta Embedded Signup.
CREATE TABLE whatsapp_accounts (
    id           uuid        PRIMARY KEY,
    workspace_id uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    -- Meta's IDs. A WABA belongs to one workspace.
    waba_id      text        NOT NULL UNIQUE,
    business_id  text,
    name         text        NOT NULL DEFAULT '',
    -- Secret: business integration token, AES-256-GCM with TOKEN_ENCRYPTION_KEY.
    access_token bytea       NOT NULL,
    status       text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disconnected')),
    connected_by uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX whatsapp_accounts_workspace_idx ON whatsapp_accounts (workspace_id);
CREATE TRIGGER whatsapp_accounts_set_updated_at BEFORE UPDATE ON whatsapp_accounts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- A phone number on a WABA.
CREATE TABLE whatsapp_numbers (
    id                   uuid        PRIMARY KEY,
    workspace_id         uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    account_id           uuid        NOT NULL REFERENCES whatsapp_accounts (id) ON DELETE CASCADE,
    phone_number_id      text        NOT NULL UNIQUE,
    display_phone_number text        NOT NULL DEFAULT '',
    number_type          text        NOT NULL CHECK (number_type IN ('new', 'coexistence', 'migration')),
    verified_name        text        NOT NULL DEFAULT '',
    name_status          text        NOT NULL DEFAULT 'pending_review',
    quality_rating       text,
    messaging_limit      text,
    -- Secret: the two-step verification PIN set at registration, encrypted. Needed to re-register.
    registration_pin     bytea,
    status               text        NOT NULL DEFAULT 'setting_up'
                                     CHECK (status IN ('setting_up', 'connected', 'failed', 'disconnected')),
    -- {register, subscribe_webhooks, sync_templates, import_profile} -> waiting|running|done|failed
    setup_tasks          jsonb       NOT NULL DEFAULT '{}',
    -- Meta's error code and ours. Never tokens.
    last_error           jsonb,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX whatsapp_numbers_workspace_idx ON whatsapp_numbers (workspace_id);
CREATE TRIGGER whatsapp_numbers_set_updated_at BEFORE UPDATE ON whatsapp_numbers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Test messages sent during setup, until the Inbox feature owns messages.
CREATE TABLE whatsapp_test_messages (
    id           uuid        PRIMARY KEY,
    workspace_id uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    number_id    uuid        NOT NULL REFERENCES whatsapp_numbers (id) ON DELETE CASCADE,
    to_phone     text        NOT NULL,
    wamid        text        UNIQUE,
    status       text        NOT NULL DEFAULT 'queued'
                             CHECK (status IN ('queued', 'sent', 'delivered', 'read', 'failed')),
    error_code   text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX whatsapp_test_messages_workspace_idx ON whatsapp_test_messages (workspace_id, created_at);
CREATE TRIGGER whatsapp_test_messages_set_updated_at BEFORE UPDATE ON whatsapp_test_messages
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE whatsapp_test_messages;
DROP TABLE whatsapp_numbers;
DROP TABLE whatsapp_accounts;
