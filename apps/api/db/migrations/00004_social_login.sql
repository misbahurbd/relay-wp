-- +goose Up
-- Social sign-in details that weren't in 00002. oauth_flows gains:
--   timezone           from the browser at /start, used if the sign-in creates an account
--   app_code_challenge  mobile PKCE (S256): only the app that started the flow can redeem it
--   user_id            the signed-in user while a mobile flow waits to be redeemed ('ready')
ALTER TABLE oauth_flows
    ADD COLUMN timezone           text NOT NULL DEFAULT 'UTC',
    ADD COLUMN app_code_challenge text,
    ADD COLUMN user_id            uuid REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE oauth_flows DROP CONSTRAINT oauth_flows_status_check;
ALTER TABLE oauth_flows ADD CONSTRAINT oauth_flows_status_check
    CHECK (status IN ('started', 'awaiting_email', 'ready'));
ALTER TABLE oauth_flows ADD CONSTRAINT oauth_flows_ready_user_check
    CHECK ((status = 'ready') = (user_id IS NOT NULL));

-- A provider-verified sign-in claims an unverified account: its sessions end.
ALTER TABLE auth_sessions DROP CONSTRAINT auth_sessions_revoke_reason_check;
ALTER TABLE auth_sessions ADD CONSTRAINT auth_sessions_revoke_reason_check CHECK (revoke_reason IN (
    'logout', 'logout_all', 'password_changed', 'password_reset', 'reuse_detected',
    'user_disabled', 'revoked_by_user', 'account_claimed'
));

-- +goose Down
UPDATE auth_sessions SET revoke_reason = 'password_reset' WHERE revoke_reason = 'account_claimed';
ALTER TABLE auth_sessions DROP CONSTRAINT auth_sessions_revoke_reason_check;
ALTER TABLE auth_sessions ADD CONSTRAINT auth_sessions_revoke_reason_check CHECK (revoke_reason IN (
    'logout', 'logout_all', 'password_changed', 'password_reset', 'reuse_detected',
    'user_disabled', 'revoked_by_user'
));
DELETE FROM oauth_flows WHERE status = 'ready';
ALTER TABLE oauth_flows DROP CONSTRAINT oauth_flows_ready_user_check;
ALTER TABLE oauth_flows DROP CONSTRAINT oauth_flows_status_check;
ALTER TABLE oauth_flows ADD CONSTRAINT oauth_flows_status_check
    CHECK (status IN ('started', 'awaiting_email'));
ALTER TABLE oauth_flows
    DROP COLUMN user_id,
    DROP COLUMN app_code_challenge,
    DROP COLUMN timezone;
