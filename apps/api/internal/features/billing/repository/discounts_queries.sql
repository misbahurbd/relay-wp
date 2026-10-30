-- ---------- Discounts (owned by Relay, not by a workspace) ----------

-- name: GetDiscountByCode :one
SELECT id, code, name, kind, percent_bps, amount_off_minor, currency, duration, duration_months, applies_to, plan_id,
       allow_installments, max_redemptions, per_workspace_limit, redeemed_count, starts_at, expires_at, is_active
FROM discounts
WHERE code = upper(@code::text);

-- name: GetDiscount :one
SELECT id, code, name, kind, percent_bps, amount_off_minor, currency, duration, duration_months, applies_to, plan_id,
       allow_installments, max_redemptions, per_workspace_limit, redeemed_count, starts_at, expires_at, is_active
FROM discounts
WHERE id = @discount_id;

-- name: ListDiscounts :many
SELECT id, code, name, kind, percent_bps, amount_off_minor, currency, duration, duration_months, applies_to, plan_id,
       allow_installments, max_redemptions, per_workspace_limit, redeemed_count, starts_at, expires_at, is_active
FROM discounts
WHERE is_active OR NOT @active_only::bool
ORDER BY created_at DESC
LIMIT 500;

-- name: CreateDiscount :one
INSERT INTO discounts (code, name, kind, percent_bps, amount_off_minor, currency, duration, duration_months,
                       applies_to, plan_id, allow_installments, max_redemptions, per_workspace_limit, starts_at,
                       expires_at, created_by)
VALUES (upper(@code::text), @name, @kind, sqlc.narg('percent_bps'), sqlc.narg('amount_off_minor'),
        sqlc.narg('currency'), @duration, sqlc.narg('duration_months'), @applies_to, sqlc.narg('plan_id'),
        @allow_installments, sqlc.narg('max_redemptions'), @per_workspace_limit, @starts_at,
        sqlc.narg('expires_at'), sqlc.narg('created_by'))
RETURNING id;

-- name: DeactivateDiscount :execrows
UPDATE discounts SET is_active = false WHERE id = @discount_id AND is_active;

-- ---------- Redemptions ----------

-- name: CountWorkspaceRedemptions :one
-- Uses that still count against per_workspace_limit (reserved or redeemed).
SELECT count(*)::int
FROM discount_redemptions
WHERE discount_id = @discount_id AND workspace_id = @workspace_id AND status <> 'released';

-- name: ReserveDiscount :one
-- One statement: the conditional bump of redeemed_count takes the discount's
-- row lock, so max_redemptions holds however many checkouts race for it.
-- No row back means the code is used up, inactive, outside its dates, or at
-- the workspace's limit.
WITH bumped AS (
    UPDATE discounts d
    SET redeemed_count = d.redeemed_count + 1
    WHERE d.id = @discount_id
      AND d.is_active
      AND d.starts_at <= @now
      AND (d.expires_at IS NULL OR d.expires_at > @now)
      AND (d.max_redemptions IS NULL OR d.redeemed_count < d.max_redemptions)
      AND (SELECT count(*) FROM discount_redemptions r
           WHERE r.discount_id = d.id AND r.workspace_id = @workspace_id AND r.status <> 'released')
          < d.per_workspace_limit
    RETURNING d.id
)
INSERT INTO discount_redemptions (discount_id, workspace_id, checkout_session_id, amount_minor)
SELECT bumped.id, @workspace_id, @checkout_session_id, @amount_minor
FROM bumped
RETURNING id;

-- name: RedeemDiscount :execrows
-- Payment went through: the reservation becomes a redemption.
UPDATE discount_redemptions
SET status = 'redeemed', redeemed_at = @now,
    subscription_id = sqlc.narg('subscription_id'), order_id = sqlc.narg('order_id')
WHERE workspace_id = @workspace_id AND checkout_session_id = @checkout_session_id AND status = 'reserved';

-- name: ReleaseDiscount :execrows
-- The checkout failed or expired: free the reservation and give the use back,
-- in one statement so redeemed_count never drifts.
WITH released AS (
    UPDATE discount_redemptions
    SET status = 'released'
    WHERE workspace_id = @workspace_id AND checkout_session_id = @checkout_session_id AND status = 'reserved'
    RETURNING discount_id
)
UPDATE discounts d
SET redeemed_count = d.redeemed_count - 1
FROM released
WHERE d.id = released.discount_id;

-- name: ReclaimDiscount :execrows
-- A released reservation's session was paid after all: hold the use again,
-- unless the code has been used up since.
WITH bumped AS (
    UPDATE discounts d
    SET redeemed_count = d.redeemed_count + 1
    FROM discount_redemptions r
    WHERE r.workspace_id = @workspace_id AND r.checkout_session_id = @checkout_session_id
      AND r.status = 'released' AND d.id = r.discount_id
      AND (d.max_redemptions IS NULL OR d.redeemed_count < d.max_redemptions)
    RETURNING d.id
)
UPDATE discount_redemptions dr
SET status = 'reserved'
WHERE dr.workspace_id = @workspace_id AND dr.checkout_session_id = @checkout_session_id AND dr.status = 'released'
  AND EXISTS (SELECT 1 FROM bumped);

-- ---------- Provider coupons (subscriptions the provider renews) ----------

-- name: GetProviderDiscount :one
SELECT id, discount_id, provider, external_coupon_id, external_promotion_code_id
FROM provider_discounts
WHERE discount_id = @discount_id AND provider = @provider AND is_active;

-- name: CreateProviderDiscount :one
INSERT INTO provider_discounts (discount_id, provider, external_coupon_id, external_promotion_code_id)
VALUES (@discount_id, @provider, @external_coupon_id, sqlc.narg('external_promotion_code_id'))
RETURNING id, discount_id, provider, external_coupon_id, external_promotion_code_id;

-- name: DeactivateProviderDiscounts :execrows
UPDATE provider_discounts SET is_active = false WHERE discount_id = @discount_id AND is_active;
