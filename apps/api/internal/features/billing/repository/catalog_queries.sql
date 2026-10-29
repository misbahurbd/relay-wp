-- ---------- Plans ----------

-- name: ListActivePlans :many
SELECT id, code, version, name, trial_days, trial_requires_card, limits, sort_order
FROM plans
WHERE is_active
ORDER BY sort_order, code, version DESC;

-- name: GetPlan :one
SELECT id, code, version, name, trial_days, trial_requires_card, limits, is_active, sort_order
FROM plans
WHERE id = @plan_id;

-- name: CreatePlan :one
INSERT INTO plans (code, version, name, trial_days, trial_requires_card, limits, sort_order)
VALUES (@code, @version, @name, @trial_days, @trial_requires_card, @limits, @sort_order)
RETURNING id;

-- name: DeactivatePlan :execrows
UPDATE plans SET is_active = false WHERE id = @plan_id AND is_active;

-- ---------- Prices (never updated: deactivate and insert a replacement) ----------

-- name: ListActivePlanPrices :many
SELECT pp.id, pp.plan_id, pp.billing_type, pp.interval, pp.currency, pp.amount_minor
FROM plan_prices pp
JOIN plans p ON p.id = pp.plan_id
WHERE pp.is_active AND p.is_active
ORDER BY pp.plan_id, pp.billing_type DESC, pp.interval NULLS LAST, pp.currency;

-- name: GetPlanPrice :one
SELECT pp.id, pp.plan_id, pp.billing_type, pp.interval, pp.currency, pp.amount_minor, pp.is_active,
       p.code AS plan_code, p.name AS plan_name
FROM plan_prices pp
JOIN plans p ON p.id = pp.plan_id
WHERE pp.id = @plan_price_id;

-- name: CreatePlanPrice :one
INSERT INTO plan_prices (plan_id, billing_type, interval, currency, amount_minor)
VALUES (@plan_id, @billing_type, sqlc.narg('interval'), @currency, @amount_minor)
RETURNING id;

-- name: DeactivatePlanPrice :execrows
UPDATE plan_prices SET is_active = false WHERE id = @plan_price_id AND is_active;

-- ---------- Installment offers (one-time prices only) ----------

-- name: ListActiveInstallmentOffers :many
SELECT o.id, o.plan_price_id, o.count, o.interval, o.total_minor, o.grace_days, o.max_missed
FROM installment_offers o
JOIN plan_prices pp ON pp.id = o.plan_price_id
WHERE o.is_active AND pp.is_active
ORDER BY o.plan_price_id, o.count;

-- name: GetInstallmentOffer :one
SELECT id, plan_price_id, count, interval, total_minor, grace_days, max_missed, is_active
FROM installment_offers
WHERE id = @installment_offer_id;

-- name: CreateInstallmentOffer :one
INSERT INTO installment_offers (plan_price_id, count, total_minor, grace_days, max_missed)
VALUES (@plan_price_id, @count, @total_minor, @grace_days, @max_missed)
RETURNING id;

-- name: DeactivateInstallmentOffer :execrows
UPDATE installment_offers SET is_active = false WHERE id = @installment_offer_id AND is_active;

-- ---------- Provider prices ----------

-- name: ListActiveProviderPrices :many
SELECT plan_price_id, installment_offer_id, provider, external_price_id, collection
FROM provider_prices
WHERE is_active
ORDER BY plan_price_id, installment_offer_id NULLS FIRST, provider;

-- name: GetProviderPrice :one
-- installment_offer_id null means the price itself, not one of its installment offers.
SELECT id, plan_price_id, installment_offer_id, provider, external_price_id, collection
FROM provider_prices
WHERE plan_price_id = @plan_price_id
  AND installment_offer_id IS NOT DISTINCT FROM sqlc.narg('installment_offer_id')
  AND provider = @provider
  AND is_active;

-- name: CreateProviderPrice :one
INSERT INTO provider_prices (plan_price_id, installment_offer_id, provider, external_price_id, collection)
VALUES (@plan_price_id, sqlc.narg('installment_offer_id'), @provider, sqlc.narg('external_price_id'),
        sqlc.narg('collection'))
RETURNING id;

-- name: DeactivateProviderPrice :execrows
UPDATE provider_prices SET is_active = false WHERE id = @provider_price_id AND is_active;
