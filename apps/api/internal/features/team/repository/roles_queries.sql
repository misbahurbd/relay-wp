-- ---------- Roles ----------

-- name: ListRoles :many
SELECT r.id, r.system_key, r.name, r.description, r.color, r.chat_scope, r.uses_seat, r.locked, r.version,
       ARRAY(SELECT rp.permission_key FROM role_permissions rp WHERE rp.role_id = r.id ORDER BY rp.permission_key)::text[] AS permissions,
       (SELECT count(*) FROM workspace_members m WHERE m.workspace_id = r.workspace_id AND m.role_id = r.id) AS member_count
FROM roles r
WHERE r.workspace_id = @workspace_id
ORDER BY r.system_key IS NULL, array_position(ARRAY['owner', 'admin', 'agent', 'viewer'], r.system_key), lower(r.name);

-- name: GetRole :one
SELECT r.id, r.system_key, r.name, r.description, r.color, r.chat_scope, r.uses_seat, r.locked, r.version,
       ARRAY(SELECT rp.permission_key FROM role_permissions rp WHERE rp.role_id = r.id ORDER BY rp.permission_key)::text[] AS permissions,
       (SELECT count(*) FROM workspace_members m WHERE m.workspace_id = r.workspace_id AND m.role_id = r.id) AS member_count
FROM roles r
WHERE r.workspace_id = @workspace_id AND r.id = @role_id;

-- name: LockRole :one
SELECT version
FROM roles
WHERE workspace_id = @workspace_id AND id = @role_id
FOR UPDATE;

-- name: GetSystemRoleID :one
SELECT id
FROM roles
WHERE workspace_id = @workspace_id AND system_key = @system_key;

-- name: CreateRole :exec
INSERT INTO roles (id, workspace_id, system_key, name, description, color, chat_scope, uses_seat, locked, created_by)
VALUES (@id, @workspace_id, @system_key, @name, @description, @color, @chat_scope, @uses_seat, @locked, @created_by);

-- name: UpdateRole :exec
UPDATE roles
SET name = @name, description = @description, color = @color, chat_scope = @chat_scope, version = version + 1
WHERE workspace_id = @workspace_id AND id = @role_id;

-- name: SetRolePermissions :exec
-- Replaces the role's permissions with @keys.
WITH removed AS (
    DELETE FROM role_permissions
    WHERE role_id = @role_id AND NOT (permission_key = ANY (@keys::text[]))
)
INSERT INTO role_permissions (workspace_id, role_id, permission_key)
SELECT @workspace_id, @role_id, k FROM unnest(@keys::text[]) AS k
ON CONFLICT DO NOTHING;

-- name: DeleteRole :execrows
DELETE FROM roles
WHERE workspace_id = @workspace_id AND id = @role_id AND system_key IS NULL;

-- name: MoveRoleMembers :many
UPDATE workspace_members
SET role_id = @to_role_id, access_version = access_version + 1
WHERE workspace_id = @workspace_id AND role_id = @from_role_id
RETURNING user_id;

-- name: MoveRoleInvitations :exec
UPDATE invitations
SET role_id = @to_role_id
WHERE workspace_id = @workspace_id AND role_id = @from_role_id;
