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
