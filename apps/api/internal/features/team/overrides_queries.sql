-- ---------- Overrides ----------

-- name: ListMemberOverrides :many
SELECT o.id, o.permission_key, o.effect, o.reason, o.expires_at, o.revoked_at, o.created_at,
       o.granted_by,
       COALESCE(NULLIF(p.display_name, ''), NULLIF(trim(concat_ws(' ', p.first_name, p.last_name)), ''), '')::text AS granted_by_name
FROM member_permission_overrides o
LEFT JOIN user_profiles p ON p.user_id = o.granted_by
WHERE o.workspace_id = @workspace_id AND o.user_id = @user_id
  AND (o.revoked_at IS NULL OR o.revoked_at > now() - interval '30 days')
ORDER BY o.revoked_at IS NOT NULL, o.created_at DESC
LIMIT 50;

-- name: CreateOverride :exec
INSERT INTO member_permission_overrides (id, workspace_id, user_id, permission_key, effect, reason, granted_by, expires_at)
VALUES (@id, @workspace_id, @user_id, @permission_key, @effect, @reason, @granted_by, @expires_at);

-- name: RevokeOverride :one
UPDATE member_permission_overrides
SET revoked_at = now(), revoked_by = @revoked_by
WHERE workspace_id = @workspace_id AND user_id = @user_id AND id = @id AND revoked_at IS NULL
RETURNING permission_key, effect;

-- name: RevokeActiveOverride :exec
-- Clears an active override for the same permission before a new one is added.
UPDATE member_permission_overrides
SET revoked_at = now(), revoked_by = @revoked_by
WHERE workspace_id = @workspace_id AND user_id = @user_id AND permission_key = @permission_key AND revoked_at IS NULL;

-- name: ExpireOverrides :many
-- Marks overrides past their end date as revoked, so they're processed once.
UPDATE member_permission_overrides
SET revoked_at = expires_at
WHERE revoked_at IS NULL AND expires_at <= now()
RETURNING workspace_id, user_id, permission_key, effect, granted_by;
