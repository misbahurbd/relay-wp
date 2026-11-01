-- ---------- Broadcasts ----------

-- name: InsertBroadcast :exec
INSERT INTO broadcasts (id, workspace_id, name, number_id, template_id, audience, variables, created_by)
VALUES (@id, @workspace_id, @name, sqlc.narg('number_id'), sqlc.narg('template_id'), @audience, @variables,
        sqlc.narg('created_by'));

-- name: GetBroadcast :one
SELECT b.id, b.name, b.number_id, b.template_id, t.name AS template_name, t.language AS template_language,
       t.category AS template_category, b.audience, b.variables, b.status, b.scheduled_at, b.started_at,
       b.completed_at, b.recipients_total, b.failure_reason, b.created_at, b.updated_at
FROM broadcasts b
LEFT JOIN message_templates t ON t.workspace_id = b.workspace_id AND t.id = b.template_id
WHERE b.workspace_id = @workspace_id AND b.id = @id;

-- name: ListBroadcasts :many
-- Newest first, with each broadcast's delivery counts.
SELECT b.id, b.name, b.template_id, t.name AS template_name, b.status, b.scheduled_at, b.started_at,
       b.completed_at, b.recipients_total, b.created_at,
       s.sent::int AS sent, s.delivered::int AS delivered, s.read::int AS read, s.failed::int AS failed
FROM broadcasts b
LEFT JOIN message_templates t ON t.workspace_id = b.workspace_id AND t.id = b.template_id
LEFT JOIN LATERAL (
    SELECT count(m.id) FILTER (WHERE m.status IN ('sent', 'delivered', 'read')) AS sent,
           count(m.id) FILTER (WHERE m.status IN ('delivered', 'read'))         AS delivered,
           count(m.id) FILTER (WHERE m.status = 'read')                         AS read,
           count(r.contact_id) FILTER (WHERE r.state = 'failed' OR m.status = 'failed') AS failed
    FROM broadcast_recipients r
    LEFT JOIN messages m ON m.id = r.message_id
    WHERE r.broadcast_id = b.id
) s ON true
WHERE b.workspace_id = @workspace_id
  AND (sqlc.narg('status')::text IS NULL OR b.status = sqlc.narg('status'))
  AND (sqlc.narg('query')::text IS NULL OR b.name ILIKE '%' || sqlc.narg('query') || '%')
  AND (sqlc.narg('before_id')::uuid IS NULL OR b.id < sqlc.narg('before_id'))
ORDER BY b.id DESC
LIMIT @lim;

-- name: LockBroadcast :one
SELECT id, name, status, number_id, template_id, audience, variables, scheduled_at, snapshot_at, usage_period
FROM broadcasts
WHERE workspace_id = @workspace_id AND id = @id
FOR UPDATE;

-- name: UpdateBroadcast :exec
UPDATE broadcasts
SET name = @name, number_id = sqlc.narg('number_id'), template_id = sqlc.narg('template_id'),
    audience = @audience, variables = @variables, updated_at = now()
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteDraft :execrows
DELETE FROM broadcasts WHERE workspace_id = @workspace_id AND id = @id AND status = 'draft';

-- name: ScheduleBroadcast :exec
UPDATE broadcasts
SET status = 'scheduled', scheduled_at = @scheduled_at::timestamptz, usage_period = @usage_period, updated_at = now()
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SendBroadcastNow :exec
UPDATE broadcasts
SET status = 'sending', scheduled_at = NULL, started_at = @now::timestamptz, usage_period = @usage_period, updated_at = now()
WHERE workspace_id = @workspace_id AND id = @id;

-- name: Reschedule :exec
UPDATE broadcasts SET scheduled_at = @scheduled_at::timestamptz, updated_at = now()
WHERE workspace_id = @workspace_id AND id = @id AND status = 'scheduled';

-- name: StartBroadcast :one
-- Moves a due broadcast to sending, once: no row once its audience is taken or it was cancelled.
UPDATE broadcasts
SET status = 'sending', started_at = COALESCE(started_at, @now::timestamptz), updated_at = now()
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('scheduled', 'sending') AND snapshot_at IS NULL
  AND (status = 'sending' OR scheduled_at <= @now::timestamptz)
RETURNING audience;

-- name: SetSnapshot :exec
UPDATE broadcasts SET recipients_total = @recipients_total, snapshot_at = @now::timestamptz, updated_at = now()
WHERE workspace_id = @workspace_id AND id = @id;

-- name: BroadcastStatus :one
SELECT status, number_id, template_id, variables FROM broadcasts WHERE workspace_id = @workspace_id AND id = @id;

-- name: CancelBroadcast :one
UPDATE broadcasts SET status = 'cancelled', completed_at = @now::timestamptz, updated_at = now()
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('scheduled', 'sending')
RETURNING usage_period;

-- name: CompleteBroadcast :one
UPDATE broadcasts SET status = 'completed', completed_at = @now::timestamptz, updated_at = now()
WHERE workspace_id = @workspace_id AND id = @id AND status = 'sending'
RETURNING name, recipients_total;

-- name: FailBroadcast :execrows
UPDATE broadcasts SET status = 'failed', failure_reason = @reason, completed_at = @now::timestamptz, updated_at = now()
WHERE workspace_id = @workspace_id AND id = @id AND status = 'sending';

-- ---------- The tick (every workspace) ----------

-- name: DueToStart :many
-- Scheduled broadcasts whose time has come, and sends whose start job hasn't
-- taken the audience a minute on (a reschedule or "send now" left it waiting).
SELECT workspace_id, id FROM broadcasts
WHERE (status = 'scheduled' AND scheduled_at <= @now::timestamptz)
   OR (status = 'sending' AND snapshot_at IS NULL AND started_at <= @stale::timestamptz)
ORDER BY id
LIMIT 500;

-- name: FinishedSending :many
-- Sending broadcasts with every recipient handled and no message still waiting to go.
SELECT b.workspace_id, b.id FROM broadcasts b
WHERE b.status = 'sending' AND b.snapshot_at IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM broadcast_recipients r WHERE r.broadcast_id = b.id AND r.state = 'pending')
  AND NOT EXISTS (SELECT 1 FROM broadcast_recipients r JOIN messages m ON m.id = r.message_id
                  WHERE r.broadcast_id = b.id AND m.status = 'pending')
LIMIT 100;

-- name: StalledSending :many
SELECT workspace_id, id FROM broadcasts
WHERE status = 'sending' AND started_at < @before::timestamptz
LIMIT 100;
