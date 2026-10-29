-- Rules (CLAUDE.md): list columns explicitly, never SELECT *. password_hash and
-- token hashes are selected only where they are needed.

-- name: CreateUser :exec
INSERT INTO users (id, email, password_hash)
VALUES (@id, @email, @password_hash);

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

-- name: InsertAuthEvent :exec
INSERT INTO auth_events (id, user_id, event, provider, ip, user_agent, metadata)
VALUES (@id, @user_id, @event, @provider, @ip, @user_agent, @metadata);

-- name: SetTermsAccepted :exec
UPDATE users
SET terms_accepted_at = now(), terms_version = @terms_version
WHERE id = @id;
