-- name: CreateSocialUser :exec
-- A user created by social sign-in: no password; verified only if the provider vouched.
INSERT INTO users (id, email, email_verified_at)
VALUES (@id, @email, sqlc.narg(email_verified_at));

-- name: GetSessionAuthenticatedAt :one
-- When this session last proved who it is (password or social sign-in), for sensitive actions.
SELECT authenticated_at
FROM auth_sessions
WHERE id = @id AND user_id = @user_id AND revoked_at IS NULL;

-- name: GetIdentity :one
SELECT id, user_id
FROM user_identities
WHERE provider = @provider AND provider_user_id = @provider_user_id;

-- name: CreateIdentity :exec
INSERT INTO user_identities (id, user_id, provider, provider_user_id, email, email_verified)
VALUES (@id, @user_id, @provider, @provider_user_id, @email, @email_verified);

-- name: TouchIdentity :exec
-- Refresh what the provider reports on every sign-in. Never touches the profile.
UPDATE user_identities
SET email = @email, email_verified = @email_verified, last_used_at = now()
WHERE id = @id;

-- name: ListIdentities :many
SELECT provider, email, last_used_at, created_at
FROM user_identities
WHERE user_id = @user_id
ORDER BY created_at;

-- name: CountIdentities :one
SELECT count(*)
FROM user_identities
WHERE user_id = @user_id;

-- name: DeleteIdentity :execrows
DELETE FROM user_identities
WHERE user_id = @user_id AND provider = @provider;

-- name: DeleteIdentityByProviderUser :one
-- Meta's data-deletion callback: identifies the Facebook user only.
DELETE FROM user_identities
WHERE provider = @provider AND provider_user_id = @provider_user_id
RETURNING user_id;

-- name: DeleteUnverifiedIdentities :execrows
-- A password reset proves who owns the email: identities whose provider never
-- vouched for that email (e.g. Facebook) can't keep access to the account.
DELETE FROM user_identities
WHERE user_id = @user_id AND email_verified = false;

-- name: CreateOAuthFlow :exec
INSERT INTO oauth_flows (
    id, state_hash, provider, intent, link_user_id, code_verifier, nonce,
    redirect_to, client, timezone, app_code_challenge, expires_at
) VALUES (
    @id, @state_hash, @provider, @intent, sqlc.narg(link_user_id), @code_verifier, @nonce,
    @redirect_to, @client, @timezone, sqlc.narg(app_code_challenge), @expires_at
);

-- name: ConsumeOAuthFlow :one
-- Single use: the row is deleted as it is read. Secrets are needed here to finish the exchange.
DELETE FROM oauth_flows
WHERE state_hash = @state_hash AND provider = @provider AND status = 'started' AND expires_at > now()
RETURNING intent, link_user_id, code_verifier, nonce, redirect_to, client, timezone, app_code_challenge;

-- name: CreatePendingOAuthFlow :exec
-- Second stage of a flow: 'awaiting_email' (Facebook gave no email) or 'ready'
-- (a mobile sign-in waiting for the app to redeem its one-time code).
INSERT INTO oauth_flows (
    id, state_hash, provider, intent, code_verifier, nonce, redirect_to, client, timezone,
    app_code_challenge, status, pending_identity, completion_token_hash, user_id, expires_at
) VALUES (
    @id, @state_hash, @provider, 'login', '', '', @redirect_to, @client, @timezone,
    sqlc.narg(app_code_challenge), @status, sqlc.narg(pending_identity), @completion_token_hash,
    sqlc.narg(user_id), @expires_at
);

-- name: ConsumePendingOAuthFlow :one
DELETE FROM oauth_flows
WHERE completion_token_hash = @completion_token_hash AND status = @status AND expires_at > now()
RETURNING provider, client, timezone, app_code_challenge, pending_identity, user_id;

-- name: GetAwaitingOAuthFlow :one
-- The "add your email" step of a social sign-up. Locks the row while a code is sent.
SELECT id, client, app_code_challenge, email_code_sends
FROM oauth_flows
WHERE completion_token_hash = @completion_token_hash AND status = 'awaiting_email' AND expires_at > now()
FOR UPDATE;

-- name: SetOAuthFlowEmailCode :exec
UPDATE oauth_flows
SET pending_email = @pending_email, email_code_hash = @email_code_hash, email_code_expires_at = @email_code_expires_at,
    email_code_attempts = 0, email_code_sends = email_code_sends + 1
WHERE id = @id;

-- name: GetOAuthFlowEmailCode :one
-- The only query that reads oauth_flows.email_code_hash. Locks the row so tries count correctly.
SELECT id, provider, client, timezone, app_code_challenge, pending_identity,
       pending_email, email_code_hash, email_code_expires_at, email_code_attempts
FROM oauth_flows
WHERE completion_token_hash = @completion_token_hash AND status = 'awaiting_email' AND expires_at > now()
FOR UPDATE;

-- name: RecordOAuthFlowCodeAttempt :one
UPDATE oauth_flows
SET email_code_attempts = email_code_attempts + 1
WHERE id = @id
RETURNING email_code_attempts;

-- name: DeleteOAuthFlow :exec
DELETE FROM oauth_flows
WHERE id = @id;
