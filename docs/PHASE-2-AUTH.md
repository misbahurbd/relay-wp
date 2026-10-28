# Phase 2 plan: authentication and profiles

Status: **PR 1 merged** (steps 1–7: password auth, sessions, events, profiles). **PR 2 built** (steps 8, 9 and 11: OAuth framework, Google, Facebook with the add-email step and the data-deletion callback, linking and unlinking, `password/set`). Still to do: Apple (step 10), the `auth_cleanup` job and remaining docs (steps 12–13). Follows the rules in [CLAUDE.md](../CLAUDE.md) and the architecture in [ARCHITECTURE.md](ARCHITECTURE.md).

---

## 1. Decisions

| Topic                     | Decision                                                                                                                                                                     |
| ------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Sign-in methods           | Email + password, plus Google, Facebook and Apple                                                                                                                            |
| Email verification        | Required before a **password** user can log in. Social logins are marked verified only when the provider vouches for the email.                                              |
| Password rules            | 8–128 characters; at least one lowercase, one uppercase, one digit and one symbol; rejected if found in known breaches (Have I Been Pwned, k-anonymity, password never sent) |
| Password hashing          | argon2id (m=19 MiB, t=2, p=1); parameters stored in the hash so they can be raised later                                                                                     |
| Access token              | JWT, Ed25519, **15 minutes**                                                                                                                                                 |
| Refresh token             | Opaque, **30 days, sliding**; rotated on every use; reuse revokes the session                                                                                                |
| Token transport           | httpOnly cookies for the web app, response body + `Authorization: Bearer` for mobile and API clients                                                                         |
| Signups                   | Open to anyone                                                                                                                                                               |
| Audit log                 | `auth_events`, kept **180 days**                                                                                                                                             |
| Profile vs auth           | `users` holds auth only; `user_profiles` and `user_preferences` hold what the frontend shows                                                                                 |
| Password storage          | Nullable `users.password_hash` (NULL = social-only user)                                                                                                                     |
| Avatar                    | Copied from the provider **once, at signup**; NULL for password signups; custom upload later                                                                                 |
| Display name              | Optional override; when empty, the UI shows "first last"                                                                                                                     |
| Facebook without email    | Ask the user for an email after sign-in, then verify it                                                                                                                      |
| Per-workspace preferences | Not now. If needed later: a separate `workspace_member_preferences` table, leaving `user_preferences` per user                                                               |

---

## 2. Database: 9 tables

All IDs are UUIDv7. All times are `timestamptz` in UTC. Tables with `updated_at` use the `set_updated_at()` trigger from `00001_init.sql`.

### Owned by `features/auth`

**`users`**: one per person, auth fields only.

| Column                     | Type                             | Notes                                                          |
| -------------------------- | -------------------------------- | -------------------------------------------------------------- |
| `id`                       | uuid, PK                         |                                                                |
| `email`                    | citext, NOT NULL, unique         | Login lookup                                                   |
| `email_verified_at`        | timestamptz, nullable            |                                                                |
| `password_hash`            | text, nullable                   | **Secret.** Selected only by login and change-password queries |
| `status`                   | text, NOT NULL, default `active` | `active` or `disabled` (CHECK constraint)                      |
| `last_login_at`            | timestamptz, nullable            |                                                                |
| `created_at`, `updated_at` | timestamptz                      |                                                                |

**`user_identities`**: one per linked social login.

| Column                     | Type                                | Notes                                                                                                                                                                                            |
| -------------------------- | ----------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `id`                       | uuid, PK                            |                                                                                                                                                                                                  |
| `user_id`                  | uuid, FK → users, ON DELETE CASCADE | Indexed                                                                                                                                                                                          |
| `provider`                 | text                                | `google`, `facebook`, `apple` (CHECK)                                                                                                                                                            |
| `provider_user_id`         | text                                | Google/Apple `sub`, Facebook user ID                                                                                                                                                             |
| `email`                    | citext, nullable                    | What the provider reported                                                                                                                                                                       |
| `email_verified`           | bool                                | What the provider reported (always false for Facebook)                                                                                                                                           |
| `provider_refresh_token`   | bytea, nullable                     | **Apple only. Secret.** Apple's refresh token, encrypted with AES-256-GCM (`TOKEN_ENCRYPTION_KEY`). Apple requires revoking it when the account is deleted, and it can't be fetched again later. |
| `last_used_at`             | timestamptz                         |                                                                                                                                                                                                  |
| `created_at`, `updated_at` | timestamptz                         |                                                                                                                                                                                                  |

Unique `(provider, provider_user_id)`. Unique `(user_id, provider)`: one account per provider per user.

**`email_tokens`**: verification and password-reset links.

| Column       | Type                      | Notes                            |
| ------------ | ------------------------- | -------------------------------- |
| `id`         | uuid, PK                  |                                  |
| `user_id`    | uuid, FK → users, CASCADE |                                  |
| `purpose`    | text                      | `verify` (24 h) or `reset` (1 h) |
| `email`      | citext                    | Address the link was sent to     |
| `token_hash` | bytea, unique             | **Secret.** SHA-256 of the token |
| `expires_at` | timestamptz               |                                  |
| `used_at`    | timestamptz, nullable     | Single use                       |
| `created_at` | timestamptz               |                                  |

Index `(user_id, purpose) WHERE used_at IS NULL`. A new link invalidates the user's earlier unused links of the same purpose.

**`auth_sessions`**: one per signed-in device. Its `id` is the JWT `sid`.

| Column             | Type                      | Notes                                                                                                                                                           |
| ------------------ | ------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `id`               | uuid, PK                  |                                                                                                                                                                 |
| `user_id`          | uuid, FK → users, CASCADE |                                                                                                                                                                 |
| `provider`         | text                      | How this session signed in: `password`, `google`, `facebook`, `apple`                                                                                           |
| `client`           | text                      | `web` or `mobile`; decides cookies vs JSON body on refresh, and how the device is shown                                                                         |
| `user_agent`       | text                      | Raw value, kept for security investigations                                                                                                                     |
| `device_name`      | text                      | Shown to the user, e.g. "Chrome on macOS" or "Relay app on iPhone 15". Set at sign-in.                                                                          |
| `device_type`      | text                      | `desktop`, `mobile`, `tablet` or `app` (CHECK); picks the icon                                                                                                  |
| `created_ip`       | inet                      | Where the session started                                                                                                                                       |
| `last_ip`          | inet                      | Updated on refresh; "last seen from" on the devices page                                                                                                        |
| `last_used_at`     | timestamptz               | Updated on refresh                                                                                                                                              |
| `authenticated_at` | timestamptz               | Last time the user actually entered a password or completed social login. Refresh does **not** change it. Used for "confirm it's you" before sensitive actions. |
| `expires_at`       | timestamptz               | Now + 30 days, extended on each refresh                                                                                                                         |
| `revoked_at`       | timestamptz, nullable     |                                                                                                                                                                 |
| `revoke_reason`    | text, nullable            | `logout`, `logout_all`, `password_changed`, `password_reset`, `reuse_detected`, `user_disabled`, `revoked_by_user`                                              |
| `created_at`       | timestamptz               |                                                                                                                                                                 |

Index `(user_id) WHERE revoked_at IS NULL`.

**`refresh_tokens`**: one per refresh token issued.

| Column       | Type                              | Notes               |
| ------------ | --------------------------------- | ------------------- |
| `id`         | uuid, PK                          |                     |
| `session_id` | uuid, FK → auth_sessions, CASCADE |                     |
| `token_hash` | bytea, unique                     | **Secret.** SHA-256 |
| `expires_at` | timestamptz                       |                     |
| `used_at`    | timestamptz, nullable             | Set when rotated    |
| `created_at` | timestamptz                       |                     |

**`oauth_flows`**: one per social login or link attempt; deleted when used.

| Column                  | Type                                | Notes                                                    |
| ----------------------- | ----------------------------------- | -------------------------------------------------------- |
| `id`                    | uuid, PK                            |                                                          |
| `state_hash`            | bytea, unique                       | **Secret.** CSRF protection                              |
| `provider`              | text                                |                                                          |
| `intent`                | text                                | `login` or `link` (connecting a provider from settings)  |
| `link_user_id`          | uuid, nullable, FK → users, CASCADE | Set when `intent = link`                                 |
| `code_verifier`         | text                                | PKCE                                                     |
| `nonce`                 | text                                | OIDC nonce (Google, Apple)                               |
| `redirect_to`           | text                                | Checked against `AUTH_ALLOWED_REDIRECTS`                 |
| `client`                | text                                | `web` or `mobile`                                        |
| `status`                | text                                | `started` or `awaiting_email` (Facebook without email)   |
| `pending_identity`      | jsonb, nullable                     | Provider ID, names and avatar while waiting for an email |
| `completion_token_hash` | bytea, nullable, unique             | **Secret.** For the "add your email" step                |
| `expires_at`            | timestamptz                         | 10 minutes; 30 minutes once `awaiting_email`             |
| `created_at`            | timestamptz                         |                                                          |

**`auth_events`**: append-only security log, kept 180 days.

| Column       | Type                                           | Notes                                               |
| ------------ | ---------------------------------------------- | --------------------------------------------------- |
| `id`         | uuid, PK                                       |                                                     |
| `user_id`    | uuid, nullable, FK → users, ON DELETE SET NULL | NULL for unknown emails                             |
| `event`      | text                                           | See list below                                      |
| `provider`   | text, nullable                                 |                                                     |
| `ip`         | inet                                           |                                                     |
| `user_agent` | text                                           |                                                     |
| `metadata`   | jsonb                                          | e.g. failure reason. **Never passwords or tokens.** |
| `created_at` | timestamptz                                    |                                                     |

Index `(user_id, created_at DESC)`, and `(created_at)` for the retention job.

Events: `signup`, `login_succeeded`, `login_failed`, `logout`, `logout_all`, `email_verified`, `verification_sent`, `password_changed`, `password_set`, `password_reset_requested`, `password_reset_completed`, `identity_linked`, `identity_unlinked`, `session_revoked`, `refresh_reuse_detected`.

### Owned by `features/profiles`

**`user_profiles`**: one per user (`user_id` is PK and FK, CASCADE).

| Column                     | Type                 | Notes                                         |
| -------------------------- | -------------------- | --------------------------------------------- |
| `user_id`                  | uuid, PK, FK → users |                                               |
| `first_name`               | text, NOT NULL       |                                               |
| `last_name`                | text, nullable       | Single-name people; GitHub-style single names |
| `display_name`             | text, nullable       | Optional override                             |
| `avatar_url`               | text, nullable       | Provider URL at signup; custom upload later   |
| `created_at`, `updated_at` | timestamptz          |                                               |

**`user_preferences`**: one per user (`user_id` is PK and FK, CASCADE).

| Column                     | Type                          | Notes                                            |
| -------------------------- | ----------------------------- | ------------------------------------------------ |
| `user_id`                  | uuid, PK, FK → users          |                                                  |
| `locale`                   | text, NOT NULL, default `en`  |                                                  |
| `timezone`                 | text, NOT NULL, default `UTC` | IANA name, validated; browser sends it at signup |
| `created_at`, `updated_at` | timestamptz                   |                                                  |

### Migrations

| File                     | Contents                                                                                                    |
| ------------------------ | ----------------------------------------------------------------------------------------------------------- |
| `00002_auth.sql`         | `citext` extension, the 7 auth tables, indexes, CHECKs, triggers                                            |
| `00003_profiles.sql`     | `user_profiles`, `user_preferences`                                                                         |
| `00004_social_login.sql` | `oauth_flows`: `timezone`, `app_code_challenge`, `user_id`, status `ready`; revoke reason `account_claimed` |

Both have working `Down` sections; CI already runs up → down → up.

---

## 3. API endpoints

All under `/v1`. Errors use the standard `{code, message, details}` shape. "Auth" means a valid access token (cookie or Bearer).

### Password accounts

| Method and path                  | Auth                 | What it does                                                                                                                                                                                                                                                   |
| -------------------------------- | -------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST /auth/signup`              | –                    | `email`, `password`, `first_name`, `last_name?`, `timezone?`. Creates user + profile + preferences, queues a verification email. **Same 202 response whether or not the email exists**; an existing owner gets an "you already have an account" email instead. |
| `POST /auth/verify-email`        | –                    | `token`. Sets `email_verified_at`. Does not log in.                                                                                                                                                                                                            |
| `POST /auth/verify-email/resend` | –                    | `email`. Always 202. Rate-limited.                                                                                                                                                                                                                             |
| `POST /auth/login`               | –                    | `email`, `password`. Unverified with correct password → `403 email_not_verified`. Wrong email or password → `401 invalid_credentials` (same response and timing either way).                                                                                   |
| `POST /auth/password/forgot`     | –                    | `email`. Always 202; sends a reset link if the account exists.                                                                                                                                                                                                 |
| `POST /auth/password/reset`      | –                    | `token`, `new_password`. Sets the password, revokes **all** sessions.                                                                                                                                                                                          |
| `POST /auth/password/change`     | Auth                 | `current_password`, `new_password`. Revokes all other sessions.                                                                                                                                                                                                |
| `POST /auth/password/set`        | Auth, recent sign-in | `new_password`, for social-only users adding a password. Requires a verified email.                                                                                                                                                                            |

### Sessions and tokens

| Method and path              | Auth                           | What it does                                                                                                                                                                             |
| ---------------------------- | ------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST /auth/token/refresh`   | Refresh token (cookie or body) | Rotates the refresh token, returns a new access token. Reused token → revoke session, `401`.                                                                                             |
| `POST /auth/logout`          | Auth                           | Revokes the current session, clears cookies.                                                                                                                                             |
| `POST /auth/logout-all`      | Auth                           | Revokes every session of the user.                                                                                                                                                       |
| `GET /auth/sessions`         | Auth                           | Active sessions with `device_name`, `device_type`, sign-in method, `last_used_at`, `last_ip`, and `current: true` for this device. Sorted: this device first, then most recently active. |
| `DELETE /auth/sessions/{id}` | Auth                           | Revoke one of your own sessions.                                                                                                                                                         |
| `GET /auth/events`           | Auth                           | Your recent security activity (last 50).                                                                                                                                                 |

### Social login

| Method and path                                  | Auth                          | What it does                                                                                                                                                                             |
| ------------------------------------------------ | ----------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /auth/providers`                            | –                             | Which providers this server offers (only configured ones), so the frontend knows which buttons to show.                                                                                  |
| `GET /auth/oauth/{provider}/start`               | –                             | `client`, `redirect_to`, `timezone`, and `code_challenge` (mobile). Creates an `oauth_flows` row, sets the `relay_oauth` state cookie, redirects to the provider.                        |
| `GET /auth/oauth/{provider}/link`                | Auth, recent sign-in          | Same as start with `intent = link`, for connecting a provider from settings (web).                                                                                                       |
| `GET` and `POST /auth/oauth/{provider}/callback` | –                             | Apple uses form POST. Validates state, PKCE, nonce and ID token; signs in or links (see section 4).                                                                                      |
| `POST /auth/oauth/complete`                      | Completion token              | Facebook without email: `email` → creates the account unverified, sends a verification link, signs in. Mobile also sends `code_verifier`.                                                |
| `POST /auth/oauth/token`                         | One-time code + PKCE verifier | Mobile: redeems the `code` the callback sent to the app's deep link for tokens. A wrong verifier spends the code.                                                                        |
| `POST /auth/oauth/facebook/data-deletion`        | Signed request from Meta      | **Required by Meta before go-live.** Verifies Meta's `signed_request`, removes that Facebook identity, logs `identity_unlinked`, and returns the confirmation URL and code Meta expects. |
| `GET /auth/identities`                           | Auth                          | Connected providers, and whether a password is set.                                                                                                                                      |
| `DELETE /auth/identities/{provider}`             | Auth, recent sign-in          | Disconnect a provider. **Refused if it's the last sign-in method.**                                                                                                                      |

### Profile and preferences

| Method and path         | Auth | What it does                                                                                |
| ----------------------- | ---- | ------------------------------------------------------------------------------------------- |
| `GET /me`               | Auth | Profile + preferences + `email` + `email_verified` + `has_password`. Never `password_hash`. |
| `PATCH /me/profile`     | Auth | `first_name`, `last_name`, `display_name`                                                   |
| `PATCH /me/preferences` | Auth | `locale`, `timezone`                                                                        |

Changing email is out of scope for this phase (it needs its own verify-the-new-address flow).

---

## 4. Flows

### Social sign-in (callback)

1. The `state` must equal the `relay_oauth` cookie set at start (HttpOnly, SameSite=Lax, path `/v1/auth/oauth`, 10 minutes). This binds the attempt to the browser that began it, so a callback link sent by an attacker can't sign a victim into the attacker's account. Load the `oauth_flows` row by `state_hash`; reject if missing or expired; **delete it**.
2. Exchange the code with PKCE; verify the ID token (signature, issuer, audience, expiry, `nonce`). Facebook has no ID token in this flow; fetch `/me?fields=id,email,first_name,last_name,picture`.
3. Normalise to `{provider, provider_user_id, email?, email_verified, first_name?, last_name?, avatar_url?}`. Facebook: `email_verified = false`. Apple: names come only on the first authorisation; keep them for step 4.
4. In one transaction:
   - **Identity exists** → that user. Refresh `user_identities.email` and `last_used_at`. Never overwrite the profile or avatar.
   - **`intent = link`** → attach the identity to `link_user_id`. Refuse if the provider account already belongs to another user.
   - **No identity, provider-verified email matches a user**:
     - existing user's email verified → link the identity;
     - existing user's email **not** verified → this is the real owner: mark verified, **clear `password_hash`**, revoke that user's sessions, link.
   - **No identity, no verified match, email present** → create user (+ verified if the provider vouched), identity, profile (names + avatar), preferences.
   - **No email (Facebook)** → keep the flow as `awaiting_email` with `pending_identity`; return a completion token; the frontend asks for an email (`POST /auth/oauth/complete`).
   - **Email matches an existing user but isn't provider-verified** → never link. Respond `409 email_in_use_sign_in_first` so the user signs in and links from settings.
5. Refuse if the user is `disabled`. **Web:** create the session, set the cookies and redirect to `redirect_to`. **Mobile:** the callback runs in the system browser, which can't hand tokens to the app. It stores a `ready` flow holding the user ID and redirects to the app's deep link with a one-time `code` (2 minutes). The app redeems it at `POST /auth/oauth/token` with the PKCE verifier matching the `code_challenge` it sent to start; another app that intercepts the deep link can't use the code. The session is created at redemption, so it gets the app's `X-Device-Name`. Failures redirect to `redirect_to?error=<code>`. To create the session: (`client` from the flow, `device_name` and `device_type` as described under "Device names", `created_ip` = `last_ip` = request IP, `authenticated_at` = now) and its refresh token. For Apple, store the encrypted Apple refresh token on the identity. Write `auth_events`, return tokens.

### Refresh

1. Hash the presented token; load it with its session.
2. Already `used_at` → **reuse**: revoke the session (`reuse_detected`), log `refresh_reuse_detected`, `401`.
3. Expired, session revoked or user disabled → `401`.
4. Mark it used, insert a new token (expires now + 30 days), extend `auth_sessions.expires_at`, update `last_used_at` and `last_ip` (never `authenticated_at`); issue a new 15-minute access token, as cookies or JSON depending on the session's `client`.

Concurrent refreshes from two browser tabs: the frontend serialises refresh calls, so a second tab never presents an already-rotated token.

### Device names (every new session, password or social)

- **Mobile app**: sends `X-Device-Name` (e.g. "iPhone 15"). Stored as "Relay app on iPhone 15", `device_type = app`.
- **Web**: parsed from the user agent into "Browser on OS" (e.g. "Chrome on macOS", "Safari on iPhone"), with `device_type` `desktop`, `mobile` or `tablet`.
- **Fallback**: "Unknown device", `device_type = desktop`.
- `X-Device-Name` comes from the client, so it is trimmed to 100 characters, stripped of control characters, and always rendered as plain text.
- Set once at sign-in; refresh does not change it.

### Access token check (every authenticated request)

Verify signature (`kid` → key), `exp`, `iss`, `aud`. Then one primary-key lookup (`IsSessionActive`): the session must not be revoked or expired and its user must be `active`, otherwise `401 unauthenticated` (`503` if the lookup itself fails). Put `user_id` and `session_id` into the request context (`shared/authctx`). Revoking a session, logging out, a password reset or refresh-token reuse therefore takes effect on the very next request, not when the 15-minute token expires. `auth.Accounts` implements the check; `internal/app` passes it to `RequireAuth`.

---

## 5. Security rules

- **No account enumeration**: signup, resend, forgot-password always answer 202; login errors don't reveal which part was wrong; responses take similar time (dummy argon2 verify for unknown emails).
- **Rate limits** (per IP, in addition to the global limit): login 10/min, signup 5/hour, forgot/resend 5/hour; plus per email: 5 failed logins per 15 minutes → `429` for that email.
- **Redirects**: `redirect_to` must match `AUTH_ALLOWED_REDIRECTS` exactly; otherwise the default.
- **Cookies**: `access_token` (Path=/, 15 min) and `refresh_token` (Path=/v1/auth/token/refresh, 30 days); both `HttpOnly`, `Secure`, `SameSite=Lax`.
- **CSRF** for cookie requests that change data: `Origin` must be in `CORS_ALLOWED_ORIGINS`. Bearer requests are exempt.
- **Secrets never leave their query** (CLAUDE.md): password and token hashes are never in models, DTOs, responses or logs. No `SELECT *`.
- **Password reset and change** revoke sessions; reset links are single-use.
- **Disabled users** can't log in, refresh or complete OAuth.
- **Password reset on an unverified account** removes identities whose provider didn't vouch for the email (Facebook). The reset link proves who owns the address; a Facebook login someone attached before then may not be theirs.
- **Connecting a provider** needs a recent sign-in, like disconnecting one, so a stolen session can't add the thief's Google account as a permanent way in.
- **Recent sign-in for sensitive actions**: setting a password and disconnecting a provider require `auth_sessions.authenticated_at` within `REAUTH_MAX_AGE` (10 minutes); otherwise `403 reauthentication_required` and the frontend asks the user to sign in again. Later: change email, delete account.
- **Encrypted, not hashed**: the Apple refresh token must be sent back to Apple, so it is encrypted (AES-256-GCM) rather than hashed. Like hashes, it is never in a model, DTO, response or log.

---

## 6. Code layout

```
apps/api/internal/
├── platform/jwt/            # Ed25519 sign/verify, key ring by kid (approved factory)
├── platform/middleware/
│   └── auth.go              # RequireAuth: Bearer first, then cookie; CSRF origin check; session must be active
├── shared/authctx/          # user_id, session_id in context; auth cookie names
├── features/auth/
│   ├── handler.go  service.go  repository.go  model.go  dto.go  errors.go  routes.go  jobs.go
│   ├── queries.sql
│   ├── password.go          # argon2id + combination rules
│   ├── breach.go            # Have I Been Pwned range check (interface; fake in tests)
│   ├── tokens.go            # refresh and email token generation/hashing
│   ├── cookies.go           # set/clear auth cookies
│   ├── device.go            # device_name / device_type from user agent or X-Device-Name
│   ├── oauth.go             # flow orchestration + linking rules
│   ├── provider.go          # Provider interface + registry
│   ├── provider_google.go  provider_facebook.go  provider_apple.go
│   └── *_test.go
└── features/profiles/
    ├── handler.go  service.go  repository.go  model.go  dto.go  routes.go
    ├── queries.sql
    └── *_test.go
```

- **auth → profiles**: auth declares `ProfileCreator` (create profile + preferences inside the signup transaction, taking the tx as `DBTX`); `internal/app` injects the profiles service. No cross-feature imports.
- **`/me`** lives in `profiles`; it reads `users.email` and `email_verified_at` through a small `AccountReader` interface that `profiles` declares and the auth service implements (never the hash).
- **Auth middleware** lives in `platform/middleware` (CLAUDE.md: middleware is infrastructure). It only verifies the JWT with `platform/jwt` and fills `shared/authctx`; it never imports `features/auth`.

### Factory check (CLAUDE.md section 2)

| Thing                               | Factory?                                                            | Why                                                                              |
| ----------------------------------- | ------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| `platform/jwt` key ring             | Yes (already approved)                                              | Key chosen by `kid`                                                              |
| OAuth provider registry             | **Yes, approved** ([ADR 0001](adr/0001-oauth-provider-registry.md)) | 3 implementations, chosen by the `{provider}` path value, built once at startup. |
| Breach checker                      | No                                                                  | One real implementation; tests pass a fake to `New`                              |
| Auth/profile services, repositories | No                                                                  | Plain constructors                                                               |

---

## 7. Background jobs (River)

| Job                   | When                   | What                                                                                                                                                |
| --------------------- | ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `send_email` (exists) | On demand              | Verification, reset, "already have an account"                                                                                                      |
| `auth_cleanup`        | Nightly (periodic job) | Delete expired/used `refresh_tokens`, `email_tokens`, expired `oauth_flows`, revoked sessions older than 30 days, `auth_events` older than 180 days |

Emails are enqueued in the same transaction as the change that triggers them.

---

## 8. Configuration (new environment variables)

| Variable                                                                | Example                      | Notes                                                                                 |
| ----------------------------------------------------------------------- | ---------------------------- | ------------------------------------------------------------------------------------- |
| `JWT_SIGNING_KEY`                                                       | base64 Ed25519 seed          | Current key                                                                           |
| `JWT_SIGNING_KEY_ID`                                                    | `2026-10`                    | `kid`                                                                                 |
| `JWT_PREVIOUS_PUBLIC_KEYS`                                              | `kid:base64,...`             | Still accepted during rotation                                                        |
| `JWT_ISSUER`, `JWT_AUDIENCE`                                            | `relay-api`, `relay`         |                                                                                       |
| `ACCESS_TOKEN_TTL`                                                      | `15m`                        |                                                                                       |
| `REFRESH_TOKEN_TTL`                                                     | `720h`                       | 30 days                                                                               |
| `APP_BASE_URL`                                                          | `http://localhost:3010`      | Links in emails                                                                       |
| `API_BASE_URL`                                                          | `http://localhost:8080`      | OAuth callback URLs                                                                   |
| `AUTH_ALLOWED_REDIRECTS`                                                | `relay://auth/callback`      | Extra exact URLs (mobile deep links). Any page of `APP_BASE_URL` is always allowed.   |
| `COOKIE_DOMAIN`, `COOKIE_SECURE`                                        | `localhost`, `false` locally |                                                                                       |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`                              |                              |                                                                                       |
| `FACEBOOK_APP_ID`, `FACEBOOK_APP_SECRET`                                |                              |                                                                                       |
| `APPLE_CLIENT_ID`, `APPLE_TEAM_ID`, `APPLE_KEY_ID`, `APPLE_PRIVATE_KEY` |                              | Client secret JWT generated from the `.p8` key at startup and refreshed before expiry |
| `PASSWORD_BREACH_CHECK`                                                 | `true`                       | Off in tests                                                                          |
| `TOKEN_ENCRYPTION_KEY`                                                  | base64, 32 bytes             | Encrypts stored provider tokens (Apple refresh token)                                 |
| `REAUTH_MAX_AGE`                                                        | `10m`                        | How recent `authenticated_at` must be for sensitive actions                           |

A provider with no credentials is simply not registered; its endpoints return `404`. Local development works with password auth alone.

---

## 9. Testing

- **Unit**: password rules (table-driven), argon2 hash/verify/rehash, token hashing, linking decisions (every branch in section 4 as a table-driven test), cookie attributes.
- **Repository** (dbtest, real Postgres): every query, uniqueness and cascade behaviour.
- **HTTP** (per endpoint): success, validation errors, enumeration-safe responses, rate limits, CSRF origin check.
- **OAuth**: a fake provider server (`httptest`) signing ID tokens with test keys, covering Google, Facebook (with and without email) and Apple (form POST, first-login names, private relay email, refresh token stored encrypted). Facebook data-deletion callback with valid and forged signatures.
- **Sessions**: `last_ip` updates on refresh, `authenticated_at` does not; sensitive actions fail with a stale `authenticated_at`.
- **Device names**: user-agent parsing for common browsers and OSes (table-driven), the `X-Device-Name` header from the app, length limit and control-character stripping, and the "Unknown device" fallback.
- **Refresh rotation**: rotate, reuse detection revokes the session, expired token, disabled user.
- **Jobs**: `auth_cleanup` deletes exactly what it should.

---

## 10. Build order

1. Migrations `00002_auth.sql`, `00003_profiles.sql`; sqlc queries.
2. `platform/jwt`, `shared/authctx`, auth middleware.
3. Password rules, argon2id, breach check.
4. Signup → verify email → login → refresh → logout / logout-all.
5. Forgot / reset / change / set password.
6. Sessions list and revoke; `auth_events` and `GET /auth/events`.
7. `profiles`: `/me`, profile and preferences updates; signup wiring.
8. OAuth framework + Google.
9. Facebook, including the "add your email" step and the data-deletion callback.
10. Apple (form POST callback, client secret JWT, first-login names, encrypted refresh token).
11. Link / unlink providers from settings.
12. `auth_cleanup` periodic job.
13. `.env.example`, README and ARCHITECTURE updates; ADR for the OAuth provider registry; extend the CLAUDE.md secrets rule to cover encrypted provider tokens.

Each step lands with its tests. Suggested PRs: steps 1–7 (password auth + profiles), steps 8–11 (social login), steps 12–13 (cleanup and docs).

---

## 11. Before social login can be tested

You need to create:

- **Google**: an OAuth client in Google Cloud Console (web application), with the callback URL.
- **Facebook**: a Meta app with Facebook Login; the `email` permission needs app review before real users can grant it.
- **Apple**: an Apple Developer account ($99/year), a Services ID, a Sign in with Apple key (`.p8`), and a verified domain for the callback.

Password auth (steps 1–7) needs none of these.

---

## 12. Out of scope for Phase 2

Changing email address, custom avatar upload, session location (city and country; no geo lookup for now), two-factor authentication, magic links, CAPTCHA, account deletion (note: Apple requires in-app account deletion with token revocation before an iOS release), and the frontend pages (Phase 4).
