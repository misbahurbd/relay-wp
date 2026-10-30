-- ---------- Imports ----------

-- name: CreateContactImport :exec
INSERT INTO contact_imports (id, workspace_id, created_by, file_name, file_type, file_key, row_count, columns)
VALUES (@id, @workspace_id, @created_by, @file_name, @file_type, @file_key, @row_count, @columns);

-- name: GetContactImport :one
SELECT i.id, i.file_name, i.file_type, i.file_key, i.row_count, i.columns, i.mapping, i.tag_name, i.status,
       i.created_count, i.updated_count, i.skipped_count, i.skipped_key, i.error, i.created_at
FROM contact_imports i
WHERE i.workspace_id = @workspace_id AND i.id = @id;

-- name: SetContactImportMapping :execrows
-- Mapping can change until the import starts.
UPDATE contact_imports
SET mapping = @mapping, tag_name = sqlc.narg(tag_name), status = 'mapped'
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('uploaded', 'mapped');

-- name: StartContactImport :execrows
UPDATE contact_imports
SET status = 'importing', started_at = now()
WHERE workspace_id = @workspace_id AND id = @id AND status = 'mapped';

-- name: ResetContactImportProgress :exec
-- A (re)started job counts from zero.
UPDATE contact_imports
SET created_count = 0, updated_count = 0, skipped_count = 0, error = NULL
WHERE workspace_id = @workspace_id AND id = @id;

-- name: AddContactImportProgress :exec
UPDATE contact_imports
SET created_count = created_count + @created, updated_count = updated_count + @updated
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FinishContactImport :exec
UPDATE contact_imports
SET status = @status, skipped_count = @skipped, skipped_key = sqlc.narg(skipped_key),
    error = sqlc.narg(error), finished_at = now()
WHERE workspace_id = @workspace_id AND id = @id;
