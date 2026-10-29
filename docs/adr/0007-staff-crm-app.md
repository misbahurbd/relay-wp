# 0007. Staff CRM as its own app

- **Status:** accepted
- **Date:** 2026-10-30

## Context

Billing needs staff tools before launch: build the plan catalog (plans, prices, installment offers,
provider links), issue discount codes, and move a workspace between plans by hand. The API side is
`/v1/admin` ([PHASE-6-BILLING.md](../PHASE-6-BILLING.md) section 8), open only to verified accounts
listed in `ADMIN_EMAILS`.

A first version lived in the customer web app at `/admin`. That ships staff screens, and the shape
of the admin API, to every customer's browser; it ties staff tools to the web app's releases; and
it can't be put behind a separate network rule or host.

## Decision

- Staff tools are a separate app, `apps/crm`: a static Vite + React single-page app, deployed on
  its own origin (for example `crm.<domain>`), no SSR.
- It uses `@relay/api-client` like the web app and calls only `/v1/admin`, sign-in and `/v1/me`.
  The API admits its origin through `CORS_ALLOWED_ORIGINS`; `COOKIE_DOMAIN` must cover both hosts.
- Staff sign in with email and password on the CRM itself. Authorization stays in the API
  (`RequireStaff`); the CRM's own checks are only for showing the right screen.
- `apps/crm` keeps the web app's feature layout (ADR 0004) and its own copy of the shadcn
  primitives. `apps/web` and `apps/crm` never import each other; anything both need goes in a
  package under `packages/`.

## Consequences

- Customer bundles carry no staff code; the CRM can be deployed, restricted (VPN, IP allow-list,
  basic auth at the edge) and rolled back on its own.
- Two copies of the UI primitives and design tokens can drift. If the CRM grows, move them into a
  shared `packages/ui`.
- Social-only accounts can't sign in to the CRM until it gets the OAuth flow; staff use a password.
- This is still temporary: before several people use it, add staff roles and an audit log in the
  API instead of the `ADMIN_EMAILS` list.
