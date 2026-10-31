-- +goose Up
-- What an abandoned-cart run brought back: the order placed after its reminder.
ALTER TABLE automation_runs ADD COLUMN recovered_minor bigint;
ALTER TABLE automation_runs ADD COLUMN recovered_currency text;

-- +goose Down
ALTER TABLE automation_runs DROP COLUMN recovered_currency;
ALTER TABLE automation_runs DROP COLUMN recovered_minor;
