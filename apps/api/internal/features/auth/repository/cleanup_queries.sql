-- Cleanup (CleanupWorker, hourly). Ended sessions are kept 30 days for the
-- sessions page history; their refresh tokens go with them (ON DELETE CASCADE).

-- name: DeleteExpiredOAuthFlows :execrows
DELETE FROM oauth_flows
WHERE expires_at < now();

-- name: DeleteExpiredEmailTokens :execrows
DELETE FROM email_tokens
WHERE expires_at < now();

-- name: DeleteExpiredEmailCodes :execrows
DELETE FROM email_codes
WHERE expires_at < now();

-- name: DeleteExpiredRefreshTokens :execrows
DELETE FROM refresh_tokens
WHERE expires_at < now();

-- name: DeleteEndedSessions :execrows
DELETE FROM auth_sessions
WHERE revoked_at < now() - interval '30 days'
   OR expires_at < now() - interval '30 days';

-- name: DeleteOldAuthEvents :execrows
-- auth_events is kept 180 days (00002_auth.sql).
DELETE FROM auth_events
WHERE created_at < now() - interval '180 days';
