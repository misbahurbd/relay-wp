-- name: GetUserForLogin :one
-- The only query besides GetPasswordHash that reads password_hash.
SELECT id, email_verified_at, password_hash, status
FROM users
WHERE email = @email;

-- name: CountPasswordFailures :one
-- Wrong passwords for one email (metadata.key) since @since and since its last
-- successful sign-in: from this IP, and from anywhere.
SELECT
    count(*) FILTER (WHERE e.ip = sqlc.narg(client_ip)::inet)::int AS from_ip,
    count(*)::int AS total
FROM auth_events e
WHERE e.metadata ? 'key'
  AND e.metadata ->> 'key' = @key::text
  AND e.event = 'login_failed'
  AND e.created_at > GREATEST(@since::timestamptz, COALESCE((
      SELECT max(s.created_at) FROM auth_events s
      WHERE s.metadata ? 'key' AND s.metadata ->> 'key' = @key::text AND s.event = 'login_succeeded'
  ), '-infinity'::timestamptz));
