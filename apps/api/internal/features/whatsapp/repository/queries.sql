-- name: GetWhatsAppAccountByWABA :one
SELECT id, workspace_id, status
FROM whatsapp_accounts
WHERE waba_id = @waba_id;

-- name: GetActiveAccountID :one
-- The workspace's WABA that template calls use (one active account per workspace today).
SELECT id
FROM whatsapp_accounts
WHERE workspace_id = @workspace_id AND status = 'active'
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: UpsertWhatsAppAccount :one
-- Reconnecting a WABA replaces its token. The caller has checked it belongs to this workspace.
INSERT INTO whatsapp_accounts (id, workspace_id, waba_id, business_id, name, access_token, connected_by)
VALUES (@id, @workspace_id, @waba_id, @business_id, @name, @access_token, @connected_by)
ON CONFLICT (waba_id) DO UPDATE
SET business_id = EXCLUDED.business_id, name = EXCLUDED.name, access_token = EXCLUDED.access_token,
    connected_by = EXCLUDED.connected_by, status = 'active'
WHERE whatsapp_accounts.workspace_id = EXCLUDED.workspace_id
RETURNING id;

-- name: GetAccountToken :one
-- The only query that reads access_token, and only for a Graph call.
SELECT waba_id, access_token
FROM whatsapp_accounts
WHERE workspace_id = @workspace_id AND id = @id AND status = 'active';

-- name: DisconnectWhatsAppAccount :exec
UPDATE whatsapp_accounts
SET status = 'disconnected'
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DisconnectAccountNumbers :exec
UPDATE whatsapp_numbers
SET status = 'disconnected'
WHERE workspace_id = @workspace_id AND account_id = @account_id;

-- name: GetNumberWorkspace :one
SELECT workspace_id
FROM whatsapp_numbers
WHERE phone_number_id = @phone_number_id;

-- name: UpsertWhatsAppNumber :one
INSERT INTO whatsapp_numbers (id, workspace_id, account_id, phone_number_id, display_phone_number,
                              number_type, verified_name, name_status, quality_rating, messaging_limit,
                              status, setup_tasks, last_error)
VALUES (@id, @workspace_id, @account_id, @phone_number_id, @display_phone_number, @number_type,
        @verified_name, @name_status, @quality_rating, @messaging_limit, 'setting_up', @setup_tasks, NULL)
ON CONFLICT (phone_number_id) DO UPDATE
SET account_id = EXCLUDED.account_id, display_phone_number = EXCLUDED.display_phone_number,
    number_type = EXCLUDED.number_type, verified_name = EXCLUDED.verified_name,
    name_status = EXCLUDED.name_status, quality_rating = EXCLUDED.quality_rating,
    messaging_limit = EXCLUDED.messaging_limit, status = 'setting_up',
    setup_tasks = EXCLUDED.setup_tasks, last_error = NULL
WHERE whatsapp_numbers.workspace_id = EXCLUDED.workspace_id
RETURNING id;

-- name: ListWhatsAppNumbers :many
SELECT id, account_id, phone_number_id, display_phone_number, number_type, verified_name, name_status,
       quality_rating, messaging_limit, status, setup_tasks, last_error, created_at
FROM whatsapp_numbers
WHERE workspace_id = @workspace_id
ORDER BY created_at;

-- name: GetWhatsAppNumber :one
SELECT id, account_id, phone_number_id, display_phone_number, number_type, verified_name, name_status,
       quality_rating, messaging_limit, status, setup_tasks, last_error, created_at
FROM whatsapp_numbers
WHERE workspace_id = @workspace_id AND id = @id;

-- name: HasConnectedNumber :one
SELECT EXISTS (
    SELECT 1 FROM whatsapp_numbers WHERE workspace_id = @workspace_id AND status = 'connected'
);

-- name: SetNumberTask :exec
UPDATE whatsapp_numbers
SET setup_tasks = jsonb_set(setup_tasks, ARRAY[@task::text], to_jsonb(@state::text))
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetNumberStatus :exec
UPDATE whatsapp_numbers
SET status = @status, last_error = @last_error
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetNumberPin :exec
UPDATE whatsapp_numbers
SET registration_pin = @registration_pin
WHERE workspace_id = @workspace_id AND id = @id;

-- name: GetNumberPin :one
-- The only query that reads registration_pin.
SELECT registration_pin
FROM whatsapp_numbers
WHERE workspace_id = @workspace_id AND id = @id;

-- name: UpdateNumberProfile :exec
UPDATE whatsapp_numbers
SET display_phone_number = @display_phone_number, verified_name = @verified_name,
    name_status = @name_status, quality_rating = @quality_rating, messaging_limit = @messaging_limit
WHERE workspace_id = @workspace_id AND id = @id;

-- name: UpdateNumberNameStatus :exec
UPDATE whatsapp_numbers
SET name_status = @name_status
WHERE workspace_id = @workspace_id AND account_id = @account_id
  AND regexp_replace(display_phone_number, '[^0-9]', '', 'g') = @digits;

-- name: UpdateNumberQuality :exec
UPDATE whatsapp_numbers
SET quality_rating = COALESCE(sqlc.narg(quality_rating), quality_rating),
    messaging_limit = COALESCE(sqlc.narg(messaging_limit), messaging_limit)
WHERE workspace_id = @workspace_id AND account_id = @account_id
  AND regexp_replace(display_phone_number, '[^0-9]', '', 'g') = @digits;

-- name: CountRecentTestMessages :one
SELECT count(*)
FROM whatsapp_test_messages
WHERE workspace_id = @workspace_id AND created_at > now() - interval '1 hour';

-- name: CreateTestMessage :exec
INSERT INTO whatsapp_test_messages (id, workspace_id, number_id, to_phone)
VALUES (@id, @workspace_id, @number_id, @to_phone);

-- name: SetTestMessageSent :exec
UPDATE whatsapp_test_messages
SET wamid = @wamid, status = 'sent'
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetTestMessageFailed :exec
UPDATE whatsapp_test_messages
SET status = 'failed', error_code = @error_code
WHERE workspace_id = @workspace_id AND id = @id;

-- name: GetTestMessage :one
SELECT id, number_id, to_phone, status, error_code, created_at
FROM whatsapp_test_messages
WHERE workspace_id = @workspace_id AND id = @id;

-- name: UpdateTestMessageStatus :one
-- Statuses only move forward: a late "sent" never overwrites "delivered".
UPDATE whatsapp_test_messages
SET status = @status, error_code = sqlc.narg(error_code)
WHERE workspace_id = @workspace_id AND wamid = @wamid
  AND array_position(ARRAY['queued', 'sent', 'delivered', 'read', 'failed'], status)
      < array_position(ARRAY['queued', 'sent', 'delivered', 'read', 'failed'], @status::text)
RETURNING id, status;
