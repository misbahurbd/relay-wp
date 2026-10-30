-- ---------- Webhook endpoints ----------

-- name: ListWebhookEndpoints :many
SELECT id, url, description, events, enabled, consecutive_failures, disabled_reason, created_at, updated_at
FROM webhook_endpoints
WHERE workspace_id = @workspace_id
ORDER BY created_at, id;

-- name: WebhookEndpointStats :many
-- Per endpoint that has finished deliveries: the latest one, and the last 7 days' success count.
SELECT DISTINCT ON (d.endpoint_id) d.endpoint_id, d.status AS last_status,
       d.response_status AS last_response_status, d.created_at AS last_delivery_at,
       (SELECT count(*) FROM webhook_deliveries w
        WHERE w.endpoint_id = d.endpoint_id AND w.status <> 'pending' AND w.created_at > now() - interval '7 days')::bigint AS week_total,
       (SELECT count(*) FROM webhook_deliveries w
        WHERE w.endpoint_id = d.endpoint_id AND w.status = 'succeeded' AND w.created_at > now() - interval '7 days')::bigint AS week_succeeded
FROM webhook_deliveries d
WHERE d.workspace_id = @workspace_id AND d.status <> 'pending'
ORDER BY d.endpoint_id, d.created_at DESC, d.id DESC;

-- name: GetWebhookEndpoint :one
SELECT id, url, description, events, enabled, consecutive_failures, disabled_reason, created_at, updated_at
FROM webhook_endpoints
WHERE workspace_id = @workspace_id AND id = @id;

-- name: CountWebhookEndpoints :one
SELECT count(*) FROM webhook_endpoints WHERE workspace_id = @workspace_id;

-- name: CreateWebhookEndpoint :exec
INSERT INTO webhook_endpoints (id, workspace_id, url, description, events, secret, created_by)
VALUES (@id, @workspace_id, @url, @description, @events, @secret, @created_by);

-- name: UpdateWebhookEndpoint :execrows
-- Turning an endpoint back on clears its failure streak.
UPDATE webhook_endpoints
SET url = @url, description = @description, events = @events, enabled = @enabled,
    consecutive_failures = CASE WHEN @enabled::boolean AND NOT enabled THEN 0 ELSE consecutive_failures END,
    disabled_reason = CASE WHEN @enabled::boolean THEN NULL ELSE disabled_reason END
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteWebhookEndpoint :execrows
DELETE FROM webhook_endpoints WHERE workspace_id = @workspace_id AND id = @id;

-- name: GetWebhookSecret :one
SELECT secret FROM webhook_endpoints WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetWebhookSecret :execrows
UPDATE webhook_endpoints SET secret = @secret WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListSubscribedEndpoints :many
SELECT id FROM webhook_endpoints
WHERE workspace_id = @workspace_id AND enabled AND @event_type::text = ANY (events);

-- name: RecordEndpointSuccess :exec
UPDATE webhook_endpoints SET consecutive_failures = 0
WHERE workspace_id = @workspace_id AND id = @id AND consecutive_failures <> 0;

-- name: RecordEndpointFailure :one
-- Turns the endpoint off once failures reach the limit.
UPDATE webhook_endpoints
SET consecutive_failures = consecutive_failures + 1,
    enabled = CASE WHEN consecutive_failures + 1 >= @max_failures::int THEN false ELSE enabled END,
    disabled_reason = CASE WHEN consecutive_failures + 1 >= @max_failures::int AND enabled
                           THEN @reason::text ELSE disabled_reason END
WHERE workspace_id = @workspace_id AND id = @id
RETURNING enabled;

-- ---------- Webhook deliveries ----------

-- name: CreateWebhookDelivery :exec
INSERT INTO webhook_deliveries (id, workspace_id, endpoint_id, event_id, event_type, payload)
VALUES (@id, @workspace_id, @endpoint_id, @event_id, @event_type, @payload);

-- name: GetDeliveryForSend :one
-- The worker's view: the delivery, its endpoint's URL and secret, and whether it's on.
SELECT d.id, d.workspace_id, d.endpoint_id, d.event_type, d.payload, d.status, e.url, e.secret, e.enabled
FROM webhook_deliveries d
JOIN webhook_endpoints e ON e.id = d.endpoint_id
WHERE d.workspace_id = @workspace_id AND d.id = @id;

-- name: RecordDeliveryAttempt :exec
UPDATE webhook_deliveries
SET attempts = attempts + 1, status = @status, response_status = sqlc.narg(response_status),
    response_ms = sqlc.narg(response_ms), error = sqlc.narg(error),
    delivered_at = CASE WHEN @status = 'succeeded' THEN now() ELSE delivered_at END
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListWebhookDeliveries :many
-- Cursor: (created_at, id) of the last row of the previous page.
SELECT id, event_id, event_type, payload, status, attempts, response_status, response_ms, error, created_at, delivered_at
FROM webhook_deliveries
WHERE workspace_id = @workspace_id AND endpoint_id = @endpoint_id
  AND (sqlc.narg(after_created_at)::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT @page_size;

-- name: ResetDeliveryForRetry :execrows
UPDATE webhook_deliveries SET status = 'pending', error = NULL
WHERE workspace_id = @workspace_id AND endpoint_id = @endpoint_id AND id = @id AND status <> 'pending';

-- name: DeleteOldDeliveries :execrows
DELETE FROM webhook_deliveries WHERE created_at < @before;
