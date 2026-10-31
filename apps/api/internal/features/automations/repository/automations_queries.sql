-- name: ListAutomations :many
SELECT kind, enabled, settings, updated_at
FROM automations
WHERE workspace_id = @workspace_id;

-- name: GetAutomation :one
SELECT kind, enabled, settings, paused_at, updated_at
FROM automations
WHERE workspace_id = @workspace_id AND kind = @kind;

-- name: SaveAutomationSettings :one
INSERT INTO automations (id, workspace_id, kind, settings, updated_by)
VALUES (@id, @workspace_id, @kind, @settings, @updated_by)
ON CONFLICT (workspace_id, kind) DO UPDATE
    SET settings = EXCLUDED.settings, updated_by = EXCLUDED.updated_by
RETURNING kind, enabled, settings, updated_at;

-- name: SetAutomationEnabled :one
-- settings is only used when the kind has no row yet.
INSERT INTO automations (id, workspace_id, kind, enabled, settings, updated_by)
VALUES (@id, @workspace_id, @kind, @enabled, @settings, @updated_by)
ON CONFLICT (workspace_id, kind) DO UPDATE
    SET enabled = EXCLUDED.enabled, updated_by = EXCLUDED.updated_by,
        paused_at = CASE WHEN EXCLUDED.enabled THEN NULL
                         WHEN automations.enabled THEN now()
                         ELSE automations.paused_at END
RETURNING kind, enabled, settings, updated_at;
