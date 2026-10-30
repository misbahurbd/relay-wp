-- Staff admin tools only (/v1/admin): the whole catalog, active or not, and
-- every workspace's plan. Never called from a workspace's own requests.

-- name: AdminListPlans :many
SELECT id, code, version, name, description, features, recommended, trial_days, trial_requires_card, limits,
       is_active, sort_order
FROM plans
ORDER BY sort_order, code, version DESC;

-- name: AdminListPlanPrices :many
SELECT id, plan_id, billing_type, interval, currency, amount_minor, is_active
FROM plan_prices
ORDER BY plan_id, is_active DESC, billing_type DESC, interval NULLS LAST, currency, created_at DESC;

-- name: AdminListInstallmentOffers :many
SELECT id, plan_price_id, count, total_minor, grace_days, max_missed, is_active
FROM installment_offers
ORDER BY plan_price_id, is_active DESC, count, created_at DESC;

-- name: AdminListProviderPrices :many
SELECT id, plan_price_id, installment_offer_id, provider, external_price_id, collection, is_active
FROM provider_prices
ORDER BY plan_price_id, installment_offer_id NULLS FIRST, is_active DESC, provider, created_at DESC;

-- name: NextPlanVersion :one
-- A plan is never edited once sold: a new one with a used code is its next version.
SELECT (COALESCE(max(version), 0) + 1)::int AS version
FROM plans
WHERE code = @code;

-- name: AdminListWorkspaces :many
-- Newest first. search matches the workspace name or the creator's email.
SELECT w.id, w.name, w.plan, w.trial_ends_at, w.created_at, COALESCE(u.email, '')::text AS owner_email,
       g.id AS grant_id, g.source AS grant_source, g.status AS grant_status, g.ends_at AS grant_ends_at,
       g.overrides AS grant_overrides, p.code AS grant_plan_code, p.name AS grant_plan_name,
       p.limits AS grant_plan_limits
FROM workspaces w
LEFT JOIN users u ON u.id = w.created_by
LEFT JOIN plan_grants g ON g.workspace_id = w.id AND g.status IN ('active', 'suspended')
LEFT JOIN plans p ON p.id = g.plan_id
WHERE w.deleted_at IS NULL
  AND (@search::text = '' OR w.name ILIKE '%' || @search::text || '%' OR u.email::text ILIKE '%' || @search::text || '%')
ORDER BY w.created_at DESC
LIMIT 200;

-- name: AdminWorkspaceExists :one
SELECT EXISTS (SELECT 1 FROM workspaces WHERE id = @workspace_id AND deleted_at IS NULL);

-- name: AdminCountPlanWorkspaces :many
-- Workspaces whose limits come from each plan right now: an active grant on it.
SELECT plan_id, count(*)::int AS workspaces
FROM plan_grants
WHERE status = 'active'
GROUP BY plan_id;

-- name: AdminCountFreeWorkspaces :one
-- Workspaces on the free plan's limits: no grant, or a suspended one.
SELECT count(*)::int
FROM workspaces w
WHERE w.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM plan_grants g WHERE g.workspace_id = w.id AND g.status = 'active');

-- name: InsertLimitChange :exec
INSERT INTO limit_changes (plan_id, grant_id, changed_by, before, after, reason)
VALUES (sqlc.narg('plan_id'), sqlc.narg('grant_id'), @changed_by, @before, @after, @reason);
