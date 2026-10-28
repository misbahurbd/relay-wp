-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;

-- One row per person. Auth fields only; display data lives in user_profiles.
CREATE TABLE users (
    id                uuid        PRIMARY KEY,
    email             citext      NOT NULL UNIQUE,
    email_verified_at timestamptz,
    -- Secret: argon2id hash. NULL for social-only users.
    password_hash     text,
    status            text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    last_login_at     timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One row per linked social login.
CREATE TABLE user_identities (
    id                     uuid        PRIMARY KEY,
    user_id                uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider               text        NOT NULL CHECK (provider IN ('google', 'facebook', 'apple')),
    provider_user_id       text        NOT NULL,
    email                  citext,
    email_verified         boolean     NOT NULL DEFAULT false,
    -- Secret: Apple refresh token, AES-256-GCM encrypted. NULL for other providers.
    provider_refresh_token bytea,
    last_used_at           timestamptz NOT NULL DEFAULT now(),
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_user_id),
    UNIQUE (user_id, provider)
);
CREATE TRIGGER user_identities_set_updated_at BEFORE UPDATE ON user_identities
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Email verification and password reset links.
CREATE TABLE email_tokens (
    id         uuid        PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose    text        NOT NULL CHECK (purpose IN ('verify', 'reset')),
    email      citext      NOT NULL,
    -- Secret: SHA-256 of the token.
    token_hash bytea       NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_tokens_unused_idx ON email_tokens (user_id, purpose) WHERE used_at IS NULL;
CREATE INDEX email_tokens_expires_at_idx ON email_tokens (expires_at);

-- One row per signed-in device. id is the JWT "sid" claim.
CREATE TABLE auth_sessions (
    id               uuid        PRIMARY KEY,
    user_id          uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider         text        NOT NULL CHECK (provider IN ('password', 'google', 'facebook', 'apple')),
    client           text        NOT NULL CHECK (client IN ('web', 'mobile')),
    user_agent       text        NOT NULL DEFAULT '',
    device_name      text        NOT NULL,
    device_type      text        NOT NULL CHECK (device_type IN ('desktop', 'mobile', 'tablet', 'app')),
    created_ip       inet,
    last_ip          inet,
    -- Last real sign-in (password or provider). Refresh never changes it.
    authenticated_at timestamptz NOT NULL,
    last_used_at     timestamptz NOT NULL,
    expires_at       timestamptz NOT NULL,
    revoked_at       timestamptz,
    revoke_reason    text CHECK (revoke_reason IN (
        'logout', 'logout_all', 'password_changed', 'password_reset',
        'reuse_detected', 'user_disabled', 'revoked_by_user')),
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auth_sessions_active_idx ON auth_sessions (user_id) WHERE revoked_at IS NULL;

-- One row per refresh token issued; rotated on every refresh.
CREATE TABLE refresh_tokens (
    id         uuid        PRIMARY KEY,
    session_id uuid        NOT NULL REFERENCES auth_sessions (id) ON DELETE CASCADE,
    -- Secret: SHA-256 of the token.
    token_hash bytea       NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_session_idx ON refresh_tokens (session_id);
CREATE INDEX refresh_tokens_expires_at_idx ON refresh_tokens (expires_at);

-- One row per social login or link attempt; deleted when used.
CREATE TABLE oauth_flows (
    id                    uuid        PRIMARY KEY,
    -- Secret: SHA-256 of the state parameter.
    state_hash            bytea       NOT NULL UNIQUE,
    provider              text        NOT NULL CHECK (provider IN ('google', 'facebook', 'apple')),
    intent                text        NOT NULL CHECK (intent IN ('login', 'link')),
    link_user_id          uuid        REFERENCES users (id) ON DELETE CASCADE,
    code_verifier         text        NOT NULL,
    nonce                 text        NOT NULL,
    redirect_to           text        NOT NULL,
    client                text        NOT NULL CHECK (client IN ('web', 'mobile')),
    status                text        NOT NULL DEFAULT 'started' CHECK (status IN ('started', 'awaiting_email')),
    pending_identity      jsonb,
    -- Secret: SHA-256 of the "add your email" completion token.
    completion_token_hash bytea UNIQUE,
    expires_at            timestamptz NOT NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    CHECK ((intent = 'link') = (link_user_id IS NOT NULL))
);
CREATE INDEX oauth_flows_expires_at_idx ON oauth_flows (expires_at);

-- Append-only security log, kept 180 days.
CREATE TABLE auth_events (
    id         uuid        PRIMARY KEY,
    user_id    uuid        REFERENCES users (id) ON DELETE SET NULL,
    event      text        NOT NULL,
    provider   text,
    ip         inet,
    user_agent text        NOT NULL DEFAULT '',
    -- Never passwords or tokens.
    metadata   jsonb       NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auth_events_user_created_idx ON auth_events (user_id, created_at DESC);
CREATE INDEX auth_events_created_at_idx ON auth_events (created_at);

-- +goose Down
DROP TABLE auth_events;
DROP TABLE oauth_flows;
DROP TABLE refresh_tokens;
DROP TABLE auth_sessions;
DROP TABLE email_tokens;
DROP TABLE user_identities;
DROP TABLE users;
-- citext is left installed: other objects may depend on it.
