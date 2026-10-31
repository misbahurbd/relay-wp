-- ---------- Usage counters: monthly plan resources (ADR 0008) ----------

-- name: ConsumeUsage :one
-- Adds n to this month's count unless that would pass the cap; a cap below 0 is
-- unlimited. No row back means the cap is reached. The caller refuses an n
-- larger than the cap itself, since a first insert isn't checked.
INSERT INTO usage_counters (workspace_id, resource, period_start, used)
VALUES (@workspace_id, @resource, @period_start, @n::integer)
ON CONFLICT (workspace_id, resource, period_start) DO UPDATE
    SET used = usage_counters.used + EXCLUDED.used, updated_at = now()
    WHERE @cap::integer < 0 OR usage_counters.used + EXCLUDED.used <= @cap::integer
RETURNING used;

-- name: ReleaseUsage :execrows
-- Gives back n of what was consumed in the month that started at period_start.
UPDATE usage_counters
SET used = greatest(used - @n::integer, 0), updated_at = now()
WHERE workspace_id = @workspace_id AND resource = @resource AND period_start = @period_start;

-- name: ListUsage :many
-- Every resource the workspace used in the month that started at period_start.
SELECT resource, used
FROM usage_counters
WHERE workspace_id = @workspace_id AND period_start = @period_start;
