-- ---------- Plan grants: what a workspace can use ----------

-- name: GetCurrentGrant :one
-- The workspace's one active or suspended grant. No row means the free plan.
-- Limits are the plan's as they are now, plus the grant's overrides (ADR 0008).
SELECT g.id, g.plan_id, p.code AS plan_code, p.name AS plan_name, g.source, g.subscription_id, g.order_id, g.status,
       p.limits AS plan_limits, g.overrides, g.starts_at, g.ends_at, g.suspended_at, g.suspend_reason
FROM plan_grants g
JOIN plans p ON p.id = g.plan_id
WHERE g.workspace_id = @workspace_id AND g.status IN ('active', 'suspended');

-- name: LockCurrentGrant :one
SELECT id, plan_id, source, subscription_id, order_id, status, overrides, starts_at, ends_at, suspended_at, suspend_reason
FROM plan_grants
WHERE workspace_id = @workspace_id AND status IN ('active', 'suspended')
FOR UPDATE;

-- name: GetOrderGrant :one
SELECT id, plan_id, source, subscription_id, order_id, status, starts_at, ends_at, suspended_at, suspend_reason
FROM plan_grants
WHERE workspace_id = @workspace_id AND order_id = @order_id;

-- name: CreateGrant :one
-- The caller ends the current grant first: one current grant per workspace.
INSERT INTO plan_grants (workspace_id, plan_id, source, subscription_id, order_id, status, starts_at, ends_at,
                         created_by, reason)
VALUES (@workspace_id, @plan_id, @source, sqlc.narg('subscription_id'), sqlc.narg('order_id'), 'active',
        @starts_at, sqlc.narg('ends_at'), sqlc.narg('created_by'), sqlc.narg('reason'))
RETURNING id;

-- name: GetFreePlanLimits :one
-- What a workspace with no active grant may use: the free plan row's limits.
SELECT limits
FROM plans
WHERE code = 'free' AND is_active
ORDER BY version DESC
LIMIT 1;

-- name: SetGrantOverrides :execrows
UPDATE plan_grants SET overrides = @overrides, updated_at = now() WHERE id = @grant_id;

-- name: SuspendGrant :execrows
UPDATE plan_grants
SET status = 'suspended', suspended_at = @now, suspend_reason = @reason
WHERE workspace_id = @workspace_id AND id = @grant_id AND status = 'active';

-- name: RestoreGrant :execrows
UPDATE plan_grants
SET status = 'active', suspended_at = NULL, suspend_reason = NULL
WHERE workspace_id = @workspace_id AND id = @grant_id AND status = 'suspended';

-- name: EndGrant :execrows
UPDATE plan_grants
SET status = 'ended', suspended_at = NULL, suspend_reason = NULL, ends_at = COALESCE(LEAST(ends_at, @now), @now)
WHERE workspace_id = @workspace_id AND id = @grant_id AND status IN ('active', 'suspended');

-- name: SetWorkspacePlanCache :execrows
-- workspaces.plan and trial_ends_at are a cache of the current grant (Q8):
-- written only in the same transaction as the grant change.
UPDATE workspaces
SET plan = @plan_code, trial_ends_at = sqlc.narg('trial_ends_at')
WHERE id = @workspace_id;

-- name: ListExpiredGrants :many
-- Job only (all workspaces): card-less trials and staff grants that have reached their end.
SELECT id, workspace_id
FROM plan_grants
WHERE source IN ('trial', 'manual') AND status IN ('active', 'suspended') AND ends_at IS NOT NULL AND ends_at <= @now
ORDER BY ends_at
LIMIT @max_rows;

-- name: ListPlanCacheDrift :many
-- Job only (all workspaces): workspaces whose cached plan disagrees with their grant.
SELECT w.id AS workspace_id, w.plan AS cached_plan, COALESCE(p.code, @free_plan::text)::text AS granted_plan
FROM workspaces w
LEFT JOIN plan_grants g ON g.workspace_id = w.id AND g.status = 'active'
LEFT JOIN plans p ON p.id = g.plan_id
WHERE w.deleted_at IS NULL AND w.plan IS DISTINCT FROM COALESCE(p.code, @free_plan::text)
LIMIT @max_rows;
