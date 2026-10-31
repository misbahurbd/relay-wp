-- name: GetConnectionForWebhook :one
SELECT id, workspace_id, provider
FROM integration_connections
WHERE id = @id AND provider = @provider AND status <> 'disconnected';

-- name: GetConnectionByShop :one
SELECT id, workspace_id
FROM integration_connections
WHERE provider = 'shopify' AND config ->> 'shop' = @shop::text AND status <> 'disconnected';

-- name: GetConnectionID :one
SELECT id FROM integration_connections
WHERE workspace_id = @workspace_id AND provider = @provider;

-- name: InsertCommerceEvent :many
-- A delivery already stored inserts nothing and returns no row.
INSERT INTO commerce_events (connection_id, delivery_id, workspace_id, topic)
VALUES (@connection_id, @delivery_id, @workspace_id, @topic)
ON CONFLICT (connection_id, delivery_id) DO NOTHING
RETURNING delivery_id;

-- name: DeleteCommerceEvents :exec
DELETE FROM commerce_events WHERE connection_id = @connection_id;
