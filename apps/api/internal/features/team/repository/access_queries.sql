-- ---------- Access (ADR 0002: one query per request) ----------

-- name: ResolveAccess :one
SELECT m.status, m.role_id,
       COALESCE(r.system_key = 'owner', false)::boolean AS is_owner,
       COALESCE(m.chat_scope, r.chat_scope)::text AS scope,
       COALESCE((SELECT bit_or(1::bigint << p.bit) FROM role_permissions rp
                 JOIN permissions p ON p.key = rp.permission_key
                 WHERE rp.role_id = m.role_id), 0)::bigint AS role_bits,
       COALESCE((SELECT bit_or(1::bigint << p.bit) FROM member_permission_overrides o
                 JOIN permissions p ON p.key = o.permission_key
                 WHERE o.workspace_id = m.workspace_id AND o.user_id = m.user_id AND o.revoked_at IS NULL
                   AND o.effect = 'grant' AND (o.expires_at IS NULL OR o.expires_at > now())), 0)::bigint AS grant_bits,
       COALESCE((SELECT bit_or(1::bigint << p.bit) FROM member_permission_overrides o
                 JOIN permissions p ON p.key = o.permission_key
                 WHERE o.workspace_id = m.workspace_id AND o.user_id = m.user_id AND o.revoked_at IS NULL
                   AND o.effect = 'deny' AND (o.expires_at IS NULL OR o.expires_at > now())), 0)::bigint AS deny_bits,
       ARRAY(SELECT tm.team_id FROM team_members tm
             WHERE tm.workspace_id = m.workspace_id AND tm.user_id = m.user_id ORDER BY tm.team_id)::uuid[] AS team_ids,
       ARRAY(SELECT mn.number_id FROM member_numbers mn
             WHERE mn.workspace_id = m.workspace_id AND mn.user_id = m.user_id ORDER BY mn.number_id)::uuid[] AS number_ids,
       r.version AS role_version, m.access_version
FROM workspace_members m
JOIN roles r ON r.id = m.role_id
WHERE m.workspace_id = @workspace_id AND m.user_id = @user_id;

-- name: ListPermissions :many
SELECT key, bit, group_key, label, sensitive
FROM permissions
ORDER BY position;
