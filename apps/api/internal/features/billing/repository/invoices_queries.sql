-- ---------- Invoices (kept forever) ----------

-- name: CreateInvoice :one
-- ON CONFLICT: a replayed webhook for the same payment adds nothing; no row back.
-- A paid invoice gets its number afterwards, in the same transaction (NextInvoiceSeq).
INSERT INTO invoices (workspace_id, subscription_id, order_id, installment_id, checkout_session_id, provider,
                      external_payment_id, status, subtotal_minor, tax_minor, total_minor, currency, period_start,
                      period_end, bill_to, discount_id, discount_minor, paid_at)
VALUES (@workspace_id, sqlc.narg('subscription_id'), sqlc.narg('order_id'), sqlc.narg('installment_id'),
        sqlc.narg('checkout_session_id'), @provider, sqlc.narg('external_payment_id'), @status, @subtotal_minor,
        @tax_minor, @total_minor, @currency, sqlc.narg('period_start'), sqlc.narg('period_end'), @bill_to,
        sqlc.narg('discount_id'), @discount_minor, sqlc.narg('paid_at'))
ON CONFLICT (provider, external_payment_id) DO NOTHING
RETURNING id;

-- name: NextInvoiceSeq :one
-- The year's next invoice number. The row lock holds until the transaction
-- ends, so a rollback hands the same number to the next paid invoice: no gaps.
INSERT INTO invoice_number_counters (year, last_seq)
VALUES (@year, 1)
ON CONFLICT (year) DO UPDATE SET last_seq = invoice_number_counters.last_seq + 1
RETURNING last_seq;

-- name: SetInvoiceNumber :execrows
UPDATE invoices
SET number = @number
WHERE workspace_id = @workspace_id AND id = @invoice_id AND number IS NULL;

-- name: GetInvoiceDocument :one
-- Everything the invoice PDF and its email show. The plan comes through the
-- subscription's current price or the order's price.
SELECT i.id, i.number, i.provider, i.status, i.subtotal_minor, i.discount_minor, i.tax_minor, i.total_minor,
       i.currency, i.period_start, i.period_end, i.bill_to, i.paid_at, i.pdf_key, i.emailed_at,
       i.order_id, i.installment_id, w.name AS workspace_name, pl.name AS plan_name, pp.interval AS plan_interval,
       inst.seq AS installment_seq,
       (SELECT count(*) FROM installments c WHERE c.workspace_id = i.workspace_id AND c.order_id = i.order_id)::int
           AS installment_count
FROM invoices i
JOIN workspaces w ON w.id = i.workspace_id
LEFT JOIN subscriptions s ON s.id = i.subscription_id AND s.workspace_id = i.workspace_id
LEFT JOIN orders o ON o.id = i.order_id AND o.workspace_id = i.workspace_id
JOIN plan_prices pp ON pp.id = COALESCE(s.plan_price_id, o.plan_price_id)
JOIN plans pl ON pl.id = pp.plan_id
LEFT JOIN installments inst ON inst.id = i.installment_id AND inst.workspace_id = i.workspace_id
WHERE i.workspace_id = @workspace_id AND i.id = @invoice_id;

-- name: SetInvoicePDFKey :execrows
UPDATE invoices
SET pdf_key = @pdf_key
WHERE workspace_id = @workspace_id AND id = @invoice_id AND pdf_key IS NULL;

-- name: ListInvoicesWithoutPDF :many
-- Numbered invoices created before @before that still have no PDF: their
-- document job ran out of attempts, or they predate invoice documents.
SELECT id, workspace_id
FROM invoices
WHERE number IS NOT NULL AND pdf_key IS NULL AND created_at < @before
ORDER BY created_at, id
LIMIT @max_rows;

-- name: MarkInvoiceEmailed :execrows
UPDATE invoices
SET emailed_at = @emailed_at
WHERE workspace_id = @workspace_id AND id = @invoice_id AND emailed_at IS NULL;

-- name: GetInvoice :one
SELECT id, number, subscription_id, order_id, installment_id, provider, status, subtotal_minor, tax_minor,
       total_minor, currency, period_start, period_end, bill_to, discount_minor, pdf_key, paid_at, created_at
FROM invoices
WHERE workspace_id = @workspace_id AND id = @invoice_id;

-- name: ListInvoices :many
-- Newest first. The cursor is the last row's (created_at, id); null starts at the top.
SELECT id, number, subscription_id, order_id, installment_id, provider, status, subtotal_minor, tax_minor,
       total_minor, currency, period_start, period_end, discount_minor, pdf_key, paid_at, created_at
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

-- name: GetOrderRefundTotals :one
-- What a lifetime deal has been paid across its invoices, and how much of it
-- has been refunded. A refunded duplicate payment counts on both sides.
SELECT COALESCE((SELECT sum(i.total_minor) FROM invoices i
                 WHERE i.workspace_id = @workspace_id AND i.order_id = @order_id
                   AND i.status IN ('paid', 'partially_refunded', 'refunded')), 0)::bigint AS paid_minor,
       COALESCE((SELECT sum(r.amount_minor) FROM refunds r JOIN invoices i ON i.id = r.invoice_id
                 WHERE i.workspace_id = @workspace_id AND i.order_id = @order_id
                   AND r.status = 'succeeded'), 0)::bigint AS refunded_minor;

-- name: GetLatestSubscriptionInvoiceID :one
-- The subscription's newest invoice: refunding it in full ends the subscription.
SELECT id
FROM invoices
WHERE workspace_id = @workspace_id AND subscription_id = @subscription_id
ORDER BY created_at DESC, id DESC
LIMIT 1;
