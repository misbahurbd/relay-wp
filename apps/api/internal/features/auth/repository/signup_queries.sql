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
