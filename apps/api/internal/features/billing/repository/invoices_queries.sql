-- ---------- Invoices (kept forever) ----------

-- name: CreateInvoice :one
-- ON CONFLICT: a replayed webhook for the same payment adds nothing; no row back.
INSERT INTO invoices (workspace_id, number, subscription_id, order_id, installment_id, checkout_session_id, provider,
                      external_payment_id, status, subtotal_minor, tax_minor, total_minor, currency, period_start,
                      period_end, bill_to, discount_id, discount_minor, receipt_url, paid_at)
VALUES (@workspace_id, sqlc.narg('number'), sqlc.narg('subscription_id'), sqlc.narg('order_id'),
        sqlc.narg('installment_id'), sqlc.narg('checkout_session_id'), @provider, sqlc.narg('external_payment_id'),
        @status, @subtotal_minor, @tax_minor, @total_minor, @currency, sqlc.narg('period_start'),
        sqlc.narg('period_end'), @bill_to, sqlc.narg('discount_id'), @discount_minor, sqlc.narg('receipt_url'),
        sqlc.narg('paid_at'))
ON CONFLICT (provider, external_payment_id) DO NOTHING
RETURNING id;

-- name: GetInvoice :one
SELECT id, number, subscription_id, order_id, installment_id, provider, status, subtotal_minor, tax_minor,
       total_minor, currency, period_start, period_end, bill_to, discount_minor, receipt_url, paid_at, created_at
FROM invoices
WHERE workspace_id = @workspace_id AND id = @invoice_id;

-- name: ListInvoices :many
-- Newest first. The cursor is the last row's (created_at, id); null starts at the top.
SELECT id, number, subscription_id, order_id, installment_id, provider, status, subtotal_minor, tax_minor,
       total_minor, currency, period_start, period_end, discount_minor, receipt_url, paid_at, created_at
FROM invoices
WHERE workspace_id = @workspace_id
  AND (sqlc.narg('cursor_created_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_created_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
ORDER BY created_at DESC, id DESC
LIMIT @max_rows;

-- name: FindInvoiceByExternalPaymentID :one
-- Webhooks only: refunds and disputes name the provider's payment.
SELECT id, workspace_id, subscription_id, order_id, total_minor, status
FROM invoices
WHERE provider = @provider AND external_payment_id = @external_payment_id;

-- name: SetInvoiceStatus :execrows
UPDATE invoices
SET status = @status, paid_at = COALESCE(sqlc.narg('paid_at'), paid_at)
WHERE workspace_id = @workspace_id AND id = @invoice_id;

-- ---------- Refunds ----------

-- name: CreateRefund :one
INSERT INTO refunds (invoice_id, workspace_id, provider, external_refund_id, amount_minor, reason, status)
VALUES (@invoice_id, @workspace_id, @provider, sqlc.narg('external_refund_id'), @amount_minor, sqlc.narg('reason'),
        @status)
ON CONFLICT (provider, external_refund_id) DO NOTHING
RETURNING id;

-- name: SetRefundStatus :execrows
UPDATE refunds
SET status = @status
WHERE workspace_id = @workspace_id AND provider = @provider AND external_refund_id = @external_refund_id;

-- name: SumInvoiceRefunds :one
-- Pending and succeeded refunds; never more than the invoice total.
SELECT COALESCE(sum(amount_minor), 0)::bigint
FROM refunds
WHERE workspace_id = @workspace_id AND invoice_id = @invoice_id AND status <> 'failed';
