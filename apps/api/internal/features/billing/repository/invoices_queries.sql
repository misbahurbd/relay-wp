-- ---------- Invoices (kept forever) ----------

-- name: CreateInvoice :one
-- ON CONFLICT: a replayed webhook for the same payment adds nothing; no row back.
-- A paid invoice gets its number afterwards, in the same transaction (NextInvoiceSeq).
-- plan_price_id is what the payment bought: the checkout's price, else the
-- order's, else the subscription's, or its scheduled downgrade when this
-- period starts it (the renewal applies it next, in this transaction).
INSERT INTO invoices (workspace_id, subscription_id, order_id, installment_id, checkout_session_id, provider,
                      external_payment_id, status, subtotal_minor, tax_minor, total_minor, currency, period_start,
                      period_end, bill_to, discount_id, discount_minor, paid_at, proration, plan_price_id)
VALUES (@workspace_id, sqlc.narg('subscription_id'), sqlc.narg('order_id'), sqlc.narg('installment_id'),
        sqlc.narg('checkout_session_id'), @provider, sqlc.narg('external_payment_id'), @status, @subtotal_minor,
        @tax_minor, @total_minor, @currency, sqlc.narg('period_start'), sqlc.narg('period_end'), @bill_to,
        sqlc.narg('discount_id'), @discount_minor, sqlc.narg('paid_at'), @proration,
        COALESCE(
            (SELECT cs.plan_price_id FROM checkout_sessions cs
             WHERE cs.workspace_id = @workspace_id AND cs.id = sqlc.narg('checkout_session_id')),
            (SELECT o.plan_price_id FROM orders o
             WHERE o.workspace_id = @workspace_id AND o.id = sqlc.narg('order_id')),
            (SELECT CASE
                        WHEN s.pending_plan_price_id IS NOT NULL AND s.status <> 'canceled'
                            AND s.pending_change_at <= COALESCE(sqlc.narg('period_start'), now())
                            THEN s.pending_plan_price_id
                        ELSE s.plan_price_id
                    END
             FROM subscriptions s
             WHERE s.workspace_id = @workspace_id AND s.id = sqlc.narg('subscription_id'))))
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
-- Everything the invoice PDF and its email show. The plan is the price the
-- invoice was paid for. A subscription invoice that starts inside a period an
-- earlier one already paid for is a plan change (a proration).
SELECT i.id, i.number, i.provider, i.status, i.subtotal_minor, i.discount_minor, i.tax_minor, i.total_minor,
       i.currency, i.period_start, i.period_end, i.bill_to, i.paid_at, i.pdf_key, i.emailed_at,
       i.order_id, i.installment_id, w.name AS workspace_name, pl.name AS plan_name, pp.interval AS plan_interval,
       EXISTS (SELECT 1 FROM invoices prev
               WHERE prev.workspace_id = i.workspace_id AND prev.subscription_id = i.subscription_id
                 AND prev.id <> i.id AND prev.created_at <= i.created_at
                 AND prev.status IN ('paid', 'partially_refunded', 'refunded', 'disputed')
                 AND prev.period_start <= i.period_start AND i.period_start < prev.period_end) AS prorated,
       inst.seq AS installment_seq,
       (SELECT count(*) FROM installments c WHERE c.workspace_id = i.workspace_id AND c.order_id = i.order_id)::int
           AS installment_count
FROM invoices i
JOIN workspaces w ON w.id = i.workspace_id
JOIN plan_prices pp ON pp.id = i.plan_price_id
JOIN plans pl ON pl.id = pp.plan_id
LEFT JOIN installments inst ON inst.id = i.installment_id AND inst.workspace_id = i.workspace_id
WHERE i.workspace_id = @workspace_id AND i.id = @invoice_id;

-- name: SetInvoicePDFKey :execrows
UPDATE invoices
SET pdf_key = @pdf_key
WHERE workspace_id = @workspace_id AND id = @invoice_id AND pdf_key IS NULL;

-- name: ClaimInvoicesToDeliver :many
-- Numbered invoices created before `before` whose PDF, or receipt email to a
-- billing contact (none for a refunded payment), is still undelivered: their document job ran out of
-- attempts, or they predate invoice documents. Each claim counts a sweep and
-- the next waits twice as long (15 minutes, then 30, up to a day), and
-- the least recently swept go first, so failing invoices can't crowd out new ones.
WITH due AS (
    SELECT x.id
    FROM invoices x
    WHERE x.number IS NOT NULL AND x.created_at < sqlc.arg(before)::timestamptz
      AND (x.pdf_key IS NULL OR x.emailed_at IS NULL)
      AND (x.pdf_key IS NULL OR (COALESCE(x.bill_to ->> 'email', '') <> '' AND x.status <> 'refunded'))
      AND (x.document_swept_at IS NULL
           OR x.document_swept_at <= sqlc.arg(now)::timestamptz
               - least(interval '15 minutes' * power(2, least(x.document_sweeps - 1, 7)), interval '1 day'))
    ORDER BY x.document_swept_at NULLS FIRST, x.created_at, x.id
    LIMIT @max_rows
    FOR UPDATE SKIP LOCKED
)
UPDATE invoices
SET document_sweeps = document_sweeps + 1, document_swept_at = @now::timestamptz
WHERE id IN (SELECT id FROM due)
RETURNING id, workspace_id;

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
SELECT id, workspace_id, subscription_id, order_id, installment_id, total_minor, tax_minor, status
FROM invoices
WHERE provider = @provider AND external_payment_id = @external_payment_id;

-- name: LockInvoiceByExternalPaymentID :one
-- Webhooks only: refunds and disputes on one invoice apply one at a time.
SELECT id, workspace_id, subscription_id, order_id, installment_id, total_minor, tax_minor, status
FROM invoices
WHERE provider = @provider AND external_payment_id = @external_payment_id
FOR UPDATE;

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

-- name: RequestRefund :one
-- A refund Relay asks for, recorded before the provider is called (IssueRefund).
INSERT INTO refunds (invoice_id, workspace_id, provider, amount_minor, reason, status)
VALUES (@invoice_id, @workspace_id, @provider, @amount_minor, sqlc.narg('reason'), 'pending')
RETURNING id;

-- name: GetRefundToIssue :one
SELECT r.id, r.invoice_id, r.provider, r.amount_minor, r.reason, r.status, r.external_refund_id, r.issued_at,
       i.external_payment_id, i.currency
FROM refunds r
JOIN invoices i ON i.id = r.invoice_id AND i.workspace_id = r.workspace_id
WHERE r.workspace_id = @workspace_id AND r.id = @refund_id;

-- name: MarkRefundIssued :execrows
-- The provider accepted the request; its refund ID, when it gave one, unless
-- the refund's webhook already filled it in.
UPDATE refunds
SET issued_at = @issued_at, external_refund_id = COALESCE(external_refund_id, sqlc.narg('external_refund_id'))
WHERE workspace_id = @workspace_id AND id = @refund_id AND issued_at IS NULL;

-- name: FailRequestedRefund :execrows
-- The provider refused the request for good; it is never sent again.
UPDATE refunds
SET status = 'failed'
WHERE workspace_id = @workspace_id AND id = @refund_id AND issued_at IS NULL AND external_refund_id IS NULL;

-- name: ClaimRequestedRefund :execrows
-- A provider refund we have no row for is the oldest refund Relay asked for
-- on the invoice that no webhook has named yet (Creem's refund reply has no ID).
UPDATE refunds
SET external_refund_id = @external_refund_id, status = @status, amount_minor = @amount_minor
WHERE id = (SELECT r.id FROM refunds r
            WHERE r.workspace_id = @workspace_id AND r.invoice_id = @invoice_id AND r.provider = @provider
              AND r.external_refund_id IS NULL AND r.status <> 'failed'
            ORDER BY r.created_at, r.id
            LIMIT 1
            FOR UPDATE);

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

-- name: HasOtherInstallmentPayment :one
-- Whether another invoice still pays for the installment (it was paid again
-- after a refund, or twice at once).
SELECT EXISTS (
    SELECT 1 FROM invoices
    WHERE workspace_id = @workspace_id AND installment_id = @installment_id AND id <> @invoice_id
      AND status IN ('paid', 'partially_refunded', 'disputed')
);

-- name: GetLatestSubscriptionInvoiceID :one
-- The subscription's newest period invoice: refunding it in full ends the
-- subscription. A proration (a mid-period plan change) never does.
SELECT id
FROM invoices
WHERE workspace_id = @workspace_id AND subscription_id = @subscription_id AND NOT proration
ORDER BY created_at DESC, id DESC
LIMIT 1;
