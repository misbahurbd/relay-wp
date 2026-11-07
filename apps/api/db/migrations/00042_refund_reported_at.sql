-- +goose Up
-- When the stuck report first logged a refund Relay asked for and never got
-- sent (its job gave up), so each one is reported once. Existing rows start
-- unreported: an old unsent refund is reported on the next run.
ALTER TABLE refunds ADD COLUMN reported_at timestamptz;
CREATE INDEX refunds_unsent_idx ON refunds (created_at)
    WHERE status = 'pending' AND issued_at IS NULL AND external_refund_id IS NULL AND reported_at IS NULL;

-- +goose Down
DROP INDEX refunds_unsent_idx;
ALTER TABLE refunds DROP COLUMN reported_at;
