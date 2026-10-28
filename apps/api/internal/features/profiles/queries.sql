-- Rules (CLAUDE.md): list columns explicitly, never SELECT *.

-- name: CreateProfile :exec
INSERT INTO user_profiles (user_id, first_name, last_name, avatar_url)
VALUES (@user_id, @first_name, @last_name, @avatar_url);

-- name: CreatePreferences :exec
INSERT INTO user_preferences (user_id, locale, timezone)
VALUES (@user_id, @locale, @timezone);

-- name: GetProfile :one
SELECT p.first_name, p.last_name, p.display_name, p.avatar_url, pr.locale, pr.timezone
FROM user_profiles p
JOIN user_preferences pr ON pr.user_id = p.user_id
WHERE p.user_id = @user_id;

-- name: UpdateProfile :exec
-- A NULL argument keeps the current value; an empty string clears an optional field.
UPDATE user_profiles
SET first_name   = COALESCE(sqlc.narg(first_name), first_name),
    last_name    = NULLIF(COALESCE(sqlc.narg(last_name), last_name), ''),
    display_name = NULLIF(COALESCE(sqlc.narg(display_name), display_name), '')
WHERE user_id = @user_id;

-- name: UpdatePreferences :exec
UPDATE user_preferences
SET locale   = COALESCE(sqlc.narg(locale), locale),
    timezone = COALESCE(sqlc.narg(timezone), timezone)
WHERE user_id = @user_id;
