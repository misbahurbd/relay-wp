# 0011. API keys act as their creator in one workspace

- **Status:** accepted
- **Date:** 2026-10-30

## Context

The Integrations page gives customers an API key, so their own software, Zapier, Make or n8n can call Relay. Until now every request came from a person's session: a JWT in the `Authorization` header (mobile) or the access cookie (web), checked by `RequireAuth`, then `RequireWorkspace` resolved the member's access.

A key needs three things settled: what it's allowed to do, where the check lives, and how a leaked key is contained.

We looked at two permission models:

- **Its own permission list on the key.** Flexible, but it's a second access model next to roles and overrides (Phase 3). It also has to be kept below the creator's access forever, including after the creator's role changes.
- **The key acts as the member who made it, in the workspace it was made in.** No new model: `RequireWorkspace` resolves the creator's current access on every request, as for a session. Suspending or removing the member, or narrowing their role, narrows the key at once.

## Decision

- A key is `relay_live_` followed by 32 random bytes in base64url. Only its SHA-256 is stored. A slow hash adds nothing at this entropy, and a unique index makes the lookup one probe.
- `RequireAuth` treats a bearer token that starts with `relay_live_` as a key. It asks a `middleware.KeyVerifier` (implemented by `features/integrations`), then puts the creator's user ID and the key's workspace in the context (`authctx.WithAPIKey`). Cookies are never keys.
- Keys work **only under `/v1/workspace/`**. Elsewhere they get 403 `api_key_not_allowed`.
- `RequireWorkspace` returns 404 when the `X-Workspace-ID` header names any other workspace than the key's. It then checks the creator's access as usual.
- `middleware.RequireSession` refuses keys (403 `session_required`). Every integrations route uses it, so a key can't create keys, webhooks or connections, and a leak can't spread.
- `last_used_at` is written at most once a minute per key.
- Making a key needs `integrations.api_keys`, which is marked sensitive in the catalog.

## Consequences

- One access model covers people and their keys. A key never exceeds what its creator can do today.
- When the creator leaves the workspace, their keys stop working. This is safe, but a team's integration can break when one person leaves. The page shows each key's creator, so the cause is visible. Workspace-owned keys with their own permissions can come later as an override list on the key.
- Handlers that read the session ID see `uuid.Nil` for key requests. Routes that manage sessions are outside `/v1/workspace/`, so keys can't reach them.
