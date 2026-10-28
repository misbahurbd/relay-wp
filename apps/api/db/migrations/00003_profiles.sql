-- +goose Up
-- What the frontend shows about a person. One row per user.
CREATE TABLE user_profiles (
    user_id      uuid        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    first_name   text        NOT NULL,
    last_name    text,
    -- Optional override; when NULL the UI shows "first_name last_name".
    display_name text,
    avatar_url   text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER user_profiles_set_updated_at BEFORE UPDATE ON user_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- How the app behaves for a person. One row per user.
CREATE TABLE user_preferences (
    user_id    uuid        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    locale     text        NOT NULL DEFAULT 'en',
    timezone   text        NOT NULL DEFAULT 'UTC',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER user_preferences_set_updated_at BEFORE UPDATE ON user_preferences
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE user_preferences;
DROP TABLE user_profiles;
