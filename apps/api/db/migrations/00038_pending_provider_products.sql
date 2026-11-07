-- +goose Up
-- A product (or price) a provider made for a plan price whose link isn't
-- saved yet. It is recorded as soon as the provider returns it and deleted
-- with the link's insert, so a link that fails to save is retried with the
-- same product instead of leaving an orphan on the provider (Creem takes no
-- idempotency key). Catalog data: no workspace.
CREATE TABLE pending_provider_products (
    plan_price_id     uuid        NOT NULL REFERENCES plan_prices (id) ON DELETE CASCADE,
    provider          text        NOT NULL CHECK (provider IN ('stripe', 'creem', 'paypal', 'sslcommerz')),
    external_price_id text        NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (plan_price_id, provider)
);

-- +goose Down
DROP TABLE pending_provider_products;
