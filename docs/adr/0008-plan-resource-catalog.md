# 0008. Plan resource catalog

- **Status:** accepted
- **Date:** 2026-10-30

## Context

A plan's limits live in `plans.limits` (jsonb), copied onto `plan_grants.limits` when a grant is made. Only two keys are read, through a fixed struct, `model.Limits{Members, WhatsAppNumbers}` (`billing/model/grants_model.go`):

- Team and WhatsApp each declare a one-method interface (`MemberLimit`, `NumberLimit`), and `internal/app` passes the billing service in. The check runs inside the feature's service, counts under a row lock, and returns `409 plan_member_limit` / `plan_number_limit`.
- The free plan is a Go constant, `FreeLimits`, used when a workspace has no grant or a suspended one. Changing it needs a deploy.
- A new limit means a new struct field, a new interface method, new CRM dialog fields and a new web meter. An unknown key in `limits` is ignored, so a typo silently means unlimited.
- Because grants copy the limits, raising a plan's limit only reaches future buyers. Existing customers keep the old numbers until they change plan.
- The only per-workspace override is a manual grant, which replaces the whole limits object.

[PHASE-7-BILLING.md](../PHASE-7-BILLING.md) plans contacts, broadcasts, flows and API-call limits next. These are not all the same shape:

1. **How many you have now** (members, numbers, contacts): counted live, under a lock.
2. **How many you used this period** (broadcasts, API calls): needs a counter that resets each month.
3. **Whether you may use it at all** (flows, API access): a yes/no that can gate a whole route.

We considered two tables and rejected both:

- **A `plan_resources` catalog table.** A resource does nothing until code enforces it (a count query, a usage counter or a route gate), so a row staff add in the CRM without that code would be a limit nobody checks.
- **A `plan_resource_limits` values table** (`plan_id`, `resource`, `value`). The keys would have no foreign key to point at (the catalog is in code), values mix numbers and booleans, and every limit check would need a join or a second query. Validating jsonb against the catalog gives the same guarantee with one row read.

## Decision

- **The catalog is a Go registry** in `billing/model/resources_model.go`: a fixed list of `Resource{Key, Kind, Unit, Label, Required, Enforced}`. It is not a factory (no implementation is chosen at runtime). `GET /v1/billing/resources` serves it so the CRM plan editor and the web meters render from it instead of hard-coding fields.
- **It lists everything the product sells**, from the Billing design: team seats, WhatsApp numbers, contacts, chatbot flows (count); broadcasts and API calls (monthly); automations, AI replies, Shopify & WooCommerce, roles/audit/SSO and priority support (access). `Required` resources (team seats, WhatsApp numbers) are in every plan. `Enforced` says a feature checks the limit; a resource that isn't enforced yet can be in a plan for the pricing table, and the CRM marks it so staff aren't misled.
- **Not every plan includes every resource.** A key a plan leaves out is not included: 0 for a count or monthly resource, no access for a feature. Unlimited is always stated, as `-1`. Values are a cap (`-1` unlimited) for count and monthly resources, and `1` for an included feature.
- **Three kinds:**

  | Kind      | Value                        | Enforced                                                                             | Refusal                   |
  | --------- | ---------------------------- | ------------------------------------------------------------------------------------ | ------------------------- |
  | `count`   | max items, `-1` unlimited    | in the owning feature's service, live count under its row lock (as today)            | `409`, feature's own code |
  | `monthly` | max per calendar month (UTC) | billing's `usage_counters`, one atomic `UPDATE … WHERE used + n <= limit`            | `409`, feature's own code |
  | `access`  | `1` included, absent not     | `RequirePlan(key)` route middleware, added with the first enforced `access` resource | `402 plan_required`       |

  The kind is called `access`, not `feature`, because `plans.features` already holds the pricing card's marketing lines.
- **Limits are live, not copied.** A grant points at its plan version through `plan_id` and has no limits of its own. Every check reads `plans.limits` as it is now, so a change to a plan version reaches every workspace on it on its next request. There is no job and no cache.
- **A plan version can be edited in place, except its code, version and prices.** Providers hold copies of prices, so prices stay immutable. Limits, the pricing card and the trial are only ours (`PUT /v1/admin/billing/plans/{id}`). A change to the limits needs a reason. To keep existing customers on their old terms, publish a new version for new buyers and leave the old one alone.
- **Overrides are per grant.** `plan_grants.overrides` (jsonb, default `{}`) holds only the keys that differ for that workspace, for example `{"members": 20}`. Staff set them with `PUT /v1/admin/workspaces/grant-overrides`, with a required reason. A new grant (a plan change) starts with no overrides.
- **Effective limits are `plans.limits || plan_grants.overrides`**, merged key by key, with overrides winning. A key missing from both is not included; an override of `0` takes a resource away from one workspace (never a required one). One query resolves it:

  ```sql
  SELECT p.code, (p.limits || g.overrides) -> @key AS value
  FROM plan_grants g JOIN plans p ON p.id = g.plan_id
  WHERE g.workspace_id = @workspace_id AND g.status = 'active';
  ```

  No active grant (none, or suspended) reads the same key from the latest active `free` version.
- **Values are validated against the catalog** when a plan is created or edited, or overrides are set. An unknown key, a value of the wrong kind, or a plan without a required resource is refused.
- **Every limit or override edit is audited** in `limit_changes` (who, when, the plan or grant, before and after, reason), because an edit now affects live customers.
- **One billing method replaces the per-limit ones:** `Limit(ctx, workspaceID, key string) (int64, error)`, plus `Consume`/`Release` for `monthly` and `HasAccess` for `access`. Each feature keeps declaring its own small interface and its own key constants (features never import billing). An unknown key is an error, and a test in `internal/app` checks that every key a feature uses is in the catalog.
- **The free plan becomes a `plans` row** (`code = 'free'`, no prices, seeded by migration with today's `FreeLimits`). Staff edit its limits like any other plan, and every free workspace follows. `FreeLimits` is deleted.
- **`usage_counters`** (`workspace_id`, `resource`, `period_start`, `used`; primary key on the first three) is billing's table. A feature calls `Consume` before the action and `Release` if the action then fails. Counters are never reset in place; a new month is a new row.

## Delivery

Migrations `00017_plan_resources` and `00018_plan_resources_included` build the whole catalog and live limits with per-grant overrides, the `free` plan row, `limit_changes`, `GET /v1/billing/resources`, `PUT /v1/admin/billing/plans/{id}`, `PUT /v1/admin/workspaces/grant-overrides`, and the CRM plan editor (one sheet for add, new version and edit) and per-workspace limits. Team seats, WhatsApp numbers and contacts are enforced (contacts: `409 plan_contact_limit` on create; an import skips new rows past the cap). `usage_counters`, `Consume`/`Release`, `HasAccess` and `RequirePlan` arrive with the first enforced resource of their kind, so none of them ship unused. The free plan can't be sold or granted by hand: it is what a workspace has with no grant.

## Migration

1. Add `plan_grants.overrides`. For each grant whose `limits` differ from its plan's, copy only the differing keys into `overrides`.
2. Drop `plan_grants.limits`, and update `CreateGrant`, `GetCurrentGrant` and `ActivateGrant` (whose `GrantInput.Limits` becomes `Overrides`).
3. Seed the `free` plan row and switch `planOf` to it.
4. Update the comment on `plans` in `00013_billing.sql` ("never edited once sold") in the new migration: limits may change, prices may not.
5. Switch a missing key from unlimited to not included: every existing plan gets `-1` for team seats, WhatsApp numbers and contacts where it had no key, so nobody loses what they had.

## Consequences

- A new limit is one catalog entry, one key constant in the feature that enforces it, and the enforcement itself. The CRM and web pick it up from the API.
- Raising a limit reaches existing customers at once. Lowering one never removes anything: members or numbers already over the new cap stay, and only new adds are refused, the same as a lapsed plan today.
- Keeping old terms is a deliberate act (publish a new version), not the default. Staff must know that editing a version's limits changes it for everyone on it; the CRM should say how many workspaces a change affects before saving.
- A typo in limits or overrides is refused when it is saved, instead of meaning unlimited.
- A plan inserted by hand with `limits = '{}'` includes nothing. Plans come from the CRM, which always sends the required resources.
- Free limits become data: the uncommitted change of `FreeLimits.Members` from 5 to 1 becomes the seed value of the `free` row instead.
- The pricing page and admin catalog must handle a plan with no prices (`free`): the public list either hides it or shows it as "Free".
- `Consume` runs outside the feature's transaction, so a crash between `Consume` and `Release` can overcount by one action. That is acceptable for monthly quotas; reconcile only if it shows up in support.
- What to watch: keep the catalog small and kinds fixed. A limit that fits none of the three kinds (add-on seats, a wallet) needs its own ADR, not a fourth kind slipped in.
