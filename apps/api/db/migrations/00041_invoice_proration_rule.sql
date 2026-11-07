-- +goose Up
-- invoices.proration now follows Relay's one rule, for every provider: a
-- subscription payment whose period starts inside a period the subscription
-- was already invoiced for. Only Stripe invoices had it set (from Stripe's
-- billing reason); the PDF used this rule on its own.
UPDATE invoices i
SET proration = EXISTS (
    SELECT 1 FROM invoices prev
    WHERE prev.workspace_id = i.workspace_id AND prev.subscription_id = i.subscription_id
      AND prev.id <> i.id AND prev.created_at <= i.created_at
      AND prev.status IN ('paid', 'partially_refunded', 'refunded', 'disputed', 'dispute_lost')
      AND prev.period_start <= i.period_start AND i.period_start < prev.period_end)
WHERE i.subscription_id IS NOT NULL;

-- +goose Down
-- Nothing to undo: the column stays, and the rule's values are as good as any.
SELECT 1;
