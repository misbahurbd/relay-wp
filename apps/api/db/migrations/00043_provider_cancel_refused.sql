-- +goose Up
-- When the provider refused for good to stop billing for a subscription or an
-- installment schedule Relay had ended (a refund, a lost dispute, a default).
-- Relay keeps its own change; the hourly sweep asks the provider again until
-- it takes the cancel, then clears this. Existing rows have none recorded.
ALTER TABLE subscriptions ADD COLUMN provider_cancel_refused_at timestamptz;
ALTER TABLE orders ADD COLUMN provider_cancel_refused_at timestamptz;
CREATE INDEX subscriptions_provider_cancel_refused_idx ON subscriptions (provider_cancel_refused_at)
    WHERE provider_cancel_refused_at IS NOT NULL;
CREATE INDEX orders_provider_cancel_refused_idx ON orders (provider_cancel_refused_at)
    WHERE provider_cancel_refused_at IS NOT NULL;

-- +goose Down
DROP INDEX orders_provider_cancel_refused_idx;
DROP INDEX subscriptions_provider_cancel_refused_idx;
ALTER TABLE orders DROP COLUMN provider_cancel_refused_at;
ALTER TABLE subscriptions DROP COLUMN provider_cancel_refused_at;
