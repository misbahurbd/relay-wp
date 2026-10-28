-- Rules (CLAUDE.md): list columns explicitly, never SELECT *. password_hash and
-- token hashes are selected only where they are needed.

-- name: CreateUser :exec
INSERT INTO users (id, email, password_hash)
VALUES (@id, @email, @password_hash);

-- name: GetUserForLogin :one
-- The only query besides GetPasswordHash that reads password_hash.
SELECT id, email_verified_at, password_hash, status
FROM users
WHERE email = @email;

-- name: GetPasswordHash :one
-- Change password, and confirming the password on the verify-link page.
SELECT password_hash
FROM users
WHERE id = @id;

-- name: GetAccount :one
SELECT id, email, email_verified_at, status, (password_hash IS NOT NULL)::boolean AS has_password
FROM users
WHERE id = @id;

-- name: GetAccountByEmail :one
SELECT id, email, email_verified_at, status, (password_hash IS NOT NULL)::boolean AS has_password
FROM users
WHERE email = @email;

-- name: SetPasswordHash :exec
UPDATE users
SET password_hash = @password_hash
WHERE id = @id;

-- name: MarkEmailVerified :exec
UPDATE users
SET email_verified_at = COALESCE(email_verified_at, now())
WHERE id = @id;

-- name: TouchLastLogin :exec
UPDATE users
SET last_login_at = now()
WHERE id = @id;

-- name: InvalidateEmailTokens :exec
UPDATE email_tokens
SET used_at = now()
WHERE user_id = @user_id AND purpose = @purpose AND used_at IS NULL;

-- name: CreateEmailToken :exec
INSERT INTO email_tokens (id, user_id, purpose, email, token_hash, expires_at)
VALUES (@id, @user_id, @purpose, @email, @token_hash, @expires_at);

-- name: ConsumeEmailToken :one
-- Atomically marks a valid token used and returns who it belongs to.
UPDATE email_tokens
SET used_at = now()
WHERE token_hash = @token_hash
  AND purpose = @purpose
  AND used_at IS NULL
  AND expires_at > now()
RETURNING user_id, email;

-- name: CreateSession :exec
INSERT INTO auth_sessions (
    id, user_id, provider, client, user_agent, device_name, device_type,
    created_ip, last_ip, authenticated_at, last_used_at, expires_at
) VALUES (
    @id, @user_id, @provider, @client, @user_agent, @device_name, @device_type,
    @ip, @ip, @now, @now, @expires_at
);

-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (id, session_id, token_hash, expires_at)
VALUES (@id, @session_id, @token_hash, @expires_at);

-- name: GetRefreshTokenForUpdate :one
-- Locks the token row so two concurrent refreshes can't both rotate it.
SELECT
    rt.id,
    rt.session_id,
    rt.expires_at,
    rt.used_at,
    s.user_id,
    s.client,
    s.revoked_at AS session_revoked_at,
    s.expires_at AS session_expires_at,
    s.created_at AS session_created_at,
    u.status AS user_status,
    u.email_verified_at AS user_email_verified_at
FROM refresh_tokens rt
JOIN auth_sessions s ON s.id = rt.session_id
JOIN users u ON u.id = s.user_id
WHERE rt.token_hash = @token_hash
FOR UPDATE OF rt;

-- name: MarkRefreshTokenUsed :exec
-- used_at comes from the service clock, which also checks the reuse grace window.
UPDATE refresh_tokens
SET used_at = @used_at
WHERE id = @id;

-- name: ExtendSession :exec
UPDATE auth_sessions
SET last_used_at = now(), last_ip = @ip, expires_at = @expires_at
WHERE id = @id;

-- name: RevokeSession :execrows
UPDATE auth_sessions
SET revoked_at = now(), revoke_reason = @reason
WHERE id = @id AND user_id = @user_id AND revoked_at IS NULL;

-- name: RevokeSessionByID :exec
UPDATE auth_sessions
SET revoked_at = now(), revoke_reason = @reason
WHERE id = @id AND revoked_at IS NULL;

-- name: RevokeUserSessions :execrows
-- Revokes every active session of a user, optionally keeping one (the current device).
UPDATE auth_sessions
SET revoked_at = now(), revoke_reason = @reason
WHERE user_id = @user_id
  AND revoked_at IS NULL
  AND (sqlc.narg(keep_id)::uuid IS NULL OR id <> sqlc.narg(keep_id)::uuid);

-- name: IsSessionActive :one
-- Checked on every authenticated request, so a revoked session, an expired one,
-- a disabled user or an unproven email stops working at once instead of when
-- the access token expires.
SELECT EXISTS (
    SELECT 1
    FROM auth_sessions s
    JOIN users u ON u.id = s.user_id
    WHERE s.id = @id
      AND s.user_id = @user_id
      AND s.revoked_at IS NULL
      AND s.expires_at > now()
      AND u.status = 'active'
      AND u.email_verified_at IS NOT NULL
)::boolean AS active;

-- name: ListActiveSessions :many
SELECT id, provider, client, device_name, device_type, last_ip, last_used_at, created_at
FROM auth_sessions
WHERE user_id = @user_id AND revoked_at IS NULL AND expires_at > now()
ORDER BY last_used_at DESC;

-- name: InsertAuthEvent :exec
INSERT INTO auth_events (id, user_id, event, provider, ip, user_agent, metadata)
VALUES (@id, @user_id, @event, @provider, @ip, @user_agent, @metadata);

-- name: ListAuthEvents :many
SELECT id, event, provider, ip, user_agent, created_at
FROM auth_events
WHERE user_id = @user_id
ORDER BY created_at DESC
LIMIT 50;

-- name: CreateSocialUser :exec
-- A user created by social sign-in: no password; verified only if the provider vouched.
INSERT INTO users (id, email, email_verified_at)
VALUES (@id, @email, sqlc.narg(email_verified_at));

-- name: SetPasswordIfUnset :execrows
-- POST /auth/password/set: only for accounts that have no password yet.
UPDATE users
SET password_hash = @password_hash
WHERE id = @id AND password_hash IS NULL;

-- name: GetSessionAuthenticatedAt :one
-- When this session last proved who it is (password or social sign-in), for sensitive actions.
SELECT authenticated_at
FROM auth_sessions
WHERE id = @id AND user_id = @user_id AND revoked_at IS NULL;

-- name: GetIdentity :one
SELECT id, user_id
FROM user_identities
WHERE provider = @provider AND provider_user_id = @provider_user_id;

-- name: CreateIdentity :exec
INSERT INTO user_identities (id, user_id, provider, provider_user_id, email, email_verified)
VALUES (@id, @user_id, @provider, @provider_user_id, @email, @email_verified);

-- name: TouchIdentity :exec
-- Refresh what the provider reports on every sign-in. Never touches the profile.
UPDATE user_identities
SET email = @email, email_verified = @email_verified, last_used_at = now()
WHERE id = @id;

-- name: ListIdentities :many
SELECT provider, email, last_used_at, created_at
FROM user_identities
WHERE user_id = @user_id
ORDER BY created_at;

-- name: CountIdentities :one
SELECT count(*)
FROM user_identities
WHERE user_id = @user_id;

-- name: DeleteIdentity :execrows
DELETE FROM user_identities
WHERE user_id = @user_id AND provider = @provider;

-- name: DeleteIdentityByProviderUser :one
-- Meta's data-deletion callback: identifies the Facebook user only.
DELETE FROM user_identities
WHERE provider = @provider AND provider_user_id = @provider_user_id
RETURNING user_id;

-- name: DeleteUnverifiedIdentities :execrows
-- A password reset proves who owns the email: identities whose provider never
-- vouched for that email (e.g. Facebook) can't keep access to the account.
DELETE FROM user_identities
WHERE user_id = @user_id AND email_verified = false;

-- name: CreateOAuthFlow :exec
INSERT INTO oauth_flows (
    id, state_hash, provider, intent, link_user_id, code_verifier, nonce,
    redirect_to, client, timezone, app_code_challenge, expires_at
) VALUES (
    @id, @state_hash, @provider, @intent, sqlc.narg(link_user_id), @code_verifier, @nonce,
    @redirect_to, @client, @timezone, sqlc.narg(app_code_challenge), @expires_at
);

-- name: ConsumeOAuthFlow :one
-- Single use: the row is deleted as it is read. Secrets are needed here to finish the exchange.
DELETE FROM oauth_flows
WHERE state_hash = @state_hash AND provider = @provider AND status = 'started' AND expires_at > now()
RETURNING intent, link_user_id, code_verifier, nonce, redirect_to, client, timezone, app_code_challenge;

-- name: CreatePendingOAuthFlow :exec
-- Second stage of a flow: 'awaiting_email' (Facebook gave no email) or 'ready'
-- (a mobile sign-in waiting for the app to redeem its one-time code).
INSERT INTO oauth_flows (
    id, state_hash, provider, intent, code_verifier, nonce, redirect_to, client, timezone,
    app_code_challenge, status, pending_identity, completion_token_hash, user_id, expires_at
) VALUES (
    @id, @state_hash, @provider, 'login', '', '', @redirect_to, @client, @timezone,
    sqlc.narg(app_code_challenge), @status, sqlc.narg(pending_identity), @completion_token_hash,
    sqlc.narg(user_id), @expires_at
);

-- name: ConsumePendingOAuthFlow :one
DELETE FROM oauth_flows
WHERE completion_token_hash = @completion_token_hash AND status = @status AND expires_at > now()
RETURNING provider, client, timezone, app_code_challenge, pending_identity, user_id;

-- name: SetTermsAccepted :exec
UPDATE users
SET terms_accepted_at = now(), terms_version = @terms_version
WHERE id = @id;

-- name: InvalidateEmailCodes :exec
UPDATE email_codes
SET used_at = now()
WHERE user_id = @user_id AND purpose = @purpose AND used_at IS NULL;

-- name: CreateEmailCode :exec
INSERT INTO email_codes (id, user_id, purpose, email, code_hash, expires_at)
VALUES (@id, @user_id, @purpose, @email, @code_hash, @expires_at);

-- name: GetActiveEmailCode :one
-- The only query that reads code_hash. Locks the row so concurrent tries count correctly.
SELECT id, code_hash, attempts
FROM email_codes
WHERE user_id = @user_id AND purpose = @purpose AND used_at IS NULL AND expires_at > now()
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE;

-- name: RecordEmailCodeAttempt :one
UPDATE email_codes
SET attempts = attempts + 1
WHERE id = @id
RETURNING attempts;

-- name: UseEmailCode :exec
UPDATE email_codes
SET used_at = now()
WHERE id = @id;

-- name: CountRecentEmailCodes :one
-- Codes sent to a user lately: resends are capped per account, not just per IP.
SELECT count(*)::int
FROM email_codes
WHERE user_id = @user_id AND purpose = @purpose AND created_at > @since;

-- name: CountPasswordFailures :one
-- Wrong passwords for one email (metadata.key) since @since and since its last
-- successful sign-in: from this IP, and from anywhere.
SELECT
    count(*) FILTER (WHERE e.ip = sqlc.narg(client_ip)::inet)::int AS from_ip,
    count(*)::int AS total
FROM auth_events e
WHERE e.metadata ? 'key'
  AND e.metadata ->> 'key' = @key::text
  AND e.event = 'login_failed'
  AND e.created_at > GREATEST(@since::timestamptz, COALESCE((
      SELECT max(s.created_at) FROM auth_events s
      WHERE s.metadata ? 'key' AND s.metadata ->> 'key' = @key::text AND s.event = 'login_succeeded'
  ), '-infinity'::timestamptz));

-- name: GetAwaitingOAuthFlow :one
-- The "add your email" step of a social sign-up. Locks the row while a code is sent.
SELECT id, client, app_code_challenge, email_code_sends
FROM oauth_flows
WHERE completion_token_hash = @completion_token_hash AND status = 'awaiting_email' AND expires_at > now()
FOR UPDATE;

-- name: SetOAuthFlowEmailCode :exec
UPDATE oauth_flows
SET pending_email = @pending_email, email_code_hash = @email_code_hash, email_code_expires_at = @email_code_expires_at,
    email_code_attempts = 0, email_code_sends = email_code_sends + 1
WHERE id = @id;

-- name: GetOAuthFlowEmailCode :one
-- The only query that reads oauth_flows.email_code_hash. Locks the row so tries count correctly.
SELECT id, provider, client, timezone, app_code_challenge, pending_identity,
       pending_email, email_code_hash, email_code_expires_at, email_code_attempts
FROM oauth_flows
WHERE completion_token_hash = @completion_token_hash AND status = 'awaiting_email' AND expires_at > now()
FOR UPDATE;

-- name: RecordOAuthFlowCodeAttempt :one
UPDATE oauth_flows
SET email_code_attempts = email_code_attempts + 1
WHERE id = @id
RETURNING email_code_attempts;

-- name: DeleteOAuthFlow :exec
DELETE FROM oauth_flows
WHERE id = @id;

-- Cleanup (CleanupWorker, hourly). Ended sessions are kept 30 days for the
-- sessions page history; their refresh tokens go with them (ON DELETE CASCADE).

-- name: DeleteExpiredOAuthFlows :execrows
DELETE FROM oauth_flows
WHERE expires_at < now();

-- name: DeleteExpiredEmailTokens :execrows
DELETE FROM email_tokens
WHERE expires_at < now();

-- name: DeleteExpiredEmailCodes :execrows
DELETE FROM email_codes
WHERE expires_at < now();

-- name: DeleteExpiredRefreshTokens :execrows
DELETE FROM refresh_tokens
WHERE expires_at < now();

-- name: DeleteEndedSessions :execrows
DELETE FROM auth_sessions
WHERE revoked_at < now() - interval '30 days'
   OR expires_at < now() - interval '30 days';

-- name: DeleteOldAuthEvents :execrows
-- auth_events is kept 180 days (00002_auth.sql).
DELETE FROM auth_events
WHERE created_at < now() - interval '180 days';
