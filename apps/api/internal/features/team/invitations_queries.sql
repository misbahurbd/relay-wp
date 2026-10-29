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
