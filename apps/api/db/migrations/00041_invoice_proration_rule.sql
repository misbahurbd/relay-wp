-- +goose Up
-- invoices.proration now follows Relay's one rule, for every provider: a
-- subscription payment whose period lies inside a period the subscription
-- was already invoiced for (an interval switch runs past it, so it isn't
-- one). Only Stripe invoices had it set (from Stripe's billing reason); the
-- PDF used this rule on its own.
--
-- Only rows whose flag changes are written: the updated_at trigger would
-- otherwise restamp every invoice, and a disputed one's updated_at is when
-- its dispute opened.
UPDATE invoices i
SET proration = r.proration
FROM (
    SELECT cur.id, EXISTS (
        SELECT 1 FROM invoices prev
        WHERE prev.workspace_id = cur.workspace_id AND prev.subscription_id = cur.subscription_id
          AND prev.id <> cur.id AND prev.created_at <= cur.created_at
          AND prev.status IN ('paid', 'partially_refunded', 'refunded', 'disputed', 'dispute_lost')
          AND prev.period_start <= cur.period_start AND cur.period_start < prev.period_end
          AND cur.period_end <= prev.period_end) AS proration
    FROM invoices cur
    WHERE cur.subscription_id IS NOT NULL
) r
WHERE i.id = r.id AND i.proration IS DISTINCT FROM r.proration;

-- +goose Down
-- Nothing to undo: the column stays, and the rule's values are as good as any.
SELECT 1;
