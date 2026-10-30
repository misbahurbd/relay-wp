-- +goose Up
-- Billing (docs/PHASE-6-BILLING.md): the plan catalog and discounts, provider
-- customers, subscriptions, lifetime deals paid in installments, the single
-- access model (plan_grants), money (checkout sessions, invoices, refunds) and
-- the webhook inbox. Built from the load-tested billing schema draft v12.
--
-- Every billing foreign key to workspaces is ON DELETE RESTRICT: financial
-- records outlive a workspace, which is soft-deleted instead.

ALTER TABLE workspaces ADD COLUMN deleted_at timestamptz;

-- A plan is never edited once sold: publish a new version instead.
-- No current grant means the free plan.
CREATE TABLE plans (
    id                  uuid        PRIMARY KEY DEFAULT uuidv7(),
    code                text        NOT NULL CHECK (code ~ '^[a-z][a-z0-9_]{1,40}$'),
    version             int         NOT NULL DEFAULT 1 CHECK (version > 0),
    name                text        NOT NULL,
    -- 0 turns trials off. A card-less trial is a plan_grants row with source = trial.
    trial_days          int         NOT NULL DEFAULT 0 CHECK (trial_days BETWEEN 0 AND 90),
    trial_requires_card bool        NOT NULL DEFAULT false,
    limits              jsonb       NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(limits) = 'object'),
    is_active           bool        NOT NULL DEFAULT true,
    sort_order          int         NOT NULL DEFAULT 0,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (code, version)
);

-- What the pricing page shows, the same for every provider. Rows are never
-- updated: deactivate and insert a replacement.
CREATE TABLE plan_prices (
    id           uuid        PRIMARY KEY DEFAULT uuidv7(),
    plan_id      uuid        NOT NULL REFERENCES plans (id) ON DELETE RESTRICT,
    billing_type text        NOT NULL CHECK (billing_type IN ('recurring', 'one_time')),
    interval     text        CHECK (interval IN ('month', 'year')),
    currency     text        NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    amount_minor bigint      NOT NULL CHECK (amount_minor >= 0),
    is_active    bool        NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CHECK ((billing_type = 'recurring') = (interval IS NOT NULL)),
    -- Target of installment_offers' composite FK.
    UNIQUE (id, billing_type)
);
CREATE UNIQUE INDEX plan_prices_active_uq ON plan_prices (plan_id, billing_type, interval, currency)
    NULLS NOT DISTINCT WHERE is_active;

-- A one-time price split into N payments. The composite FK only accepts one_time prices.
CREATE TABLE installment_offers (
    id            uuid        PRIMARY KEY DEFAULT uuidv7(),
    plan_price_id uuid        NOT NULL,
    price_type    text        NOT NULL DEFAULT 'one_time' CHECK (price_type = 'one_time'),
    count         int         NOT NULL CHECK (count BETWEEN 2 AND 24),
    interval      text        NOT NULL DEFAULT 'month' CHECK (interval IN ('month')),
    -- May include a markup over the one-time price; the last installment absorbs rounding.
    total_minor   bigint      NOT NULL CHECK (total_minor > 0),
    grace_days    int         NOT NULL DEFAULT 7 CHECK (grace_days BETWEEN 0 AND 60),
    max_missed    int         NOT NULL DEFAULT 2 CHECK (max_missed >= 1),
    is_active     bool        NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (plan_price_id, price_type) REFERENCES plan_prices (id, billing_type) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX installment_offers_active_uq ON installment_offers (plan_price_id, count, interval)
    WHERE is_active;

-- How each provider sells a price, or a price paid through an installment offer.
-- The amount lives only in plan_prices / installment_offers.
CREATE TABLE provider_prices (
    id                   uuid        PRIMARY KEY DEFAULT uuidv7(),
    plan_price_id        uuid        NOT NULL REFERENCES plan_prices (id) ON DELETE RESTRICT,
    installment_offer_id uuid        REFERENCES installment_offers (id) ON DELETE RESTRICT,
    provider             text        NOT NULL CHECK (provider IN ('stripe', 'creem', 'paypal', 'sslcommerz')),
    external_price_id    text,
    collection           text        CHECK (collection IN ('provider_schedule', 'app_link', 'provider_financed')),
    is_active            bool        NOT NULL DEFAULT true,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    CHECK ((installment_offer_id IS NULL) = (collection IS NULL)),
    UNIQUE (provider, external_price_id)
);
CREATE UNIQUE INDEX provider_prices_active_uq ON provider_prices (plan_price_id, installment_offer_id, provider)
    NULLS NOT DISTINCT WHERE is_active;

-- Relay owns every discount; providers only mirror the ones on subscriptions.
CREATE TABLE discounts (
    id                  uuid        PRIMARY KEY DEFAULT uuidv7(),
    code                text        NOT NULL UNIQUE CHECK (code ~ '^[A-Z0-9][A-Z0-9_-]{2,39}$'),
    name                text        NOT NULL,
    kind                text        NOT NULL CHECK (kind IN ('percent', 'amount')),
    -- Basis points: 2000 = 20%.
    percent_bps         int         CHECK (percent_bps BETWEEN 1 AND 10000),
    amount_off_minor    bigint      CHECK (amount_off_minor > 0),
    currency            text        CHECK (currency ~ '^[A-Z]{3}$'),
    -- One-time payments always apply once.
    duration            text        NOT NULL DEFAULT 'once' CHECK (duration IN ('once', 'repeating', 'forever')),
    duration_months     int         CHECK (duration_months BETWEEN 1 AND 36),
    applies_to          text        NOT NULL DEFAULT 'all' CHECK (applies_to IN ('all', 'recurring', 'one_time')),
    plan_id             uuid        REFERENCES plans (id) ON DELETE RESTRICT,
    allow_installments  bool        NOT NULL DEFAULT true,
    max_redemptions     int         CHECK (max_redemptions > 0),
    per_workspace_limit int         NOT NULL DEFAULT 1 CHECK (per_workspace_limit > 0),
    redeemed_count      int         NOT NULL DEFAULT 0 CHECK (redeemed_count >= 0),
    starts_at           timestamptz NOT NULL DEFAULT now(),
    expires_at          timestamptz,
    is_active           bool        NOT NULL DEFAULT true,
    created_by          uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CHECK ((kind = 'percent') = (percent_bps IS NOT NULL)),
    CHECK ((kind = 'amount') = (amount_off_minor IS NOT NULL AND currency IS NOT NULL)),
    CHECK ((duration = 'repeating') = (duration_months IS NOT NULL)),
    CHECK (max_redemptions IS NULL OR redeemed_count <= max_redemptions),
    CHECK (expires_at IS NULL OR expires_at > starts_at)
);

-- The provider coupon that mirrors a discount, for subscriptions the provider renews.
CREATE TABLE provider_discounts (
    id                         uuid        PRIMARY KEY DEFAULT uuidv7(),
    discount_id                uuid        NOT NULL REFERENCES discounts (id) ON DELETE RESTRICT,
    provider                   text        NOT NULL CHECK (provider IN ('stripe', 'creem', 'paypal', 'sslcommerz')),
    external_coupon_id         text        NOT NULL,
    external_promotion_code_id text,
    is_active                  bool        NOT NULL DEFAULT true,
    created_at                 timestamptz NOT NULL DEFAULT now(),
    updated_at                 timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, external_coupon_id)
);
CREATE UNIQUE INDEX provider_discounts_active_uq ON provider_discounts (discount_id, provider) WHERE is_active;

-- The workspace's billing details, the source for provider checkouts and for
-- invoices.bill_to. Personal data: never logged.
CREATE TABLE billing_profiles (
    id            uuid        PRIMARY KEY DEFAULT uuidv7(),
    workspace_id  uuid        NOT NULL UNIQUE REFERENCES workspaces (id) ON DELETE RESTRICT,
    legal_name    text        NOT NULL,
    email         text        NOT NULL,
    phone         text,
    address_line1 text,
    address_line2 text,
    city          text,
    state         text,
    postal_code   text,
    -- ISO 3166-1 alpha-2
    country       text        CHECK (country ~ '^[A-Z]{2}$'),
    tax_id_type   text,
    tax_id        text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CHECK ((tax_id IS NULL) = (tax_id_type IS NULL))
);

-- Only the provider's customer record; details live in billing_profiles.
CREATE TABLE billing_customers (
    id                   uuid        PRIMARY KEY DEFAULT uuidv7(),
    workspace_id         uuid        NOT NULL REFERENCES workspaces (id) ON DELETE RESTRICT,
    provider             text        NOT NULL CHECK (provider IN ('stripe', 'creem', 'paypal', 'sslcommerz')),
    external_customer_id text,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, provider),
    UNIQUE (provider, external_customer_id)
);

-- incomplete is embedded checkout before payment; it never counts as live.
CREATE TABLE subscriptions (
    id                       uuid        PRIMARY KEY DEFAULT uuidv7(),
    workspace_id             uuid        NOT NULL REFERENCES workspaces (id) ON DELETE RESTRICT,
    plan_price_id            uuid        NOT NULL REFERENCES plan_prices (id) ON DELETE RESTRICT,
    billing_customer_id      uuid        NOT NULL REFERENCES billing_customers (id) ON DELETE RESTRICT,
    provider                 text        NOT NULL CHECK (provider IN ('stripe', 'creem', 'paypal', 'sslcommerz')),
    external_subscription_id text,
    renewal_mode             text        NOT NULL DEFAULT 'provider' CHECK (renewal_mode IN ('provider', 'app')),
    status                   text        NOT NULL
                                         CHECK (status IN ('incomplete', 'trialing', 'active', 'past_due', 'canceled')),
    provider_status          text,
    current_period_start     timestamptz,
    current_period_end       timestamptz,
    grace_ends_at            timestamptz,
    cancel_at_period_end     bool        NOT NULL DEFAULT false,
    canceled_at              timestamptz,
    -- A downgrade waiting for the period end.
    pending_plan_price_id    uuid        REFERENCES plan_prices (id) ON DELETE RESTRICT,
    pending_change_at        timestamptz,
    discount_id              uuid        REFERENCES discounts (id) ON DELETE RESTRICT,
    discount_ends_at         timestamptz,
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),
    CHECK (current_period_end IS NULL OR current_period_end > current_period_start),
    CHECK ((pending_plan_price_id IS NULL) = (pending_change_at IS NULL)),
    CHECK (discount_ends_at IS NULL OR discount_id IS NOT NULL),
    UNIQUE (provider, external_subscription_id)
);
CREATE UNIQUE INDEX subscriptions_live_uq ON subscriptions (workspace_id)
    WHERE status IN ('trialing', 'active', 'past_due');
CREATE INDEX subscriptions_workspace_idx ON subscriptions (workspace_id, created_at DESC);
CREATE INDEX subscriptions_app_renewal_idx ON subscriptions (current_period_end)
    WHERE renewal_mode = 'app' AND status IN ('active', 'past_due');

-- Append-only history of plan changes.
CREATE TABLE subscription_changes (
    id              uuid        PRIMARY KEY DEFAULT uuidv7(),
    subscription_id uuid        NOT NULL REFERENCES subscriptions (id) ON DELETE RESTRICT,
    workspace_id    uuid        NOT NULL REFERENCES workspaces (id) ON DELETE RESTRICT,
    from_price_id   uuid        NOT NULL REFERENCES plan_prices (id) ON DELETE RESTRICT,
    to_price_id     uuid        NOT NULL REFERENCES plan_prices (id) ON DELETE RESTRICT,
    kind            text        NOT NULL CHECK (kind IN ('upgrade', 'downgrade', 'interval')),
    effective_at    timestamptz NOT NULL,
    proration_minor bigint      NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX subscription_changes_sub_idx ON subscription_changes (subscription_id, effective_at);

-- A one-time purchase such as a lifetime deal, paid at once or in installments.
CREATE TABLE orders (
    id                       uuid        PRIMARY KEY DEFAULT uuidv7(),
    workspace_id             uuid        NOT NULL REFERENCES workspaces (id) ON DELETE RESTRICT,
    plan_price_id            uuid        NOT NULL REFERENCES plan_prices (id) ON DELETE RESTRICT,
    installment_offer_id     uuid        REFERENCES installment_offers (id) ON DELETE RESTRICT,
    billing_customer_id      uuid        NOT NULL REFERENCES billing_customers (id) ON DELETE RESTRICT,
    provider                 text        NOT NULL CHECK (provider IN ('stripe', 'creem', 'paypal', 'sslcommerz')),
    external_ref             text,
    status                   text        NOT NULL
                                         CHECK (status IN ('pending', 'in_installments', 'paid', 'defaulted',
                                                           'upgraded', 'refunded', 'canceled')),
    total_minor              bigint      NOT NULL CHECK (total_minor > 0),
    discount_id              uuid        REFERENCES discounts (id) ON DELETE RESTRICT,
    discount_minor           bigint      NOT NULL DEFAULT 0 CHECK (discount_minor >= 0),
    -- Money carried in: unused subscription time, or what was paid on the upgraded order.
    credit_minor             bigint      NOT NULL DEFAULT 0 CHECK (credit_minor >= 0),
    paid_minor               bigint      NOT NULL DEFAULT 0 CHECK (paid_minor >= 0),
    currency                 text        NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    paid_in_full_at          timestamptz,
    defaulted_at             timestamptz,
    upgraded_from_order_id   uuid        REFERENCES orders (id) ON DELETE RESTRICT,
    replaced_subscription_id uuid        REFERENCES subscriptions (id) ON DELETE RESTRICT,
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),
    CHECK (discount_minor + credit_minor + paid_minor <= total_minor),
    CHECK (discount_minor = 0 OR discount_id IS NOT NULL),
    UNIQUE (provider, external_ref)
);
CREATE UNIQUE INDEX orders_live_uq ON orders (workspace_id) WHERE status IN ('in_installments', 'paid', 'defaulted');
CREATE INDEX orders_workspace_idx ON orders (workspace_id, created_at DESC);

-- One row per scheduled payment, all created at checkout.
CREATE TABLE installments (
    id               uuid        PRIMARY KEY DEFAULT uuidv7(),
    order_id         uuid        NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    workspace_id     uuid        NOT NULL REFERENCES workspaces (id) ON DELETE RESTRICT,
    seq              int         NOT NULL CHECK (seq >= 1),
    due_at           timestamptz NOT NULL,
    amount_minor     bigint      NOT NULL CHECK (amount_minor > 0),
    status           text        NOT NULL
                                 CHECK (status IN ('scheduled', 'due', 'paid', 'overdue', 'failed', 'waived', 'canceled')),
    paid_at          timestamptz,
    reminder_sent_at timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (order_id, seq),
    CHECK ((status = 'paid') = (paid_at IS NOT NULL))
);
CREATE INDEX installments_open_due_idx ON installments (due_at) WHERE status IN ('scheduled', 'due', 'overdue');

-- The single answer to "what can this workspace use". ends_at null means
-- lifetime. A missed installment suspends a grant; grants are never deleted.
CREATE TABLE plan_grants (
    id              uuid        PRIMARY KEY DEFAULT uuidv7(),
    workspace_id    uuid        NOT NULL REFERENCES workspaces (id) ON DELETE RESTRICT,
    plan_id         uuid        NOT NULL REFERENCES plans (id) ON DELETE RESTRICT,
    source          text        NOT NULL CHECK (source IN ('subscription', 'order', 'trial', 'manual')),
    subscription_id uuid        REFERENCES subscriptions (id) ON DELETE RESTRICT,
    order_id        uuid        REFERENCES orders (id) ON DELETE RESTRICT,
    status          text        NOT NULL CHECK (status IN ('active', 'suspended', 'ended')),
    limits          jsonb       NOT NULL CHECK (jsonb_typeof(limits) = 'object'),
    starts_at       timestamptz NOT NULL,
    ends_at         timestamptz,
    suspended_at    timestamptz,
    created_by      uuid        REFERENCES users (id) ON DELETE SET NULL,
    reason          text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CHECK ((source = 'subscription') = (subscription_id IS NOT NULL)),
    CHECK ((source = 'order') = (order_id IS NOT NULL)),
    CHECK (source <> 'trial' OR ends_at IS NOT NULL),
    CHECK (source <> 'manual' OR reason IS NOT NULL),
    CHECK ((status = 'suspended') = (suspended_at IS NOT NULL))
);
CREATE UNIQUE INDEX plan_grants_current_uq ON plan_grants (workspace_id) WHERE status IN ('active', 'suspended');
CREATE UNIQUE INDEX plan_grants_order_uq ON plan_grants (order_id) WHERE order_id IS NOT NULL;
CREATE INDEX plan_grants_subscription_idx ON plan_grants (subscription_id) WHERE subscription_id IS NOT NULL;
CREATE INDEX plan_grants_trial_end_idx ON plan_grants (ends_at) WHERE source = 'trial' AND status = 'active';

-- Our id goes to the provider as its reference; webhooks map back only through it.
-- The Elements client secret goes straight to the browser and is never stored.
CREATE TABLE checkout_sessions (
    id                   uuid        PRIMARY KEY DEFAULT uuidv7(),
    workspace_id         uuid        NOT NULL REFERENCES workspaces (id) ON DELETE RESTRICT,
    plan_price_id        uuid        NOT NULL REFERENCES plan_prices (id) ON DELETE RESTRICT,
    installment_offer_id uuid        REFERENCES installment_offers (id) ON DELETE RESTRICT,
    purpose              text        NOT NULL CHECK (purpose IN ('new', 'renewal', 'change', 'ltd', 'installment')),
    subscription_id      uuid        REFERENCES subscriptions (id) ON DELETE RESTRICT,
    order_id             uuid        REFERENCES orders (id) ON DELETE RESTRICT,
    installment_id       uuid        REFERENCES installments (id) ON DELETE RESTRICT,
    provider             text        NOT NULL CHECK (provider IN ('stripe', 'creem', 'paypal', 'sslcommerz')),
    external_session_id  text,
    status               text        NOT NULL DEFAULT 'pending'
                                     CHECK (status IN ('pending', 'completed', 'failed', 'expired')),
    amount_minor         bigint      NOT NULL CHECK (amount_minor >= 0),
    currency             text        NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    idempotency_key      text        NOT NULL,
    discount_id          uuid        REFERENCES discounts (id) ON DELETE RESTRICT,
    discount_minor       bigint      NOT NULL DEFAULT 0 CHECK (discount_minor >= 0),
    created_by           uuid        REFERENCES users (id) ON DELETE SET NULL,
    expires_at           timestamptz NOT NULL,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    CHECK (purpose <> 'installment' OR installment_id IS NOT NULL),
    CHECK (discount_minor = 0 OR discount_id IS NOT NULL),
    UNIQUE (workspace_id, idempotency_key),
    UNIQUE (provider, external_session_id)
);
CREATE INDEX checkout_sessions_pending_idx ON checkout_sessions (expires_at) WHERE status = 'pending';

-- Issued by Stripe and Creem at launch (number is theirs). bill_to is a copy
-- of billing_profiles taken at issue and never changed. Kept forever.
CREATE TABLE invoices (
    id                  uuid        PRIMARY KEY DEFAULT uuidv7(),
    workspace_id        uuid        NOT NULL REFERENCES workspaces (id) ON DELETE RESTRICT,
    number              text,
    subscription_id     uuid        REFERENCES subscriptions (id) ON DELETE RESTRICT,
    order_id            uuid        REFERENCES orders (id) ON DELETE RESTRICT,
    installment_id      uuid        REFERENCES installments (id) ON DELETE RESTRICT,
    checkout_session_id uuid        REFERENCES checkout_sessions (id) ON DELETE RESTRICT,
    provider            text        NOT NULL CHECK (provider IN ('stripe', 'creem', 'paypal', 'sslcommerz')),
    external_payment_id text,
    status              text        NOT NULL
                                    CHECK (status IN ('open', 'paid', 'failed', 'void', 'partially_refunded',
                                                      'refunded', 'disputed')),
    subtotal_minor      bigint      NOT NULL CHECK (subtotal_minor >= 0),
    tax_minor           bigint      NOT NULL DEFAULT 0 CHECK (tax_minor >= 0),
    total_minor         bigint      NOT NULL,
    currency            text        NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    period_start        timestamptz,
    period_end          timestamptz,
    bill_to             jsonb       NOT NULL,
    discount_id         uuid        REFERENCES discounts (id) ON DELETE RESTRICT,
    discount_minor      bigint      NOT NULL DEFAULT 0 CHECK (discount_minor >= 0),
    receipt_url         text,
    paid_at             timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CHECK (discount_minor <= subtotal_minor),
    CHECK (total_minor = subtotal_minor - discount_minor + tax_minor),
    CHECK (discount_minor = 0 OR discount_id IS NOT NULL),
    CHECK (num_nonnulls(subscription_id, order_id) = 1),
    CHECK (installment_id IS NULL OR order_id IS NOT NULL),
    UNIQUE (provider, external_payment_id)
);
CREATE INDEX invoices_workspace_idx ON invoices (workspace_id, created_at DESC, id DESC);
CREATE INDEX invoices_subscription_idx ON invoices (subscription_id) WHERE subscription_id IS NOT NULL;
CREATE INDEX invoices_order_idx ON invoices (order_id) WHERE order_id IS NOT NULL;
CREATE INDEX invoices_installment_idx ON invoices (installment_id) WHERE installment_id IS NOT NULL;

-- A code reserved at checkout, redeemed on payment, released if the checkout
-- fails or expires. Reserving bumps discounts.redeemed_count in the same statement.
CREATE TABLE discount_redemptions (
    id                  uuid        PRIMARY KEY DEFAULT uuidv7(),
    discount_id         uuid        NOT NULL REFERENCES discounts (id) ON DELETE RESTRICT,
    workspace_id        uuid        NOT NULL REFERENCES workspaces (id) ON DELETE RESTRICT,
    checkout_session_id uuid        NOT NULL UNIQUE REFERENCES checkout_sessions (id) ON DELETE RESTRICT,
    subscription_id     uuid        REFERENCES subscriptions (id) ON DELETE RESTRICT,
    order_id            uuid        REFERENCES orders (id) ON DELETE RESTRICT,
    status              text        NOT NULL DEFAULT 'reserved' CHECK (status IN ('reserved', 'redeemed', 'released')),
    amount_minor        bigint      NOT NULL CHECK (amount_minor >= 0),
    redeemed_at         timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CHECK ((status = 'redeemed') = (redeemed_at IS NOT NULL))
);
CREATE INDEX discount_redemptions_workspace_idx ON discount_redemptions (discount_id, workspace_id)
    WHERE status <> 'released';

-- Each partial refund is its own row.
CREATE TABLE refunds (
    id                 uuid        PRIMARY KEY DEFAULT uuidv7(),
    invoice_id         uuid        NOT NULL REFERENCES invoices (id) ON DELETE RESTRICT,
    workspace_id       uuid        NOT NULL REFERENCES workspaces (id) ON DELETE RESTRICT,
    provider           text        NOT NULL CHECK (provider IN ('stripe', 'creem', 'paypal', 'sslcommerz')),
    external_refund_id text,
    amount_minor       bigint      NOT NULL CHECK (amount_minor > 0),
    reason             text,
    status             text        NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed')),
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, external_refund_id)
);
CREATE INDEX refunds_invoice_idx ON refunds (invoice_id);

-- The webhook inbox. River owns retries; only processed_at is kept.
-- payload is never logged or returned, and is nulled after 90 days.
CREATE TABLE payment_events (
    id                  uuid        PRIMARY KEY DEFAULT uuidv7(),
    provider            text        NOT NULL CHECK (provider IN ('stripe', 'creem', 'paypal', 'sslcommerz')),
    external_event_id   text        NOT NULL,
    event_type          text        NOT NULL,
    workspace_id        uuid        REFERENCES workspaces (id) ON DELETE RESTRICT,
    checkout_session_id uuid        REFERENCES checkout_sessions (id) ON DELETE RESTRICT,
    payload             jsonb,
    received_at         timestamptz NOT NULL DEFAULT now(),
    processed_at        timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, external_event_id)
);
CREATE INDEX payment_events_unprocessed_idx ON payment_events (received_at) WHERE processed_at IS NULL;
CREATE INDEX payment_events_purge_idx ON payment_events (received_at) WHERE payload IS NOT NULL;
-- The support view of a workspace's webhook events (a full scan without it in the load test).
CREATE INDEX payment_events_workspace_idx ON payment_events (workspace_id, received_at DESC)
    WHERE workspace_id IS NOT NULL;

CREATE TRIGGER plans_set_updated_at BEFORE UPDATE ON plans
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER plan_prices_set_updated_at BEFORE UPDATE ON plan_prices
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER installment_offers_set_updated_at BEFORE UPDATE ON installment_offers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER provider_prices_set_updated_at BEFORE UPDATE ON provider_prices
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER discounts_set_updated_at BEFORE UPDATE ON discounts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER provider_discounts_set_updated_at BEFORE UPDATE ON provider_discounts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER billing_profiles_set_updated_at BEFORE UPDATE ON billing_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER billing_customers_set_updated_at BEFORE UPDATE ON billing_customers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER subscriptions_set_updated_at BEFORE UPDATE ON subscriptions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER subscription_changes_set_updated_at BEFORE UPDATE ON subscription_changes
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER orders_set_updated_at BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER installments_set_updated_at BEFORE UPDATE ON installments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER plan_grants_set_updated_at BEFORE UPDATE ON plan_grants
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER checkout_sessions_set_updated_at BEFORE UPDATE ON checkout_sessions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER invoices_set_updated_at BEFORE UPDATE ON invoices
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER discount_redemptions_set_updated_at BEFORE UPDATE ON discount_redemptions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER refunds_set_updated_at BEFORE UPDATE ON refunds
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER payment_events_set_updated_at BEFORE UPDATE ON payment_events
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE payment_events;
DROP TABLE refunds;
DROP TABLE discount_redemptions;
DROP TABLE invoices;
DROP TABLE checkout_sessions;
DROP TABLE plan_grants;
DROP TABLE installments;
DROP TABLE orders;
DROP TABLE subscription_changes;
DROP TABLE subscriptions;
DROP TABLE billing_customers;
DROP TABLE billing_profiles;
DROP TABLE provider_discounts;
DROP TABLE discounts;
DROP TABLE provider_prices;
DROP TABLE installment_offers;
DROP TABLE plan_prices;
DROP TABLE plans;
ALTER TABLE workspaces DROP COLUMN deleted_at;
