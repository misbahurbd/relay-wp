-- +goose Up
-- A social sign-up whose provider doesn't vouch for the email (Facebook) must
-- prove it with an emailed code before the account exists. The awaiting_email
-- flow keeps the typed address and the code:
--   pending_email          the address the person typed
--   email_code_hash        secret: HMAC of the 6-digit code
--   email_code_expires_at  the code's own deadline (the flow lives longer)
--   email_code_attempts    wrong tries on the current code
--   email_code_sends       codes sent for this flow
ALTER TABLE oauth_flows
    ADD COLUMN pending_email         text,
    ADD COLUMN email_code_hash       bytea,
    ADD COLUMN email_code_expires_at timestamptz,
    ADD COLUMN email_code_attempts   int NOT NULL DEFAULT 0,
    ADD COLUMN email_code_sends      int NOT NULL DEFAULT 0;

-- Password failures are counted from auth_events by a keyed hash of the email
-- (metadata.key), so limits hold across instances and restarts.
CREATE INDEX auth_events_key_idx ON auth_events ((metadata ->> 'key'), created_at)
    WHERE metadata ? 'key';

-- +goose Down
DROP INDEX auth_events_key_idx;
ALTER TABLE oauth_flows
    DROP COLUMN email_code_sends,
    DROP COLUMN email_code_attempts,
    DROP COLUMN email_code_expires_at,
    DROP COLUMN email_code_hash,
    DROP COLUMN pending_email;
