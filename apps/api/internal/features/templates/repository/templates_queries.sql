-- name: CreateTemplate :exec
INSERT INTO message_templates (id, workspace_id, name, language, category, components, created_by)
VALUES (@id, @workspace_id, @name, @language, @category, @components, @created_by);

-- name: GetTemplate :one
SELECT id, account_id, name, language, category, status, meta_template_id, rejected_reason, components,
       header_media_key, header_media_type, header_media_size, submitted_at, created_by, created_at, updated_at
FROM message_templates
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListTemplates :many
-- Newest first. Cursor: the id of the last row of the previous page (UUIDv7 sorts by time).
SELECT id, account_id, name, language, category, status, meta_template_id, rejected_reason, components,
       header_media_key, header_media_type, header_media_size, submitted_at, created_by, created_at, updated_at
FROM message_templates
WHERE workspace_id = @workspace_id
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(category)::text IS NULL OR category = sqlc.narg(category))
  AND (sqlc.narg(q)::text IS NULL OR name ILIKE '%' || sqlc.narg(q) || '%')
  AND (sqlc.narg(before_id)::uuid IS NULL OR id < sqlc.narg(before_id))
ORDER BY id DESC
LIMIT @page_size;

-- name: CountTemplates :one
-- Tab counts for the list; they follow the category and search filters, not the status one.
SELECT count(id)::bigint AS all_count,
       count(id) FILTER (WHERE status = 'approved')::bigint AS approved,
       count(id) FILTER (WHERE status = 'pending')::bigint AS pending,
       count(id) FILTER (WHERE status = 'rejected')::bigint AS rejected
FROM message_templates
WHERE workspace_id = @workspace_id
  AND (sqlc.narg(category)::text IS NULL OR category = sqlc.narg(category))
  AND (sqlc.narg(q)::text IS NULL OR name ILIKE '%' || sqlc.narg(q) || '%');

-- name: UpdateTemplate :execrows
-- Only while editable: a template Meta is reviewing (or has paused or disabled) can't change.
UPDATE message_templates
SET name = @name, language = @language, category = @category, components = @components
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('draft', 'rejected', 'approved');

-- name: SetTemplateSample :exec
UPDATE message_templates
SET header_media_key = @header_media_key, header_media_type = @header_media_type,
    header_media_size = @header_media_size
WHERE workspace_id = @workspace_id AND id = @id;

-- name: MarkTemplateSubmitted :exec
-- Records the submission to the workspace's active WhatsApp account (the one submit used).
UPDATE message_templates
SET meta_template_id = @meta_template_id, status = @status, rejected_reason = NULL, submitted_at = now(),
    account_id = (SELECT a.id FROM whatsapp_accounts a
                  WHERE a.workspace_id = @workspace_id AND a.status = 'active'
                  ORDER BY a.created_at DESC, a.id DESC LIMIT 1)
WHERE message_templates.workspace_id = @workspace_id AND message_templates.id = @id;

-- name: DeleteTemplate :exec
DELETE FROM message_templates
WHERE workspace_id = @workspace_id AND id = @id;

-- name: UpsertSyncedTemplate :exec
-- A template pulled from Meta. Matches an existing row by (name, language); Meta is the truth.
INSERT INTO message_templates (id, workspace_id, account_id, name, language, category, status, meta_template_id,
                               rejected_reason, components)
VALUES (@id, @workspace_id,
        (SELECT a.id FROM whatsapp_accounts a
         WHERE a.workspace_id = @workspace_id AND a.status = 'active'
         ORDER BY a.created_at DESC, a.id DESC LIMIT 1),
        @name, @language, @category, @status, @meta_template_id, @rejected_reason, @components)
ON CONFLICT ON CONSTRAINT message_templates_name_language_key DO UPDATE
SET account_id = EXCLUDED.account_id, category = EXCLUDED.category, status = EXCLUDED.status,
    meta_template_id = EXCLUDED.meta_template_id, rejected_reason = EXCLUDED.rejected_reason,
    components = EXCLUDED.components;

-- name: SetTemplateStatusByMetaID :execrows
-- From Meta's status webhook. A reason is kept only for a rejection.
UPDATE message_templates
SET status = @status,
    rejected_reason = CASE WHEN @status::text = 'rejected' THEN sqlc.narg(rejected_reason)::text END
WHERE workspace_id = @workspace_id AND meta_template_id = @meta_template_id;
