-- ---------- Orders (lifetime deals and other one-time purchases) ----------

-- name: CreateOrder :one
INSERT INTO orders (workspace_id, plan_price_id, installment_offer_id, billing_customer_id, provider, status,
                    total_minor, discount_id, discount_minor, credit_minor, currency, upgraded_from_order_id,
                    replaced_subscription_id)
VALUES (@workspace_id, @plan_price_id, sqlc.narg('installment_offer_id'), @billing_customer_id, @provider, @status,
        @total_minor, sqlc.narg('discount_id'), @discount_minor, @credit_minor, @currency,
        sqlc.narg('upgraded_from_order_id'), sqlc.narg('replaced_subscription_id'))
RETURNING id;

-- name: GetOrder :one
SELECT id, workspace_id, plan_price_id, installment_offer_id, billing_customer_id, provider, external_ref, status,
       total_minor, discount_id, discount_minor, credit_minor, paid_minor, currency, paid_in_full_at, defaulted_at,
       upgraded_from_order_id, replaced_subscription_id, created_at
FROM orders
WHERE workspace_id = @workspace_id AND id = @order_id;

-- name: LockOrder :one
-- Every balance change locks the order first.
SELECT id, workspace_id, plan_price_id, installment_offer_id, billing_customer_id, provider, external_ref, status,
       total_minor, discount_id, discount_minor, credit_minor, paid_minor, currency, paid_in_full_at, defaulted_at,
       upgraded_from_order_id, replaced_subscription_id, created_at
FROM orders
WHERE workspace_id = @workspace_id AND id = @order_id
FOR UPDATE;

-- name: GetLiveOrder :one
-- The workspace's one in_installments, paid or defaulted order.
SELECT id, workspace_id, plan_price_id, installment_offer_id, billing_customer_id, provider, external_ref, status,
       total_minor, discount_id, discount_minor, credit_minor, paid_minor, currency, paid_in_full_at, defaulted_at,
       upgraded_from_order_id, replaced_subscription_id, created_at
FROM orders
WHERE workspace_id = @workspace_id AND status IN ('in_installments', 'paid', 'defaulted');

-- name: FindOrderByExternalRef :one
-- Webhooks only: installment payments carry the provider's schedule ID, which we stored.
SELECT id, workspace_id
FROM orders
WHERE provider = @provider AND external_ref = @external_ref;

-- name: SetOrderExternalRef :execrows
UPDATE orders
SET external_ref = @external_ref
WHERE workspace_id = @workspace_id AND id = @order_id AND external_ref IS NULL;

-- name: SetOrderStatus :execrows
-- paid stamps paid_in_full_at once; defaulted stamps defaulted_at; going back
-- to in_installments (a resumed order) clears it.
UPDATE orders
SET status          = @status,
    paid_in_full_at = CASE WHEN @status = 'paid' THEN COALESCE(paid_in_full_at, @now::timestamptz) ELSE paid_in_full_at END,
    defaulted_at    = CASE WHEN @status = 'defaulted' THEN @now::timestamptz
                           WHEN @status = 'in_installments' THEN NULL
                           ELSE defaulted_at END
WHERE workspace_id = @workspace_id AND id = @order_id;

-- name: AddOrderPayment :one
-- Runs in the same transaction as the paid invoice. The CHECK on orders
-- rejects paying more than the total.
UPDATE orders
SET paid_minor = paid_minor + @amount_minor
WHERE workspace_id = @workspace_id AND id = @order_id
RETURNING paid_minor, total_minor, discount_minor, credit_minor;
