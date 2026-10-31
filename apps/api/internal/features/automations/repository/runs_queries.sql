-- name: InsertRun :many
-- A dedupe key already stored inserts nothing and returns no row.
INSERT INTO automation_runs (id, workspace_id, kind, contact_id, conversation_id, dedupe_key, state, reason, next_at, context)
VALUES (@id, @workspace_id, @kind, @contact_id, sqlc.narg(conversation_id), @dedupe_key, @state, sqlc.narg(reason),
        sqlc.narg(next_at), @context)
ON CONFLICT (workspace_id, dedupe_key) DO NOTHING
RETURNING id;

-- name: ClaimRun :one
-- A due run, locked; another worker holding it means nothing to do here.
SELECT id, kind, contact_id, conversation_id, step, context, created_at
FROM automation_runs
WHERE workspace_id = @workspace_id AND id = @id AND state = 'scheduled' AND next_at <= @now::timestamptz
FOR UPDATE SKIP LOCKED;

-- name: FinishRun :exec
UPDATE automation_runs
SET state = @state, reason = sqlc.narg(reason), next_at = NULL
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DueRuns :many
SELECT workspace_id, id, step
FROM automation_runs
WHERE state = 'scheduled' AND next_at <= @now::timestamptz
ORDER BY next_at
LIMIT @page_size;

-- name: CancelContactRuns :exec
UPDATE automation_runs
SET state = 'cancelled', reason = @reason, next_at = NULL
WHERE workspace_id = @workspace_id AND contact_id = @contact_id AND state = 'scheduled';

-- name: CountRecentReplies :one
SELECT count(id)
FROM automation_runs
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND state = 'done'
  AND created_at >= @since::timestamptz;

-- name: RunStats :many
SELECT r.kind, count(r.id) FILTER (WHERE r.state = 'done') AS sent,
       count(r.id) FILTER (WHERE r.state = 'failed') AS failed,
       count(m.id) FILTER (WHERE m.status = 'read') AS read
FROM automation_runs r
LEFT JOIN messages m ON m.automation_run_id = r.id
WHERE r.workspace_id = @workspace_id AND r.created_at >= @since::timestamptz
GROUP BY r.kind;

-- name: RedactContactRuns :exec
UPDATE automation_runs
SET state = CASE WHEN state = 'scheduled' THEN 'cancelled' ELSE state END,
    reason = CASE WHEN state = 'scheduled' THEN 'redacted' ELSE reason END,
    next_at = NULL, context = '{}'
WHERE workspace_id = @workspace_id AND contact_id = @contact_id;

-- name: RedactStoreRuns :exec
UPDATE automation_runs
SET state = CASE WHEN state = 'scheduled' THEN 'cancelled' ELSE state END,
    reason = CASE WHEN state = 'scheduled' THEN 'redacted' ELSE reason END,
    next_at = NULL, context = '{}'
WHERE workspace_id = @workspace_id AND kind IN ('order', 'cart');
