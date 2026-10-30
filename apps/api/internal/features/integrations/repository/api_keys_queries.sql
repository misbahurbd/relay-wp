-- ---------- API keys ----------

-- name: ListAPIKeys :many
SELECT k.id, k.name, k.app, k.prefix, k.last4, k.created_by,
       COALESCE(NULLIF(p.display_name, ''), NULLIF(trim(concat_ws(' ', p.first_name, p.last_name)), ''), u.email::text)::text AS created_by_name,
       k.last_used_at, k.created_at
FROM api_keys k
JOIN users u ON u.id = k.created_by
LEFT JOIN user_profiles p ON p.user_id = k.created_by
WHERE k.workspace_id = @workspace_id AND k.revoked_at IS NULL
ORDER BY k.created_at DESC, k.id DESC;

-- name: CountActiveAPIKeys :one
SELECT count(*) FROM api_keys WHERE workspace_id = @workspace_id AND revoked_at IS NULL;

-- name: CreateAPIKey :exec
INSERT INTO api_keys (id, workspace_id, name, app, prefix, last4, token_hash, created_by)
VALUES (@id, @workspace_id, @name, sqlc.narg(app), @prefix, @last4, @token_hash, @created_by);

-- name: GetActiveAPIKey :one
SELECT id, name, app FROM api_keys
WHERE workspace_id = @workspace_id AND id = @id AND revoked_at IS NULL
FOR UPDATE;

-- name: RevokeAPIKey :execrows
UPDATE api_keys SET revoked_at = now()
WHERE workspace_id = @workspace_id AND id = @id AND revoked_at IS NULL;

-- name: GetAPIKeyByHash :one
-- The one lookup without a workspace: the key is how the workspace is found.
-- RequireWorkspace then checks the creator's current access there.
SELECT k.id, k.workspace_id, k.created_by, k.last_used_at
FROM api_keys k
WHERE k.token_hash = @token_hash AND k.revoked_at IS NULL;

-- name: TouchAPIKey :exec
-- At most one write a minute per key.
UPDATE api_keys SET last_used_at = now()
WHERE workspace_id = @workspace_id AND id = @id AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute');

-- name: LockWorkspaceIntegrations :exec
-- One workspace's key and endpoint creations take turns, so the limits hold under concurrency.
SELECT pg_advisory_xact_lock(hashtextextended('integrations:' || sqlc.arg(workspace_id)::uuid, 0));
