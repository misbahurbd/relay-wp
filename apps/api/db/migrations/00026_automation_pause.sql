-- +goose Up
-- When an automation was turned off, so resuming a drip can shift its dates.
ALTER TABLE automations ADD COLUMN paused_at timestamptz;

-- +goose Down
ALTER TABLE automations DROP COLUMN paused_at;
