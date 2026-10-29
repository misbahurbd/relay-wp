-- name: SetPasswordIfUnset :execrows
-- POST /auth/password/set: only for accounts that have no password yet.
UPDATE users
SET password_hash = @password_hash
WHERE id = @id AND password_hash IS NULL;
