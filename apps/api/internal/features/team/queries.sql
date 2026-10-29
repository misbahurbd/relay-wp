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

-- ---------- Members ----------

-- name: ListMembers :many
-- Cursor: (joined_at, user_id) of the last row of the previous page.
SELECT m.user_id, u.email,
       COALESCE(NULLIF(p.display_name, ''), NULLIF(trim(concat_ws(' ', p.first_name, p.last_name)), ''), u.email::text)::text AS name,
       p.avatar_url, m.role_id, r.name AS role_name, r.system_key, r.uses_seat, m.status, m.availability,
       m.last_active_at, m.joined_at,
       ARRAY(SELECT tm.team_id FROM team_members tm
             WHERE tm.workspace_id = m.workspace_id AND tm.user_id = m.user_id ORDER BY tm.team_id)::uuid[] AS team_ids
FROM workspace_members m
JOIN roles r ON r.id = m.role_id
JOIN users u ON u.id = m.user_id
LEFT JOIN user_profiles p ON p.user_id = m.user_id
WHERE m.workspace_id = @workspace_id
  AND (sqlc.narg(role_id)::uuid IS NULL OR m.role_id = sqlc.narg(role_id))
  AND (sqlc.narg(status)::text IS NULL OR m.status = sqlc.narg(status))
  AND (sqlc.narg(team_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM team_members tm WHERE tm.workspace_id = m.workspace_id AND tm.user_id = m.user_id
          AND tm.team_id = sqlc.narg(team_id)))
  AND (sqlc.narg(query)::text IS NULL OR u.email ILIKE '%' || sqlc.narg(query) || '%'
       OR concat_ws(' ', p.display_name, p.first_name, p.last_name) ILIKE '%' || sqlc.narg(query) || '%')
  AND (sqlc.narg(after_joined_at)::timestamptz IS NULL
       OR (m.joined_at, m.user_id) > (sqlc.narg(after_joined_at)::timestamptz, sqlc.narg(after_user_id)::uuid))
ORDER BY m.joined_at, m.user_id
LIMIT @page_size;

-- name: GetMember :one
SELECT m.user_id, u.email,
       COALESCE(NULLIF(p.display_name, ''), NULLIF(trim(concat_ws(' ', p.first_name, p.last_name)), ''), u.email::text)::text AS name,
       p.avatar_url, m.role_id, r.name AS role_name, r.system_key, r.uses_seat, m.status, m.availability,
       m.last_active_at, m.joined_at, m.chat_scope, r.chat_scope AS role_chat_scope,
       m.suspended_at, m.suspend_reason, m.access_version,
       ARRAY(SELECT tm.team_id FROM team_members tm
             WHERE tm.workspace_id = m.workspace_id AND tm.user_id = m.user_id ORDER BY tm.team_id)::uuid[] AS team_ids,
       ARRAY(SELECT mn.number_id FROM member_numbers mn
             WHERE mn.workspace_id = m.workspace_id AND mn.user_id = m.user_id ORDER BY mn.number_id)::uuid[] AS number_ids
FROM workspace_members m
JOIN roles r ON r.id = m.role_id
JOIN users u ON u.id = m.user_id
LEFT JOIN user_profiles p ON p.user_id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.user_id = @user_id;

-- name: LockMember :one
SELECT m.role_id, m.status, m.access_version, COALESCE(r.system_key = 'owner', false)::boolean AS is_owner,
       r.uses_seat, u.email,
       COALESCE(NULLIF(p.display_name, ''), NULLIF(trim(concat_ws(' ', p.first_name, p.last_name)), ''), u.email::text)::text AS name
FROM workspace_members m
JOIN roles r ON r.id = m.role_id
JOIN users u ON u.id = m.user_id
LEFT JOIN user_profiles p ON p.user_id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.user_id = @user_id
FOR UPDATE OF m;

-- name: AddMember :exec
INSERT INTO workspace_members (workspace_id, user_id, role_id, chat_scope, invited_by)
VALUES (@workspace_id, @user_id, @role_id, @chat_scope, @invited_by);

-- name: SetMemberRole :exec
UPDATE workspace_members
SET role_id = @role_id, access_version = access_version + 1
WHERE workspace_id = @workspace_id AND user_id = @user_id;

-- name: SetMemberScope :exec
UPDATE workspace_members
SET chat_scope = @chat_scope, access_version = access_version + 1
WHERE workspace_id = @workspace_id AND user_id = @user_id;

-- name: SetMemberStatus :exec
UPDATE workspace_members
SET status = @status, suspended_at = @suspended_at, suspended_by = @suspended_by,
    suspend_reason = @suspend_reason, access_version = access_version + 1
WHERE workspace_id = @workspace_id AND user_id = @user_id;

-- name: SetAvailability :exec
UPDATE workspace_members
SET availability = @availability
WHERE workspace_id = @workspace_id AND user_id = @user_id;

-- name: BumpAccessVersions :exec
UPDATE workspace_members
SET access_version = access_version + 1
WHERE workspace_id = @workspace_id AND user_id = ANY (@user_ids::uuid[]);

-- name: DeleteMember :execrows
DELETE FROM workspace_members
WHERE workspace_id = @workspace_id AND user_id = @user_id;

-- name: IsMemberEmail :one
SELECT EXISTS (
    SELECT 1 FROM workspace_members m JOIN users u ON u.id = m.user_id
    WHERE m.workspace_id = @workspace_id AND u.email = @email
);

-- name: SetMemberTeams :exec
-- Replaces the member's teams with @team_ids (teams must belong to the workspace; the foreign key checks).
WITH removed AS (
    DELETE FROM team_members
    WHERE workspace_id = @workspace_id AND user_id = @user_id AND NOT (team_id = ANY (@team_ids::uuid[]))
)
INSERT INTO team_members (workspace_id, team_id, user_id)
SELECT @workspace_id, t, @user_id FROM unnest(@team_ids::uuid[]) AS t
ON CONFLICT DO NOTHING;

-- name: SetMemberNumbers :execrows
-- Replaces the member's numbers with those of @number_ids that belong to the workspace.
-- Returns how many were kept, so the caller can spot foreign IDs.
WITH removed AS (
    DELETE FROM member_numbers
    WHERE workspace_id = @workspace_id AND user_id = @user_id AND NOT (number_id = ANY (@number_ids::uuid[]))
)
INSERT INTO member_numbers (workspace_id, user_id, number_id)
SELECT @workspace_id, @user_id, n.id
FROM whatsapp_numbers n
WHERE n.workspace_id = @workspace_id AND n.id = ANY (@number_ids::uuid[])
ON CONFLICT ON CONSTRAINT member_numbers_pkey DO UPDATE SET created_at = member_numbers.created_at;

-- name: CountSeatsUsed :one
-- Members and pending invites whose role uses a seat.
SELECT
    (SELECT count(*) FROM workspace_members m JOIN roles r ON r.id = m.role_id
     WHERE m.workspace_id = @workspace_id AND r.uses_seat) AS members,
    (SELECT count(*) FROM invitations i JOIN roles r ON r.id = i.role_id
     WHERE i.workspace_id = @workspace_id AND i.status = 'pending' AND i.expires_at > now() AND r.uses_seat) AS reserved;

-- name: CountWorkspaceNumbers :one
SELECT count(*)
FROM whatsapp_numbers
WHERE workspace_id = @workspace_id AND id = ANY (@number_ids::uuid[]);

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

-- ---------- Teams ----------

-- name: ListTeams :many
SELECT t.id, t.name, t.description, t.color, t.number_id, t.routing_method,
       ARRAY(SELECT tm.user_id FROM team_members tm WHERE tm.team_id = t.id ORDER BY tm.created_at)::uuid[] AS member_ids
FROM teams t
WHERE t.workspace_id = @workspace_id
ORDER BY lower(t.name);

-- name: GetTeam :one
SELECT t.id, t.name, t.description, t.color, t.number_id, t.routing_method,
       ARRAY(SELECT tm.user_id FROM team_members tm WHERE tm.team_id = t.id ORDER BY tm.created_at)::uuid[] AS member_ids
FROM teams t
WHERE t.workspace_id = @workspace_id AND t.id = @team_id;

-- name: CreateTeam :exec
INSERT INTO teams (id, workspace_id, name, description, color, number_id, routing_method)
VALUES (@id, @workspace_id, @name, @description, @color, @number_id, @routing_method);

-- name: UpdateTeam :execrows
UPDATE teams
SET name = @name, description = @description, color = @color, number_id = @number_id, routing_method = @routing_method
WHERE workspace_id = @workspace_id AND id = @team_id;

-- name: DeleteTeam :execrows
DELETE FROM teams
WHERE workspace_id = @workspace_id AND id = @team_id;

-- name: TeamMemberIDs :many
SELECT user_id
FROM team_members
WHERE workspace_id = @workspace_id AND team_id = @team_id;

-- name: SetTeamMembers :exec
WITH removed AS (
    DELETE FROM team_members
    WHERE workspace_id = @workspace_id AND team_id = @team_id AND NOT (user_id = ANY (@user_ids::uuid[]))
)
INSERT INTO team_members (workspace_id, team_id, user_id)
SELECT @workspace_id, @team_id, u FROM unnest(@user_ids::uuid[]) AS u
ON CONFLICT DO NOTHING;

-- ---------- Invitations ----------

-- name: CreateInvitation :exec
INSERT INTO invitations (id, workspace_id, email, role_id, chat_scope, token_hash, personal_message, require_2fa, invited_by, expires_at)
VALUES (@id, @workspace_id, @email, @role_id, @chat_scope, @token_hash, @personal_message, @require_2fa, @invited_by, @expires_at);

-- name: AddInvitationTeams :exec
INSERT INTO invitation_teams (workspace_id, invitation_id, team_id)
SELECT @workspace_id, @invitation_id, t FROM unnest(@team_ids::uuid[]) AS t;

-- name: AddInvitationNumbers :exec
INSERT INTO invitation_numbers (workspace_id, invitation_id, number_id)
SELECT @workspace_id, @invitation_id, n.id
FROM whatsapp_numbers n
WHERE n.workspace_id = @workspace_id AND n.id = ANY (@number_ids::uuid[]);

-- name: AddInvitationOverrides :exec
INSERT INTO invitation_permission_overrides (workspace_id, invitation_id, permission_key, effect)
SELECT @workspace_id, @invitation_id, unnest(@keys::text[]), unnest(@effects::text[]);

-- name: ListInvitations :many
SELECT i.id, i.email, i.role_id, r.name AS role_name, i.status, i.expires_at, i.last_sent_at, i.send_count,
       i.renewal_requested_at, i.created_at,
       COALESCE(NULLIF(p.display_name, ''), NULLIF(trim(concat_ws(' ', p.first_name, p.last_name)), ''), '')::text AS invited_by_name,
       ARRAY(SELECT it.team_id FROM invitation_teams it WHERE it.invitation_id = i.id ORDER BY it.team_id)::uuid[] AS team_ids
FROM invitations i
JOIN roles r ON r.id = i.role_id
LEFT JOIN user_profiles p ON p.user_id = i.invited_by
WHERE i.workspace_id = @workspace_id
  AND (i.status = 'pending' OR i.updated_at > now() - interval '30 days')
ORDER BY i.created_at DESC
LIMIT 200;

-- name: LockInvitation :one
SELECT id, email, role_id, status, expires_at, send_count
FROM invitations
WHERE workspace_id = @workspace_id AND id = @invitation_id
FOR UPDATE;

-- name: GetInvitationByTokenHash :one
-- The only query that reads token_hash (CLAUDE.md secrets rule).
SELECT i.id, i.workspace_id, i.email, i.role_id, i.chat_scope, i.status, i.expires_at, i.personal_message,
       i.invited_by, w.name AS workspace_name, r.name AS role_name, r.system_key, r.uses_seat,
       COALESCE(NULLIF(p.display_name, ''), NULLIF(trim(concat_ws(' ', p.first_name, p.last_name)), ''), '')::text AS invited_by_name,
       COALESCE((SELECT x.name FROM roles x
                 JOIN workspace_members im ON im.role_id = x.id
                 WHERE im.workspace_id = i.workspace_id AND im.user_id = i.invited_by), '')::text AS invited_by_role
FROM invitations i
JOIN workspaces w ON w.id = i.workspace_id
JOIN roles r ON r.id = i.role_id
LEFT JOIN user_profiles p ON p.user_id = i.invited_by
WHERE i.token_hash = @token_hash;

-- name: GetWorkspaceName :one
SELECT name FROM workspaces WHERE id = @workspace_id;

-- name: InvitationTeamIDs :many
SELECT team_id FROM invitation_teams WHERE invitation_id = @invitation_id;

-- name: InvitationNumberIDs :many
SELECT number_id FROM invitation_numbers WHERE invitation_id = @invitation_id;

-- name: InvitationOverrides :many
SELECT permission_key, effect FROM invitation_permission_overrides WHERE invitation_id = @invitation_id;

-- name: RoleBits :one
SELECT COALESCE((SELECT bit_or(1::bigint << p.bit) FROM role_permissions rp
                 JOIN permissions p ON p.key = rp.permission_key
                 WHERE rp.role_id = @role_id), 0)::bigint AS bits;

-- name: ResendInvitation :exec
UPDATE invitations
SET token_hash = @token_hash, expires_at = @expires_at, send_count = send_count + 1, last_sent_at = now()
WHERE workspace_id = @workspace_id AND id = @invitation_id;

-- name: RevokeInvitation :exec
UPDATE invitations
SET status = 'revoked', revoked_at = now(), revoked_by = @revoked_by
WHERE workspace_id = @workspace_id AND id = @invitation_id;

-- name: AcceptInvitation :exec
UPDATE invitations
SET status = 'accepted', accepted_at = now(), accepted_user_id = @user_id
WHERE id = @invitation_id;

-- name: RequestInvitationRenewal :execrows
UPDATE invitations
SET renewal_requested_at = now()
WHERE id = @invitation_id AND renewal_requested_at IS NULL;

-- ---------- Access log ----------

-- name: InsertAccessLog :exec
INSERT INTO access_log_events (id, workspace_id, actor_user_id, subject_user_id, subject_email, action, target,
                               before_value, after_value, note, ip, user_agent)
VALUES (@id, @workspace_id, @actor_user_id, @subject_user_id, @subject_email, @action, @target,
        @before_value, @after_value, @note, @ip, @user_agent);

-- name: ListAccessLog :many
-- Newest first. Cursor: (created_at, id) of the last row of the previous page.
SELECT e.id, e.created_at, e.actor_user_id, e.subject_user_id, e.subject_email, e.action, e.target,
       e.before_value, e.after_value, e.note, e.ip, e.user_agent,
       COALESCE(NULLIF(ap.display_name, ''), NULLIF(trim(concat_ws(' ', ap.first_name, ap.last_name)), ''), '')::text AS actor_name,
       COALESCE(NULLIF(sp.display_name, ''), NULLIF(trim(concat_ws(' ', sp.first_name, sp.last_name)), ''), '')::text AS subject_name
FROM access_log_events e
LEFT JOIN user_profiles ap ON ap.user_id = e.actor_user_id
LEFT JOIN user_profiles sp ON sp.user_id = e.subject_user_id
WHERE e.workspace_id = @workspace_id
  AND e.created_at >= @since
  AND (sqlc.narg(action)::text IS NULL OR e.action = sqlc.narg(action))
  AND (sqlc.narg(subject_user_id)::uuid IS NULL OR e.subject_user_id = sqlc.narg(subject_user_id))
  AND (sqlc.narg(before_created_at)::timestamptz IS NULL
       OR (e.created_at, e.id) < (sqlc.narg(before_created_at)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY e.created_at DESC, e.id DESC
LIMIT @page_size;

-- name: InvitationTeamNames :many
SELECT t.name
FROM invitation_teams it
JOIN teams t ON t.workspace_id = it.workspace_id AND t.id = it.team_id
WHERE it.workspace_id = @workspace_id AND it.invitation_id = @invitation_id
ORDER BY t.name;

-- name: InvitationNumberLabels :many
SELECT n.verified_name, n.display_phone_number
FROM invitation_numbers inn
JOIN whatsapp_numbers n ON n.id = inn.number_id AND n.workspace_id = inn.workspace_id
WHERE inn.workspace_id = @workspace_id AND inn.invitation_id = @invitation_id
ORDER BY n.verified_name;

-- name: AccessLogSummary :one
SELECT count(*)::bigint AS events,
       count(*) FILTER (WHERE action IN ('role_changed', 'permission_granted', 'permission_denied',
                                         'permission_revoked', 'permission_expired', 'role_updated'))::bigint AS permission_changes
FROM access_log_events
WHERE workspace_id = @workspace_id AND created_at >= @since;

-- name: ExpiringOverrides :many
SELECT o.user_id, o.permission_key, o.expires_at,
       COALESCE(NULLIF(p.display_name, ''), NULLIF(p.first_name, ''), u.email::text)::text AS name
FROM member_permission_overrides o
JOIN users u ON u.id = o.user_id
LEFT JOIN user_profiles p ON p.user_id = o.user_id
WHERE o.workspace_id = @workspace_id AND o.revoked_at IS NULL
  AND o.expires_at > now() AND o.expires_at <= @until
ORDER BY o.expires_at;

-- name: FailedLoginAlerts :many
-- Members with 3+ failed sign-ins in the window. Reads auth_events directly, never secrets.
SELECT e.user_id, count(*)::bigint AS failures, max(e.created_at)::timestamptz AS last_at,
       COALESCE(NULLIF(p.display_name, ''), NULLIF(trim(concat_ws(' ', p.first_name, p.last_name)), ''), u.email::text)::text AS name
FROM auth_events e
JOIN workspace_members m ON m.user_id = e.user_id AND m.workspace_id = @workspace_id
JOIN users u ON u.id = e.user_id
LEFT JOIN user_profiles p ON p.user_id = e.user_id
WHERE e.event = 'login_failed' AND e.created_at >= @since
GROUP BY e.user_id, p.display_name, p.first_name, p.last_name, u.email
HAVING count(*) >= 3
ORDER BY max(e.created_at) DESC;
