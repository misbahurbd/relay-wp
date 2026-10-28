-- +goose Up
-- Six-digit verification codes sent at sign-up. Codes are short, so they are
-- stored as an HMAC with a server key (EMAIL_CODE_KEY), not a plain hash, and
-- each code allows a few wrong tries.
CREATE TABLE email_codes (
    id         uuid        PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose    text        NOT NULL CHECK (purpose IN ('verify')),
    email      citext      NOT NULL,
    -- Secret: HMAC-SHA256 of the code.
    code_hash  bytea       NOT NULL,
    attempts   int         NOT NULL DEFAULT 0,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_codes_unused_idx ON email_codes (user_id, purpose) WHERE used_at IS NULL;
CREATE INDEX email_codes_expires_at_idx ON email_codes (expires_at);

-- Which terms the person accepted at sign-up. NULL for accounts created before this.
ALTER TABLE users
    ADD COLUMN terms_accepted_at timestamptz,
    ADD COLUMN terms_version     text;

-- +goose Down
ALTER TABLE users
    DROP COLUMN terms_version,
    DROP COLUMN terms_accepted_at;
DROP TABLE email_codes;
