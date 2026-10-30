# Phase 5 plan: integrations

Status: **Built.** Covers the Integrations page in the ChatPulse prototype ("Integrations"): the app catalog, API keys, and outgoing webhooks, plus connections to Shopify, WooCommerce and Google Sheets. Follows the rules in [CLAUDE.md](../CLAUDE.md) and the architecture in [ARCHITECTURE.md](ARCHITECTURE.md). API keys as credentials are recorded in [ADR 0011](adr/0011-api-keys.md).

Syncing store orders into contacts and messages, and writing contacts to a sheet, need the Contacts and Inbox features on `main`; this phase stores and checks the connections so those jobs only have to read them.

---

## 1. Requirements

### Functional

| #   | Requirement                                                                                                                                                                                                           |
| --- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| F1  | List every app with its category, whether it's connected and a one-line status ("1 active key · last used 4 min ago"). Filter by category and search.                                                                 |
| F2  | **API keys.** Create (shown once), list (prefix and last 4 only), roll ("Regenerate": new key, old one revoked at once), revoke. Up to 20 active keys per workspace.                                                  |
| F3  | An API key authenticates calls to `/v1/workspace/...` routes as the member who made it, in the workspace it was made in.                                                                                              |
| F4  | **Webhooks.** Up to 10 endpoints per workspace, each with a URL, a description, the events it wants, and on/off. Signing secret shown on request and rotatable.                                                       |
| F5  | Every event is POSTed as signed JSON, retried with backoff for about 1.5 hours, and logged (status, response code, time taken). Redeliver any logged delivery.                                                        |
| F6  | "Send test event" delivers a `ping` at once and shows the result.                                                                                                                                                     |
| F7  | **Shopify, WooCommerce:** connect in one click (the owner approves Relay on the store) where the server supports it, else with credentials. Both are checked against the store before saving. Test again, disconnect. |
| F8  | **Google Sheets:** connect with Google sign-in (offline access, `spreadsheets` scope), disconnect.                                                                                                                    |
| F9  | **Zapier, Make, n8n:** connect by creating an API key tagged with the app; the card shows connected while one is active.                                                                                              |

### Non-functional

- **Security.** Keys and hashes never leave their query (section 4). Store secrets are encrypted. Webhook targets can't reach private networks (SSRF). Keys can't manage keys, webhooks or connections, so a leaked key can't create more access for itself.
- **Permissions.** Keys need `integrations.api_keys`. Webhooks and connections need `integrations.manage`. Any member can see the catalog.
- **Latency.** API key check: one indexed lookup, like the session check (< 2 ms). Webhook delivery never blocks the request that caused the event.
- **Delivery.** At least once. Receivers dedupe on `Relay-Delivery`.

## 2. Estimates

Assume 10,000 workspaces, 10% with a webhook endpoint, and 500 messages a day each, giving about 3 events per message (received, sent, delivered or read).

| Quantity               | Estimate                                                                                                                  |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| Events with a receiver | 1,000 × 500 × 3 ≈ 1.5 M/day ≈ **17/s average, ~85/s peak**                                                                |
| Delivery log           | 1.5 M/day × ~1 KB × 30 days ≈ **45 GB**; kept 30 days, deleted daily                                                      |
| Worker time            | p50 receiver ~300 ms → 85/s × 0.3 s ≈ 26 deliveries in flight at peak, inside River's default workers                     |
| API key checks         | Same rate as authenticated requests; one unique-index lookup each, plus at most one `last_used_at` write a minute per key |

Postgres and River are enough at this size. If the delivery log outgrows one table, partition it by month the way `access_log` is.

## 3. Design

```text
             ┌──────────── /v1/workspace/integrations/... (session only) ────────────┐
web ───────► │ handler → service → repository → Postgres (api_keys, webhook_*, integration_connections)
             └──────────────────────────────────────────────────────────────────────┘
API client ─ Bearer relay_live_… ─► RequireAuth ──► KeyVerifier (integrations) ─► RequireWorkspace (as the key's creator)

feature event ─► integrations.Publish(tx, …) ─► webhook_deliveries + River job (same tx)
                                                       │
                                     webhook_deliver worker ─► signed POST ─► receiver
```

### API keys

- Format `relay_live_` + 32 random bytes in base64url. Stored as **SHA-256** (the key is high entropy, so a slow hash adds nothing) under a unique index. Shown once.
- The list shows `relay_live_••••7f3a`: a prefix and the last 4 characters.
- A key acts as its creator **with their current access**: suspending or removing the member, or narrowing their role, narrows the key on the next request. Removing the member makes the key useless.
- `RequireAuth` sees `Bearer relay_live_…`, asks the `KeyVerifier` and puts the user and the key's workspace in the context. `RequireWorkspace` refuses (404) a header naming another workspace. Keys work only under `/v1/workspace/`.
- `RequireSession` sits on the integrations routes, so a key can't create keys, endpoints or connections.

### Webhooks

- **Signature** (Stripe-style, so existing verifier code works): `Relay-Signature: t=<unix>,v1=<hex HMAC-SHA256(secret, "<t>.<body>")>`. Other headers are `Relay-Event`, `Relay-Delivery` (the delivery ID, to dedupe on) and `User-Agent: Relay-Webhooks/1.0`.
- **Body:** `{"id","type","created_at","workspace_id","data"}`.
- **Outbox.** `Publish` inserts one delivery row per matching endpoint and its River job **in the caller's transaction**, so an event is sent exactly when its change commits.
- **Retries.** A 2xx response succeeds. Any other answer, or no answer after 10 s, retries on River's backoff (attempt⁴ seconds plus jitter): 8 attempts over about 1.3 hours.
- **Health.** Each delivery that finally fails adds to `consecutive_failures`, and a success resets it. At 20 the endpoint is turned off with a reason, and the page says so.
- **SSRF.** Targets must be `https` (`http` only in development). The dialer refuses loopback, private, link-local, CGNAT and multicast addresses _after_ DNS resolves, so rebinding can't sneak through. Redirects are not followed.
- **Secret.** `whsec_` + 24 random bytes, encrypted with `TOKEN_ENCRYPTION_KEY` (it must be readable to sign). Shown on request, rotated on request.

### Connections

| Provider      | One click (OAuth)                                                                                                      | By hand                                                      | Checked by                                       |
| ------------- | ---------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------ | ------------------------------------------------ |
| Shopify       | Shopify app install: code → offline Admin API token (`read_customers,read_orders`). Needs `SHOPIFY_CLIENT_ID`/`SECRET` | Store domain and an Admin API access token from a custom app | `GET https://{shop}/admin/api/2025-10/shop.json` |
| WooCommerce   | `/wc-auth/v1/authorize`: the store posts read-only REST keys to our callback. Needs an https `API_BASE_URL`            | Store URL and REST consumer key and secret                   | `GET {url}/wp-json/wc/v3/system_status`          |
| Google Sheets | OAuth code (offline, `spreadsheets` + `email` scopes) → refresh token                                                  | (none)                                                       | Google's token and userinfo endpoints            |
- Credentials are JSON, encrypted with AES-256-GCM, in `integration_connections.credentials`. There is one row per workspace and provider. Disconnecting clears the credentials and keeps the row for its history.
- **OAuth state** is an HMAC-signed `{workspace, user, provider, target, expiry}` (15 minutes), so no table is needed. Google and Shopify also bind it to the browser with a nonce cookie scoped to their callback path, and their callbacks redirect to `/integrations?connected=<provider>` (or `?integration_error=<code>`).
- **Shopify** callbacks must carry Shopify's `hmac` over the other query parameters (signed with the app secret) and name the same shop the person entered. Its redirect URL is `{API_BASE_URL}/v1/integrations/shopify/callback`.
- **WooCommerce** calls back server to server (`POST {API_BASE_URL}/v1/integrations/woocommerce/callback`, the signed state in `user_id`), so there is no cookie: the posted keys are accepted only if they work on the store named in the state. WooCommerce refuses non-https callbacks, so one-click is offered only when `API_BASE_URL` is https. The store then sends the person to `/integrations?provider=woocommerce&success=1|0`.
- Zapier, Make and n8n stay on API keys: for them to sign in with OAuth, Relay would have to be an OAuth provider itself (later work).
- Google Sheets needs `GOOGLE_CLIENT_ID`/`GOOGLE_CLIENT_SECRET`, the same Google app as sign-in. Its redirect URI is `{API_BASE_URL}/v1/integrations/google/callback`. Without them the card says it's unavailable.

## 4. Data

Migration `00019_integrations.sql`, after contacts (00011, 00012) and billing (00013 to 00018).

| Table                     | Notes                                                                                                                                                                                                               |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `api_keys`                | `workspace_id`, `name`, `app` (`zapier`/`make`/`n8n`/null), `prefix`, `last4`, **`token_hash`** (secret: read only by `GetAPIKeyByHash`'s `WHERE`), `created_by`, `last_used_at`, `revoked_at`                      |
| `webhook_endpoints`       | `url`, `description`, `events text[]`, `enabled`, **`secret`** (encrypted; read only by `GetWebhookSecret` and the delivery worker's `GetDeliveryForSend`), `consecutive_failures`, `disabled_reason`               |
| `webhook_deliveries`      | `endpoint_id`, `event_id`, `event_type`, `payload jsonb`, `status` (`pending`/`succeeded`/`failed`), `attempts`, `response_status`, `response_ms`, `error`, `delivered_at`                                          |
| `integration_connections` | `provider`, `status` (`connected`/`error`/`disconnected`), `account_label`, `config jsonb`, **`credentials`** (encrypted; read only by `GetConnectionCredentials`), `last_checked_at`, `last_error`, `connected_by` |

Every table has `workspace_id`, and every query filters on it.

## 5. API

Everything is under `/v1/workspace/integrations` and takes `X-Workspace-ID`.

| Method and path                                                                                      | Permission              |
| ---------------------------------------------------------------------------------------------------- | ----------------------- |
| `GET /`                                                                                              | member                  |
| `GET /events`                                                                                        | member                  |
| `GET, POST /api-keys`; `POST /api-keys/{id}/roll`; `DELETE /api-keys/{id}`                           | `integrations.api_keys` |
| `GET, POST /webhooks`; `PATCH, DELETE /webhooks/{id}`                                                | `integrations.manage`   |
| `GET /webhooks/{id}/secret`; `POST /webhooks/{id}/secret/rotate`; `POST /webhooks/{id}/test`         | `integrations.manage`   |
| `GET /webhooks/{id}/deliveries`; `POST /webhooks/{id}/deliveries/{deliveryId}/redeliver`             | `integrations.manage`   |
| `PUT /connections/{provider}`; `POST /connections/{provider}/test`; `DELETE /connections/{provider}` | `integrations.manage`   |
| `POST /connections/google_sheets/authorize` → `{url}`                                                | `integrations.manage`   |
| `POST /connections/shopify/authorize` `{shop_domain}` → `{url}`                                      | `integrations.manage`   |
| `POST /connections/woocommerce/authorize` `{store_url}` → `{url}`                                    | `integrations.manage`   |
| `GET /v1/integrations/google/callback` (public; checks the signed state)                             | none                    |
| `GET /v1/integrations/shopify/callback` (public; checks `hmac` and the signed state)                 | none                    |
| `POST /v1/integrations/woocommerce/callback` (public, server to server; checks the state and keys)   | none                    |

**Events:** `message.received`, `message.status`, `contact.created`, `contact.updated`, `broadcast.completed` and `template.status_changed`. `ping` is used only for test events.

## 6. Operations

- The **`webhook_deliver`** job has 8 attempts. The **`integrations_cleanup`** job runs daily on the leader and deletes deliveries older than 30 days.
- Logs record each delivery's endpoint, status code and duration. They never include the body or secret.
- Watch three things: River's failed `webhook_deliver` count, how many endpoints are turned off, and the p95 of `response_ms`.

## 7. Trade-offs and later work

- Secrets are stored encrypted, not hashed. Signing needs the plain secret, and showing it again is what the prototype asks for.
- Keys act as a person rather than holding their own permission list. This is simpler and safe, because a key can never exceed its creator. Scoped keys can come later as an override list on the key.
- **Not built yet:** importing Shopify and WooCommerce customers and orders (needs Contacts), order-triggered messages (needs Inbox and Automations), and syncing to sheets (needs Contacts). Each becomes a River job that reads the connection's credentials. Events are published once the features that produce them exist on `main`.
