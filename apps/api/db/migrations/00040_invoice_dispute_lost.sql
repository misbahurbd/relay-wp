-- +goose Up
-- A lost dispute is final: the money went back to the customer, so the
-- invoice leaves the open-dispute list and can't be closed again as won.
ALTER TABLE invoices DROP CONSTRAINT invoices_status_check;
ALTER TABLE invoices ADD CONSTRAINT invoices_status_check
    CHECK (status IN ('open', 'paid', 'failed', 'void', 'partially_refunded', 'refunded', 'disputed', 'dispute_lost'));

-- Disputes already lost: the invoice stayed disputed, and the plan it paid
-- for was suspended over the dispute. Nothing recorded which invoice lost,
-- so only a grant's one disputed invoice is marked; with two or more, one
-- may still be open, and they stay disputed for support to close.
UPDATE invoices i
SET status = 'dispute_lost'
FROM plan_grants g
WHERE i.status = 'disputed'
  AND g.workspace_id = i.workspace_id
  AND g.status = 'suspended'
  AND 'dispute' = ANY (g.suspend_reasons)
  AND (g.subscription_id = i.subscription_id OR g.order_id = i.order_id)
  AND NOT EXISTS (
      SELECT 1 FROM invoices other
      WHERE other.workspace_id = i.workspace_id AND other.id <> i.id AND other.status = 'disputed'
        AND (g.subscription_id = other.subscription_id OR g.order_id = other.order_id));

-- +goose Down
UPDATE invoices SET status = 'disputed' WHERE status = 'dispute_lost';
ALTER TABLE invoices DROP CONSTRAINT invoices_status_check;
ALTER TABLE invoices ADD CONSTRAINT invoices_status_check
    CHECK (status IN ('open', 'paid', 'failed', 'void', 'partially_refunded', 'refunded', 'disputed'));
