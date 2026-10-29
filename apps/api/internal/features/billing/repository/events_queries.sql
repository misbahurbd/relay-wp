-- ---------- Webhook inbox ----------
-- payload is written once here and nulled by PurgeEventPayloads. No query
-- selects it: it is never logged or returned.

-- name: InsertPaymentEvent :one
-- Dedupes on (provider, external_event_id): a replay gets no row back.
INSERT INTO payment_events (provider, external_event_id, event_type, workspace_id, checkout_session_id, payload)
VALUES (@provider, @external_event_id, @event_type, sqlc.narg('workspace_id'), sqlc.narg('checkout_session_id'),
        @payload)
ON CONFLICT (provider, external_event_id) DO NOTHING
RETURNING id;

-- name: GetPaymentEvent :one
SELECT id, provider, external_event_id, event_type, workspace_id, checkout_session_id, received_at, processed_at
FROM payment_events
WHERE id = @payment_event_id;

-- name: SetPaymentEventWorkspace :execrows
-- The workspace an event belongs to, once processing has found it.
UPDATE payment_events
SET workspace_id = @workspace_id, checkout_session_id = COALESCE(sqlc.narg('checkout_session_id'), checkout_session_id)
WHERE id = @payment_event_id AND workspace_id IS NULL;

-- name: MarkPaymentEventProcessed :execrows
-- No row changed means another worker already processed it.
UPDATE payment_events
SET processed_at = @now
WHERE id = @payment_event_id AND processed_at IS NULL;

-- name: ListWorkspacePaymentEvents :many
-- The support view of a workspace's webhook events, newest first.
SELECT id, provider, external_event_id, event_type, checkout_session_id, received_at, processed_at
FROM payment_events
WHERE workspace_id = @workspace_id
ORDER BY received_at DESC
LIMIT @max_rows;

-- name: PurgeEventPayloads :execrows
-- Nulls payloads older than the cutoff, a batch at a time.
UPDATE payment_events
SET payload = NULL
WHERE id IN (SELECT pe.id FROM payment_events pe
             WHERE pe.payload IS NOT NULL AND pe.received_at < @before
             ORDER BY pe.received_at
             LIMIT @max_rows);
