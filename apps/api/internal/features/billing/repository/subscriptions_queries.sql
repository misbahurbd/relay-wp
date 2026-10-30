-- ---------- Subscriptions ----------

-- name: CreateSubscription :one
INSERT INTO subscriptions (workspace_id, plan_price_id, billing_customer_id, provider, external_subscription_id,
                           renewal_mode, status, provider_status, current_period_start, current_period_end,
                           discount_id, discount_ends_at)
VALUES (@workspace_id, @plan_price_id, @billing_customer_id, @provider, sqlc.narg('external_subscription_id'),
        @renewal_mode, @status, sqlc.narg('provider_status'), sqlc.narg('current_period_start'),
        sqlc.narg('current_period_end'), sqlc.narg('discount_id'), sqlc.narg('discount_ends_at'))
RETURNING id;

-- name: GetSubscription :one
SELECT id, workspace_id, plan_price_id, billing_customer_id, provider, external_subscription_id, renewal_mode, status,
       provider_status, current_period_start, current_period_end, grace_ends_at, cancel_at_period_end, canceled_at,
       pending_plan_price_id, pending_change_at, discount_id, discount_ends_at, created_at
FROM subscriptions
WHERE workspace_id = @workspace_id AND id = @subscription_id;

-- name: LockSubscription :one
SELECT id, workspace_id, plan_price_id, billing_customer_id, provider, external_subscription_id, renewal_mode, status,
       provider_status, current_period_start, current_period_end, grace_ends_at, cancel_at_period_end, canceled_at,
       pending_plan_price_id, pending_change_at, discount_id, discount_ends_at, created_at
FROM subscriptions
WHERE workspace_id = @workspace_id AND id = @subscription_id
FOR UPDATE;

-- name: GetLiveSubscription :one
-- The workspace's one trialing, active or past_due subscription.
SELECT id, workspace_id, plan_price_id, billing_customer_id, provider, external_subscription_id, renewal_mode, status,
       provider_status, current_period_start, current_period_end, grace_ends_at, cancel_at_period_end, canceled_at,
       pending_plan_price_id, pending_change_at, discount_id, discount_ends_at, created_at
FROM subscriptions
WHERE workspace_id = @workspace_id AND status IN ('trialing', 'active', 'past_due');

-- name: GetLatestSubscription :one
-- The workspace's live subscription if it has one, else its most recent.
SELECT id, workspace_id, plan_price_id, billing_customer_id, provider, external_subscription_id, renewal_mode, status,
       provider_status, current_period_start, current_period_end, grace_ends_at, cancel_at_period_end, canceled_at,
       pending_plan_price_id, pending_change_at, discount_id, discount_ends_at, created_at
FROM subscriptions
WHERE workspace_id = @workspace_id
ORDER BY (status IN ('trialing', 'active', 'past_due')) DESC, created_at DESC, id DESC
LIMIT 1;

-- name: FindSubscriptionByExternalID :one
-- Webhooks only: renewals carry the provider's subscription ID, which we stored.
SELECT id, workspace_id
FROM subscriptions
WHERE provider = @provider AND external_subscription_id = @external_subscription_id;

-- name: SetSubscriptionExternalID :execrows
UPDATE subscriptions
SET external_subscription_id = @external_subscription_id
WHERE workspace_id = @workspace_id AND id = @subscription_id AND external_subscription_id IS NULL;

-- name: SyncSubscription :execrows
-- Copies the provider's view of a subscription.
UPDATE subscriptions
SET status               = @status,
    provider_status      = sqlc.narg('provider_status'),
    current_period_start = sqlc.narg('current_period_start'),
    current_period_end   = sqlc.narg('current_period_end'),
    grace_ends_at        = sqlc.narg('grace_ends_at'),
    cancel_at_period_end = @cancel_at_period_end,
    canceled_at          = sqlc.narg('canceled_at')
WHERE workspace_id = @workspace_id AND id = @subscription_id;

-- name: SetSubscriptionPrice :execrows
-- An upgrade applied now, or a pending downgrade reaching its date.
UPDATE subscriptions
SET plan_price_id = @plan_price_id, pending_plan_price_id = NULL, pending_change_at = NULL
WHERE workspace_id = @workspace_id AND id = @subscription_id;

-- name: SetSubscriptionPendingChange :execrows
-- Both null clears a scheduled downgrade.
UPDATE subscriptions
SET pending_plan_price_id = sqlc.narg('pending_plan_price_id'), pending_change_at = sqlc.narg('pending_change_at')
WHERE workspace_id = @workspace_id AND id = @subscription_id;

-- name: SetSubscriptionCancelAtPeriodEnd :execrows
UPDATE subscriptions
SET cancel_at_period_end = @cancel_at_period_end
WHERE workspace_id = @workspace_id AND id = @subscription_id AND status IN ('trialing', 'active', 'past_due');

-- name: CancelSubscriptionNow :execrows
UPDATE subscriptions
SET status = 'canceled', canceled_at = @canceled_at, cancel_at_period_end = false,
    pending_plan_price_id = NULL, pending_change_at = NULL
WHERE workspace_id = @workspace_id AND id = @subscription_id AND status <> 'canceled';

-- name: SetSubscriptionDiscount :execrows
UPDATE subscriptions
SET discount_id = sqlc.narg('discount_id'), discount_ends_at = sqlc.narg('discount_ends_at')
WHERE workspace_id = @workspace_id AND id = @subscription_id;

-- name: ListDuePendingChanges :many
-- Job only (all workspaces): scheduled downgrades whose date has come.
SELECT id, workspace_id
FROM subscriptions
WHERE pending_change_at <= @now AND status IN ('trialing', 'active', 'past_due')
ORDER BY pending_change_at
LIMIT @max_rows;

-- name: ListAppRenewalsDue :many
-- Job only (all workspaces): subscriptions Relay renews itself (SSLCommerz).
SELECT id, workspace_id
FROM subscriptions
WHERE renewal_mode = 'app' AND status IN ('active', 'past_due') AND current_period_end <= @now
ORDER BY current_period_end
LIMIT @max_rows;

-- name: ListPastDueGraceEnded :many
-- Job only (all workspaces): past-due subscriptions whose grace has run out
-- and whose plan is still active.
SELECT s.id, s.workspace_id
FROM subscriptions s
WHERE s.status = 'past_due' AND s.grace_ends_at <= @now
  AND EXISTS (SELECT 1 FROM plan_grants g WHERE g.subscription_id = s.id AND g.status = 'active')
ORDER BY s.grace_ends_at
LIMIT @max_rows;

-- ---------- Change history (append-only) ----------

-- name: CreateSubscriptionChange :exec
INSERT INTO subscription_changes (subscription_id, workspace_id, from_price_id, to_price_id, kind, effective_at,
                                  proration_minor)
VALUES (@subscription_id, @workspace_id, @from_price_id, @to_price_id, @kind, @effective_at, @proration_minor);

-- name: ListSubscriptionChanges :many
SELECT id, from_price_id, to_price_id, kind, effective_at, proration_minor, created_at
FROM subscription_changes
WHERE workspace_id = @workspace_id AND subscription_id = @subscription_id
ORDER BY effective_at DESC
LIMIT 100;

-- name: CountSubscriptionChanges :one
-- How many changes the subscription has had: part of a plan change's
-- idempotency key, so a retried change reuses its key and a new one doesn't.
SELECT count(*)::int
FROM subscription_changes
WHERE workspace_id = @workspace_id AND subscription_id = @subscription_id;

-- name: HasHadTrial :one
-- Whether the workspace has used its one trial: it had a subscription that
-- started (trialing or paid), or a card-less trial grant.
SELECT EXISTS (
    SELECT 1 FROM subscriptions s WHERE s.workspace_id = @workspace_id AND s.status <> 'incomplete'
    UNION ALL
    SELECT 1 FROM plan_grants g WHERE g.workspace_id = @workspace_id AND g.source = 'trial'
);
