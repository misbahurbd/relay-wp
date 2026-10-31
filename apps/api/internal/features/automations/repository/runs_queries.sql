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
SELECT kind, count(id) FILTER (WHERE state = 'done') AS sent
FROM automation_runs
WHERE workspace_id = @workspace_id AND created_at >= @since::timestamptz
GROUP BY kind;
