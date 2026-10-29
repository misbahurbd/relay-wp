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

-- name: ListAuthEvents :many
SELECT id, event, provider, ip, user_agent, created_at
FROM auth_events
WHERE user_id = @user_id
ORDER BY created_at DESC
LIMIT 50;
