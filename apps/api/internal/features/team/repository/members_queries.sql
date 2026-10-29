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
