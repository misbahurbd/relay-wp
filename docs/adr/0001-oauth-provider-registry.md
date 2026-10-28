# 0001. OAuth provider registry

- **Status:** accepted
- **Date:** 2026-10-28

## Context

Social sign-in supports Google and Facebook now, and Apple later. Every provider goes through the same steps:

1. Build an authorization URL.
2. Exchange the returned code.
3. Turn the provider's answer into one normalised identity: provider user ID, email, whether the provider verified the email, names and avatar.

What differs is the protocol detail:

- **Google:** OpenID Connect with PKCE and a nonce, plus ID-token verification against Google's published keys.
- **Facebook:** OAuth 2.0 followed by a Graph API `/me` call signed with `appsecret_proof`.
- **Apple:** form-POST callback and a client-secret JWT.

The routes are `/v1/auth/oauth/{provider}/…`, so the implementation is chosen per request from the `{provider}` path value. A provider whose credentials are missing must not be offered at all (`404`), so password-only local development keeps working.

`CLAUDE.md` section 2 allows a factory only when all three of its conditions hold. All three do here:

1. There are two real implementations today, with a third planned.
2. The implementation is selected by a key: the `{provider}` path value.
3. They are built once at startup.

## Decision

- `features/auth/provider.go` declares the `Provider` interface (`Name`, `AuthURL`, `Exchange`) and `NewProviders(cfg)`. `NewProviders` builds every configured provider with a plain `switch`, once at startup, and returns a `Providers` map keyed by name.
- There is one file per implementation: `provider_google.go` and `provider_facebook.go`, with `provider_apple.go` to come.
- A provider with no credentials is not added to the map. Its routes answer `404 provider_not_available`.
- An incomplete configuration fails at startup, for example a client ID without its secret.
- `internal/app` builds the registry and passes it to `auth.New`. Handlers and the service only look up a provider by name; they never build one.
- Provider endpoints (authorization, token, keys, Graph API) are fields with production defaults. Tests point them at a fake `httptest` provider; production never sets them.

## Consequences

- Adding Apple means adding one file and one `case`. The flow code (`oauth.go`), which handles state, the browser-binding cookie, linking rules, sessions and events, stays shared.
- The linking rules are written once, so every provider gets the same protections. Examples: an email the provider did not verify is never linked to an existing account, and the last sign-in method can't be removed.
- Each provider's quirks stay inside its own file. Examples: Facebook never vouches for email, and Apple sends names only on the first sign-in.
- What to watch: the interface must stay small. If one provider needs something unusual, keep it in that provider's file instead of widening the interface for everyone.
