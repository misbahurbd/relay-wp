-- ---------- Installments ----------

-- name: CreateInstallments :exec
-- Every installment of an order, created together at checkout.
INSERT INTO installments (order_id, workspace_id, seq, due_at, amount_minor, status)
SELECT @order_id, @workspace_id, unnest(@seqs::int[]), unnest(@due_ats::timestamptz[]),
       unnest(@amounts_minor::bigint[]), 'scheduled';

-- name: ListOrderInstallments :many
SELECT id, order_id, seq, due_at, amount_minor, status, paid_at, reminder_sent_at
FROM installments
WHERE workspace_id = @workspace_id AND order_id = @order_id
ORDER BY seq;

-- name: LockInstallment :one
SELECT id, order_id, seq, due_at, amount_minor, status, paid_at, reminder_sent_at
FROM installments
WHERE workspace_id = @workspace_id AND id = @installment_id
FOR UPDATE;

-- name: GetInstallmentBySeq :one
SELECT id, order_id, seq, due_at, amount_minor, status, paid_at, reminder_sent_at
FROM installments
WHERE workspace_id = @workspace_id AND order_id = @order_id AND seq = @seq;

-- name: MarkInstallmentPaid :execrows
UPDATE installments
SET status = 'paid', paid_at = @paid_at
WHERE workspace_id = @workspace_id AND id = @installment_id AND status <> 'paid';

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
-- Job only (all workspaces): scheduled installments whose due date has come.
SELECT i.id, i.workspace_id, i.order_id, i.seq, i.due_at, i.amount_minor
FROM installments i
WHERE i.status = 'scheduled' AND i.due_at <= @now
ORDER BY i.due_at
LIMIT @max_rows;

-- name: ListInstallmentsPastGrace :many
-- Job only (all workspaces): due installments unpaid past their offer's grace_days.
SELECT i.id, i.workspace_id, i.order_id, i.seq, i.due_at, o.max_missed
FROM installments i
JOIN orders ord ON ord.id = i.order_id
JOIN installment_offers o ON o.id = ord.installment_offer_id
WHERE i.status = 'due' AND i.due_at + make_interval(days => o.grace_days) <= @now
ORDER BY i.due_at
LIMIT @max_rows;
