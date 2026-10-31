-- name: InsertRun :many
-- A dedupe key already stored inserts nothing and returns no row.
INSERT INTO automation_runs (id, workspace_id, kind, contact_id, conversation_id, dedupe_key, state, reason, next_at, context)
VALUES (@id, @workspace_id, @kind, @contact_id, sqlc.narg(conversation_id), @dedupe_key, @state, sqlc.narg(reason),
        sqlc.narg(next_at), @context)
ON CONFLICT (workspace_id, dedupe_key) DO NOTHING
RETURNING id;

-- name: UpsertCartRun :exec
-- One run per checkout. Each change restarts the wait, until a reminder has gone.
INSERT INTO automation_runs (id, workspace_id, kind, contact_id, dedupe_key, state, next_at, context)
VALUES (@id, @workspace_id, 'cart', @contact_id, @dedupe_key, 'scheduled', @next_at, @context)
ON CONFLICT (workspace_id, dedupe_key) DO UPDATE
SET next_at = EXCLUDED.next_at, context = EXCLUDED.context, contact_id = EXCLUDED.contact_id
WHERE automation_runs.state = 'scheduled' AND automation_runs.step = 0;

-- name: CancelCartRun :exec
-- The order came before any reminder went.
UPDATE automation_runs
SET state = 'cancelled', reason = 'order_placed', next_at = NULL
WHERE workspace_id = @workspace_id AND dedupe_key = @dedupe_key AND state = 'scheduled' AND step = 0;

-- name: RecoverCart :exec
-- The order came after a reminder: count it, once, and send no more.
UPDATE automation_runs
SET recovered_minor = @amount_minor, recovered_currency = @currency, state = 'done', next_at = NULL
WHERE workspace_id = @workspace_id AND dedupe_key = @dedupe_key AND recovered_minor IS NULL
  AND (state = 'done' OR (state = 'scheduled' AND step > 0));

-- name: EnrolDrip :exec
INSERT INTO automation_runs (id, workspace_id, kind, contact_id, dedupe_key, state, next_at)
SELECT u.id, @workspace_id, 'drip', u.contact_id, u.dedupe_key, 'scheduled', @next_at
FROM (SELECT unnest(@ids::uuid[]) AS id, unnest(@contact_ids::uuid[]) AS contact_id,
             unnest(@dedupe_keys::text[]) AS dedupe_key) AS u
ON CONFLICT (workspace_id, dedupe_key) DO NOTHING;

-- name: CancelDripRuns :exec
UPDATE automation_runs
SET state = 'cancelled', reason = 'replied', next_at = NULL
WHERE workspace_id = @workspace_id AND contact_id = @contact_id AND kind = 'drip' AND state = 'scheduled';

-- name: ShiftDripRuns :exec
-- Resuming moves every date on by the time paused, so nobody gets a burst.
UPDATE automation_runs
SET next_at = next_at + make_interval(secs => @paused_seconds::float8)
WHERE workspace_id = @workspace_id AND kind = 'drip' AND state = 'scheduled';

-- name: ActiveRuns :many
SELECT kind, count(id) AS active
FROM automation_runs
WHERE workspace_id = @workspace_id AND state = 'scheduled'
GROUP BY kind;

-- name: AdvanceRun :exec
UPDATE automation_runs
SET step = step + 1, next_at = @next_at
WHERE workspace_id = @workspace_id AND id = @id;

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
-- A paused drip holds its runs where they are.
SELECT r.workspace_id, r.id, r.step
FROM automation_runs r
WHERE r.state = 'scheduled' AND r.next_at <= @now::timestamptz
  AND NOT EXISTS (SELECT 1 FROM automations a
                  WHERE a.workspace_id = r.workspace_id AND a.kind = r.kind AND a.kind = 'drip' AND NOT a.enabled)
ORDER BY r.next_at
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
SELECT r.kind, count(m.id) AS sent,
       count(DISTINCT r.id) FILTER (WHERE r.state = 'failed') AS failed,
       count(m.id) FILTER (WHERE m.status = 'read') AS read,
       count(DISTINCT r.id) FILTER (WHERE r.recovered_minor IS NOT NULL) AS recovered,
       (SELECT COALESCE(sum(x.recovered_minor), 0) FROM automation_runs x
        WHERE x.workspace_id = @workspace_id AND x.kind = r.kind AND x.created_at >= @since::timestamptz)::bigint AS recovered_minor,
       COALESCE(min(r.recovered_currency), '')::text AS recovered_currency
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
