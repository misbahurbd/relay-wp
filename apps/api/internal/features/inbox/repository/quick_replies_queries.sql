-- ---------- Quick replies (docs/PHASE-7-INBOX.md F8) ----------

-- name: ListQuickReplies :many
SELECT q.id, q.shortcut, q.title, q.body, q.created_at, q.updated_at
FROM quick_replies q
WHERE q.workspace_id = @workspace_id
ORDER BY q.shortcut;

-- name: CreateQuickReply :one
INSERT INTO quick_replies (id, workspace_id, shortcut, title, body, created_by)
VALUES (@id, @workspace_id, @shortcut, @title, @body, @created_by)
RETURNING id, shortcut, title, body, created_at, updated_at;

-- name: UpdateQuickReply :one
UPDATE quick_replies
SET shortcut = COALESCE(sqlc.narg(shortcut), shortcut),
    title    = COALESCE(sqlc.narg(title), title),
    body     = COALESCE(sqlc.narg(body), body)
WHERE workspace_id = @workspace_id AND id = @id
RETURNING id, shortcut, title, body, created_at, updated_at;

-- name: DeleteQuickReply :execrows
DELETE FROM quick_replies
WHERE workspace_id = @workspace_id AND id = @id;
