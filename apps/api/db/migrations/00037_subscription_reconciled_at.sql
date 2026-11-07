-- +goose Up
-- When an overdue provider-renewed subscription was last read back from its
-- provider. The next read waits as long as the subscription has been overdue
-- (an hour, then two, four, ... up to a day), so one the provider leaves
-- unchanged isn't read every hour for good. A new period starts over.
ALTER TABLE subscriptions ADD COLUMN reconciled_at timestamptz;

-- +goose Down
ALTER TABLE subscriptions DROP COLUMN reconciled_at;
