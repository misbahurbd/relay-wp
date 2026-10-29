-- ---------- Checkout sessions ----------

-- name: CreateCheckoutSession :one
-- ON CONFLICT: a retried request with the same Idempotency-Key gets no row;
-- the caller then reads the first one with GetCheckoutSessionByIdempotencyKey.
INSERT INTO checkout_sessions (workspace_id, plan_price_id, installment_offer_id, purpose, subscription_id, order_id,
                               installment_id, provider, amount_minor, currency, idempotency_key, discount_id,
                               discount_minor, created_by, expires_at)
VALUES (@workspace_id, @plan_price_id, sqlc.narg('installment_offer_id'), @purpose, sqlc.narg('subscription_id'),
        sqlc.narg('order_id'), sqlc.narg('installment_id'), @provider, @amount_minor, @currency, @idempotency_key,
        sqlc.narg('discount_id'), @discount_minor, sqlc.narg('created_by'), @expires_at)
ON CONFLICT (workspace_id, idempotency_key) DO NOTHING
RETURNING id, workspace_id, plan_price_id, installment_offer_id, purpose, subscription_id, order_id, installment_id,
          provider, external_session_id, status, amount_minor, currency, discount_id, discount_minor, created_by,
          expires_at, created_at;

-- name: GetCheckoutSession :one
SELECT id, workspace_id, plan_price_id, installment_offer_id, purpose, subscription_id, order_id, installment_id,
       provider, external_session_id, status, amount_minor, currency, discount_id, discount_minor, created_by,
       expires_at, created_at
FROM checkout_sessions
WHERE workspace_id = @workspace_id AND id = @checkout_session_id;

-- name: GetCheckoutSessionByIdempotencyKey :one
SELECT id, workspace_id, plan_price_id, installment_offer_id, purpose, subscription_id, order_id, installment_id,
       provider, external_session_id, status, amount_minor, currency, discount_id, discount_minor, created_by,
       expires_at, created_at
FROM checkout_sessions
WHERE workspace_id = @workspace_id AND idempotency_key = @idempotency_key;

-- name: LockCheckoutSession :one
SELECT id, workspace_id, plan_price_id, installment_offer_id, purpose, subscription_id, order_id, installment_id,
       provider, external_session_id, status, amount_minor, currency, discount_id, discount_minor, created_by,
       expires_at, created_at
FROM checkout_sessions
WHERE workspace_id = @workspace_id AND id = @checkout_session_id
FOR UPDATE;

-- name: FindCheckoutSessionWorkspace :one
-- Webhooks only: our checkout_sessions.id, sent to the provider as its
-- reference, is how an event finds its workspace. Never look one up by email.
SELECT workspace_id
FROM checkout_sessions
WHERE id = @checkout_session_id AND provider = @provider;

-- name: SetCheckoutSessionExternalID :execrows
UPDATE checkout_sessions
SET external_session_id = @external_session_id
WHERE workspace_id = @workspace_id AND id = @checkout_session_id AND external_session_id IS NULL;

-- name: LinkCheckoutSession :execrows
-- Records the subscription or order a pending checkout created.
UPDATE checkout_sessions
SET subscription_id = COALESCE(sqlc.narg('subscription_id'), subscription_id),
    order_id        = COALESCE(sqlc.narg('order_id'), order_id)
WHERE workspace_id = @workspace_id AND id = @checkout_session_id;

-- name: FinishCheckoutSession :execrows
-- Moves a pending session to completed, failed or expired; a finished one is left alone.
UPDATE checkout_sessions
SET status = @status
WHERE workspace_id = @workspace_id AND id = @checkout_session_id AND status = 'pending';

-- name: ReopenCheckoutSession :execrows
-- A payment arrived for a failed or expired session: it completes after all.
UPDATE checkout_sessions
SET status = 'pending'
WHERE workspace_id = @workspace_id AND id = @checkout_session_id AND status IN ('failed', 'expired');

-- name: ExtendCheckoutSession :execrows
-- The provider keeps the checkout payable longer than our default.
UPDATE checkout_sessions
SET expires_at = @expires_at
WHERE workspace_id = @workspace_id AND id = @checkout_session_id AND status = 'pending';

-- name: ListExpiredCheckoutSessions :many
-- Job only (all workspaces): pending sessions past their expiry, oldest first.
SELECT id, workspace_id, discount_id
FROM checkout_sessions
WHERE status = 'pending' AND expires_at <= @now
ORDER BY expires_at
LIMIT @max_rows;
