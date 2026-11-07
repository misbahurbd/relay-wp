-- +goose Up
-- A proration invoice charges for a mid-period plan change. Refunding it in
-- full gives back the change, not the period, so it never ends the plan.
ALTER TABLE invoices ADD COLUMN proration boolean NOT NULL DEFAULT false;

-- When the stuck-event report first logged an event, so each one is reported once.
ALTER TABLE payment_events ADD COLUMN reported_at timestamptz;

-- +goose Down
ALTER TABLE payment_events DROP COLUMN reported_at;
ALTER TABLE invoices DROP COLUMN proration;
