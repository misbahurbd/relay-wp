-- ---------- Installments ----------

-- name: CreateInstallments :exec
-- Every installment of an order, created together at checkout.
INSERT INTO installments (order_id, workspace_id, seq, due_at, amount_minor, status)
SELECT @order_id, @workspace_id, unnest(@seqs::int[]), unnest(@due_ats::timestamptz[]),
       unnest(@amounts_minor::bigint[]), 'scheduled';

-- name: ListOrderInstallments :many
-- refunded: a payment of the installment was refunded, so a provider
-- schedule won't charge it again.
SELECT i.id, i.order_id, i.seq, i.due_at, i.amount_minor, i.status, i.paid_at, i.reminder_sent_at,
       EXISTS (SELECT 1 FROM invoices v
               WHERE v.workspace_id = i.workspace_id AND v.installment_id = i.id AND v.status = 'refunded') AS refunded
FROM installments i
WHERE i.workspace_id = @workspace_id AND i.order_id = @order_id
ORDER BY i.seq;

-- name: LockInstallment :one
SELECT i.id, i.order_id, i.seq, i.due_at, i.amount_minor, i.status, i.paid_at, i.reminder_sent_at,
       EXISTS (SELECT 1 FROM invoices v
               WHERE v.workspace_id = i.workspace_id AND v.installment_id = i.id AND v.status = 'refunded') AS refunded
FROM installments i
WHERE i.workspace_id = @workspace_id AND i.id = @installment_id
FOR UPDATE OF i;

-- name: GetInstallmentBySeq :one
SELECT id, order_id, seq, due_at, amount_minor, status, paid_at, reminder_sent_at
FROM installments
WHERE workspace_id = @workspace_id AND order_id = @order_id AND seq = @seq;

-- name: MarkInstallmentPaid :execrows
UPDATE installments
SET status = 'paid', paid_at = @paid_at
WHERE workspace_id = @workspace_id AND id = @installment_id AND status <> 'paid';

-- name: ReopenInstallment :execrows
-- A paid installment whose payment was refunded in full is owed again from
-- @now: the hourly pass marks it due, reminds, and grace starts over.
UPDATE installments
SET status = 'scheduled', due_at = GREATEST(due_at, @now::timestamptz), paid_at = NULL, reminder_sent_at = NULL
WHERE workspace_id = @workspace_id AND id = @installment_id AND status = 'paid';

-- name: ReopenCanceledInstallments :execrows
-- The installments a refunded balance payment settled are owed again from
-- @now, as ReopenInstallment.
UPDATE installments
SET status = 'scheduled', due_at = GREATEST(due_at, @now::timestamptz), reminder_sent_at = NULL
WHERE workspace_id = @workspace_id AND order_id = @order_id AND status = 'canceled';

-- name: SetInstallmentStatus :execrows
-- Any status but paid (MarkInstallmentPaid sets paid_at with it).
UPDATE installments
SET status = @status
WHERE workspace_id = @workspace_id AND id = @installment_id AND status <> 'paid' AND @status <> 'paid';

-- name: CancelOpenInstallments :execrows
-- An upgraded or refunded order: what is still open will never be collected.
UPDATE installments
SET status = 'canceled'
WHERE workspace_id = @workspace_id AND order_id = @order_id AND status IN ('scheduled', 'due', 'overdue', 'failed');

-- name: CountMissedInstallments :one
SELECT count(*)::int
FROM installments
WHERE workspace_id = @workspace_id AND order_id = @order_id AND status IN ('overdue', 'failed');

-- name: SetInstallmentReminderSent :execrows
UPDATE installments
SET reminder_sent_at = @now
WHERE workspace_id = @workspace_id AND id = @installment_id;

-- name: ListInstallmentsComingDue :many
-- Job only (all workspaces): scheduled installments whose due date has come,
-- in (due_at, id) order after the last row read. refunded as in
-- ListOrderInstallments.
SELECT i.id, i.workspace_id, i.order_id, i.seq, i.due_at, i.amount_minor,
       EXISTS (SELECT 1 FROM invoices v
               WHERE v.workspace_id = i.workspace_id AND v.installment_id = i.id AND v.status = 'refunded') AS refunded
FROM installments i
WHERE i.status = 'scheduled' AND i.due_at <= @now AND (i.due_at, i.id) > (@after_due_at::timestamptz, @after_id::uuid)
ORDER BY i.due_at, i.id
LIMIT @max_rows;

-- name: ListInstallmentsPastGrace :many
-- Job only (all workspaces): due installments unpaid past their offer's
-- grace_days, in (due_at, id) order after the last row read.
SELECT i.id, i.workspace_id, i.order_id, i.seq, i.due_at, o.max_missed
FROM installments i
JOIN orders ord ON ord.id = i.order_id
JOIN installment_offers o ON o.id = ord.installment_offer_id
WHERE i.status = 'due' AND i.due_at + make_interval(days => o.grace_days) <= @now
  AND (i.due_at, i.id) > (@after_due_at::timestamptz, @after_id::uuid)
ORDER BY i.due_at, i.id
LIMIT @max_rows;
