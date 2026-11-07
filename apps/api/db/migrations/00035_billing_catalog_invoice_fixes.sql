-- +goose Up
-- A provider may give one customer ID to several workspaces (Creem reuses one
-- per email). Nothing looks a workspace up by it; each workspace keeps one
-- customer row per provider.
ALTER TABLE billing_customers DROP CONSTRAINT billing_customers_provider_external_customer_id_key;

-- The price an invoice was paid for, fixed when it is recorded, so its PDF
-- names that plan even after the subscription moves to another price.
ALTER TABLE invoices ADD COLUMN plan_price_id uuid REFERENCES plan_prices (id) ON DELETE RESTRICT;

UPDATE invoices i
SET plan_price_id = COALESCE(cs.plan_price_id, s.plan_price_id, o.plan_price_id)
FROM invoices x
LEFT JOIN subscriptions s ON s.id = x.subscription_id AND s.workspace_id = x.workspace_id
LEFT JOIN orders o ON o.id = x.order_id AND o.workspace_id = x.workspace_id
LEFT JOIN checkout_sessions cs ON cs.id = x.checkout_session_id AND cs.workspace_id = x.workspace_id
WHERE i.id = x.id;

-- The invoice document sweep's bookkeeping: how often it queued the invoice
-- again, and when last. It backs off, so an invoice that keeps failing can't
-- take every slot of a sweep.
ALTER TABLE invoices
    ADD COLUMN document_sweeps   int         NOT NULL DEFAULT 0 CHECK (document_sweeps >= 0),
    ADD COLUMN document_swept_at timestamptz;

-- The sweep's work list: numbered invoices with a PDF or receipt email still to deliver.
DROP INDEX invoices_pdf_pending_idx;
CREATE INDEX invoices_document_pending_idx ON invoices (document_swept_at NULLS FIRST, created_at)
    WHERE number IS NOT NULL AND (pdf_key IS NULL OR emailed_at IS NULL);

-- When a retired mirror was deleted on the provider, so a code Relay no longer
-- honours can't be typed into the provider's own checkout. Null on an inactive
-- row means the deletion is still to do.
ALTER TABLE provider_discounts ADD COLUMN deleted_on_provider_at timestamptz;

-- A client-chosen key for creating a plan, so a CRM publish that is retried
-- (a reload, a lost reply) finds the plan it already created.
ALTER TABLE plans ADD COLUMN publish_key uuid UNIQUE;

-- A price is sold as is: deactivate it and insert a replacement (case 69).
-- +goose StatementBegin
CREATE FUNCTION plan_prices_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF (NEW.plan_id, NEW.billing_type, NEW.interval, NEW.currency, NEW.amount_minor)
        IS DISTINCT FROM (OLD.plan_id, OLD.billing_type, OLD.interval, OLD.currency, OLD.amount_minor) THEN
        RAISE EXCEPTION 'plan price % never changes: deactivate it and add a replacement', OLD.id
            USING ERRCODE = 'restrict_violation';
    END IF;
    RETURN NEW;
END
$$;
-- +goose StatementEnd

CREATE TRIGGER plan_prices_immutable
    BEFORE UPDATE OF plan_id, billing_type, interval, currency, amount_minor ON plan_prices
    FOR EACH ROW EXECUTE FUNCTION plan_prices_immutable();

-- +goose Down
DROP TRIGGER plan_prices_immutable ON plan_prices;
DROP FUNCTION plan_prices_immutable();

ALTER TABLE plans DROP COLUMN publish_key;

ALTER TABLE provider_discounts DROP COLUMN deleted_on_provider_at;

DROP INDEX invoices_document_pending_idx;
CREATE INDEX invoices_pdf_pending_idx ON invoices (created_at) WHERE number IS NOT NULL AND pdf_key IS NULL;
ALTER TABLE invoices
    DROP COLUMN document_swept_at,
    DROP COLUMN document_sweeps,
    DROP COLUMN plan_price_id;

ALTER TABLE billing_customers ADD CONSTRAINT billing_customers_provider_external_customer_id_key
    UNIQUE (provider, external_customer_id);
