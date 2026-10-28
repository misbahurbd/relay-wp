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
    u.status AS user_status
FROM refresh_tokens rt
JOIN auth_sessions s ON s.id = rt.session_id
JOIN users u ON u.id = s.user_id
WHERE rt.token_hash = @token_hash
FOR UPDATE OF rt;

-- name: MarkRefreshTokenUsed :exec
UPDATE refresh_tokens
SET used_at = now()
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
-- Checked on every authenticated request, so a revoked session, an expired one
-- or a disabled user stops working at once instead of when the access token expires.
SELECT EXISTS (
    SELECT 1
    FROM auth_sessions s
    JOIN users u ON u.id = s.user_id
    WHERE s.id = @id
      AND s.user_id = @user_id
      AND s.revoked_at IS NULL
      AND s.expires_at > now()
      AND u.status = 'active'
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
