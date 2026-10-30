-- ---------- Connections ----------

-- name: ListConnections :many
SELECT provider, status, account_label, last_checked_at, last_error, created_at, updated_at
FROM integration_connections
WHERE workspace_id = @workspace_id;

-- name: UpsertConnection :exec
INSERT INTO integration_connections
    (id, workspace_id, provider, status, account_label, config, credentials, last_checked_at, last_error, connected_by)
VALUES (@id, @workspace_id, @provider, 'connected', @account_label, @config, @credentials, now(), NULL, @connected_by)
ON CONFLICT (workspace_id, provider) DO UPDATE
SET status = 'connected', account_label = EXCLUDED.account_label, config = EXCLUDED.config,
    credentials = EXCLUDED.credentials, last_checked_at = now(), last_error = NULL,
    connected_by = EXCLUDED.connected_by;

-- name: GetConnectionCredentials :one
SELECT config, credentials FROM integration_connections
WHERE workspace_id = @workspace_id AND provider = @provider AND status <> 'disconnected'
  AND credentials IS NOT NULL;

-- name: RecordConnectionCheck :exec
UPDATE integration_connections
SET status = @status, last_checked_at = now(), last_error = sqlc.narg(last_error)
WHERE workspace_id = @workspace_id AND provider = @provider AND status <> 'disconnected';

-- name: DisconnectConnection :execrows
UPDATE integration_connections
SET status = 'disconnected', credentials = NULL, last_error = NULL
WHERE workspace_id = @workspace_id AND provider = @provider AND status <> 'disconnected';
