-- ---------- Audience ----------
-- A contact is in the audience when @everyone is set, or it has any of @tag_ids, or
-- it is one of @contact_ids (segment and list members, resolved by the
-- service). Only opted-in contacts get it; with @skip_recent, neither do
-- those sent a marketing template since @recent_since.

-- name: AudienceCount :one
-- Who matches, how many of them opted in, and how many of those the
-- recent-marketing rule leaves out.
WITH matched AS (
    SELECT c.opt_in,
           EXISTS (SELECT 1 FROM conversations cv JOIN messages m ON m.conversation_id = cv.id
                   WHERE cv.workspace_id = c.workspace_id AND cv.contact_id = c.id AND m.direction = 'out'
                     AND m.type = 'template' AND m.content -> 'template' ->> 'category' = 'marketing'
                     AND m.created_at > @recent_since::timestamptz) AS recent
    FROM contacts c
    WHERE c.workspace_id = @workspace_id
      AND (@everyone::boolean
           OR c.id = ANY(@contact_ids::uuid[])
           OR EXISTS (SELECT 1 FROM contact_tag_links l
                      WHERE l.workspace_id = c.workspace_id AND l.contact_id = c.id AND l.tag_id = ANY(@tag_ids::uuid[])))
)
SELECT count(*)::int                                                        AS matched,
       count(*) FILTER (WHERE opt_in = 'in')::int                             AS opted_in,
       count(*) FILTER (WHERE opt_in = 'in' AND recent)::int                  AS recent
FROM matched;

-- name: SnapshotRecipients :execrows
-- Takes the audience once, when sending starts.
INSERT INTO broadcast_recipients (workspace_id, broadcast_id, contact_id, phone, name)
SELECT c.workspace_id, @broadcast_id, c.id, c.phone, c.name
FROM contacts c
WHERE c.workspace_id = @workspace_id AND c.opt_in = 'in'
  AND (@everyone::boolean
       OR c.id = ANY(@contact_ids::uuid[])
       OR EXISTS (SELECT 1 FROM contact_tag_links l
                  WHERE l.workspace_id = c.workspace_id AND l.contact_id = c.id AND l.tag_id = ANY(@tag_ids::uuid[])))
  AND NOT (@skip_recent::boolean AND EXISTS (
      SELECT 1 FROM conversations cv JOIN messages m ON m.conversation_id = cv.id
      WHERE cv.workspace_id = c.workspace_id AND cv.contact_id = c.id AND m.direction = 'out'
        AND m.type = 'template' AND m.content -> 'template' ->> 'category' = 'marketing'
        AND m.created_at > @recent_since::timestamptz))
ON CONFLICT DO NOTHING;

-- ---------- Sending ----------

-- name: ClaimRecipients :many
-- The next pending recipients, locked so a second batch job skips them. The
-- contact's current attributes fill variables; a deleted contact has none.
SELECT r.contact_id, r.phone, r.name, c.attributes
FROM broadcast_recipients r
LEFT JOIN contacts c ON c.workspace_id = r.workspace_id AND c.id = r.contact_id
WHERE r.workspace_id = @workspace_id AND r.broadcast_id = @broadcast_id AND r.state = 'pending'
ORDER BY r.contact_id
LIMIT @lim
FOR UPDATE OF r SKIP LOCKED;

-- name: SetRecipientQueued :exec
UPDATE broadcast_recipients SET state = 'queued', message_id = @message_id, queued_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND broadcast_id = @broadcast_id AND contact_id = @contact_id;

-- name: SetRecipientOutcome :exec
UPDATE broadcast_recipients SET state = @state, reason = @reason
WHERE workspace_id = @workspace_id AND broadcast_id = @broadcast_id AND contact_id = @contact_id;

-- name: CountQueued :one
SELECT count(*)::int FROM broadcast_recipients
WHERE workspace_id = @workspace_id AND broadcast_id = @broadcast_id AND state = 'queued';

-- name: SkipPending :exec
UPDATE broadcast_recipients SET state = 'skipped', reason = @reason
WHERE workspace_id = @workspace_id AND broadcast_id = @broadcast_id AND state = 'pending';

-- ---------- Reports ----------

-- name: BroadcastStats :one
SELECT count(*) FILTER (WHERE r.state = 'pending')::int                       AS pending,
       count(*) FILTER (WHERE r.state = 'skipped')::int                       AS skipped,
       count(*) FILTER (WHERE r.state = 'failed' OR m.status = 'failed')::int AS failed,
       count(m.id) FILTER (WHERE m.status = 'pending')::int                   AS queued,
       count(m.id) FILTER (WHERE m.status IN ('sent', 'delivered', 'read'))::int AS sent,
       count(m.id) FILTER (WHERE m.status IN ('delivered', 'read'))::int      AS delivered,
       count(m.id) FILTER (WHERE m.status = 'read')::int                      AS read,
       count(m.id) FILTER (WHERE EXISTS (
           SELECT 1 FROM messages i
           WHERE i.conversation_id = m.conversation_id AND i.direction = 'in' AND i.id > m.id))::int AS replied
FROM broadcast_recipients r
LEFT JOIN messages m ON m.id = r.message_id
WHERE r.workspace_id = @workspace_id AND r.broadcast_id = @broadcast_id;

-- name: FailureReasons :many
-- Why recipients didn't get it, most common first: Meta's error for a failed
-- send, else the reason the broadcast skipped or failed them.
SELECT COALESCE(m.error_title, r.reason, 'unknown')::text AS reason, count(*)::int AS recipients
FROM broadcast_recipients r
LEFT JOIN messages m ON m.id = r.message_id
WHERE r.workspace_id = @workspace_id AND r.broadcast_id = @broadcast_id
  AND (r.state IN ('skipped', 'failed') OR m.status = 'failed')
GROUP BY 1
ORDER BY 2 DESC
LIMIT 10;

-- name: ListRecipients :many
SELECT r.contact_id, r.name, r.phone, r.state, r.reason, r.queued_at, m.status AS message_status,
       m.status_at, m.error_title,
       (m.id IS NOT NULL AND EXISTS (
           SELECT 1 FROM messages i
           WHERE i.conversation_id = m.conversation_id AND i.direction = 'in' AND i.id > m.id))::boolean AS replied
FROM broadcast_recipients r
LEFT JOIN messages m ON m.id = r.message_id
WHERE r.workspace_id = @workspace_id AND r.broadcast_id = @broadcast_id
  AND (sqlc.narg('after_id')::uuid IS NULL OR r.contact_id > sqlc.narg('after_id'))
  AND CASE sqlc.narg('filter')::text
        WHEN 'failed' THEN r.state IN ('skipped', 'failed') OR m.status = 'failed'
        WHEN 'delivered' THEN m.status IN ('delivered', 'read')
        WHEN 'read' THEN m.status = 'read'
        ELSE true
      END
ORDER BY r.contact_id
LIMIT @lim;

-- name: BroadcastSentCount :one
-- Messages Meta accepted: a broadcast that reached anyone activates the workspace.
SELECT count(*)::int FROM broadcast_recipients r JOIN messages m ON m.id = r.message_id
WHERE r.workspace_id = @workspace_id AND r.broadcast_id = @broadcast_id AND m.status IN ('sent', 'delivered', 'read');
