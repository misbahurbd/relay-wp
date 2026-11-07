-- +goose Up
-- A refund Relay asks for is recorded (pending, with no provider ID) and
-- committed before the provider is called, by a job queued with it. issued_at
-- is when the provider accepted the request; a row with it set is never sent
-- again. The provider's refund webhook then fills in external_refund_id.
ALTER TABLE refunds ADD COLUMN issued_at timestamptz;

-- +goose Down
ALTER TABLE refunds DROP COLUMN issued_at;
