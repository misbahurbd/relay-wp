# Phase 7: billing UI to the designs

Status: **Built on `feat/billing-ui`, not merged.** Rebuilds the billing pages to the **Billing** and **Spending & wallet** designs. Follows the rules in [CLAUDE.md](../CLAUDE.md) and the architecture in [ARCHITECTURE.md](ARCHITECTURE.md).

**The idea.** Phase 6 built the plan side of billing (plans, subscriptions, lifetime deals, checkout, invoices, billing profile). This phase builds the **whole UI** from the designs:

- **Real:** everything about the plan, wired to the Phase 6 API.
- **Preview:** everything that needs a backend Relay doesn't have yet (Meta costs, wallet, spending). It shows sample data with a clear **Preview** badge, so the screens are finished and reviewable now and only need their data source swapped later.

This phase is **mostly web**. Two backend changes: reading the saved card from the payment gateway and opening the gateway's portal to change it (section 4; Relay never stores card details), and plan details for the pricing table (section 4a, migration `00015_plan_details`).

---

## 1. What is real and what is preview

### Billing page (`/billing`)

| Section                                        | Real or preview | Data                                                                                                     |
| ---------------------------------------------- | --------------- | -------------------------------------------------------------------------------------------------------- |
| Current plan: name, price, status, renews on   | Real            | `GET /v1/workspace/billing` (plan, subscription or lifetime order)                                       |
| Next charge amount and date                    | Real            | Subscription price + `current_period_end`; hidden with no subscription, or when it cancels at period end |
| Next charge card "•••• 4242"                   | Real            | Read live from the gateway (section 4); omitted when the gateway shares no card details                  |
| Member since                                   | Real            | When the current plan started (the grant's `starts_at`); hidden on the free plan                         |
| Change plan, cancel, resume, pending downgrade | Real            | Existing Phase 6 flows (change-plan dialog, cancel and resume)                                           |
| Usage: team seats                              | Real            | Plan `limits.members` + the team seats endpoint                                                          |
| Usage: WhatsApp numbers                        | Real            | Plan `limits.whatsapp_numbers` + `GET /v1/workspace/whatsapp/numbers`                                    |
| Usage: contacts, broadcasts, flows, API calls  | Preview         | Sample numbers; no limits or counters for these yet                                                      |
| Compare plans + Monthly/Yearly + "Save N%"     | Real            | `GET /v1/billing/plans`; Save % = 1 - yearly / (monthly × 12), shown only when a plan has both prices    |
| Plan buttons (upgrade, downgrade, switch)      | Real            | Existing checkout and change-plan flows                                                                  |
| Meta message costs card                        | Preview         | Sample month; links to the Spending page                                                                 |
| Wallet balance, auto-recharge, Top up          | Preview         | Sample values; buttons open the preview dialog, which charges nothing                                    |
| Payment method card + Update                   | Real            | Card read live from the gateway; Update opens the gateway's portal (section 4)                           |
| Backup payment method                          | Real            | "Add backup payment method" opens the same portal, where more cards can be added                         |
| Billing email, address, tax ID                 | Real            | `GET/PUT /v1/workspace/billing/profile`                                                                  |
| Invoice history                                | Real            | `GET /v1/workspace/billing/invoices`, cursor paging                                                      |
| Invoice download                               | Real            | The invoice's `receipt_url`; no button when it has none                                                  |
| Download all                                   | Preview         | Disabled with "Coming soon"                                                                              |

### Spending page (`/billing/spending`, new)

Entirely preview. Every section of the design is built: the low-balance banner, summary cards, the This month / Last month / Last 3 months switcher, monthly budget with its toggles, the daily spend chart by category, spend by category, "How Meta charges", "Where the money went", spend by WhatsApp number and by country, the message wallet, wallet transactions, and the Top up dialog.

The page has one **Preview** banner at the top instead of a badge on every card. The budget field and toggles work locally, so they can be tried, but nothing is saved.

## 2. Rules for preview UI

- **Visible.** Every preview section shows a `Preview` badge, or the page banner on Spending, with the tooltip "Sample data. This part of billing is coming soon." Sample data must never pass for a customer's real numbers.
- **Harmless.** Preview actions never call the API and never take payment details. Top up and Download all are disabled, or open a dialog whose final button is disabled with "Coming soon". Toggles and inputs keep local state only.
- **Switchable.** `VITE_BILLING_PREVIEW` in `lib/env.ts` (boolean, default `true` in every environment, production included; decided 2026-10-30). Customers see the previews with their badges. The flag is a kill switch: when it is `false`, preview sections are not rendered, the Spending link is hidden, and `/billing/spending` redirects to `/billing`.
- **Same shape as the real thing.** Sample data comes through query options with the same shape the real endpoints will return, typed in `types.ts`. Going live means replacing a `queryFn` with an `@relay/api-client` call; components don't change.
- **One home for sample data.** All of it lives in `api/billing.preview.ts`. No sample values inside components.
- Preview hooks are named for what they return (`useWalletPreview`, `useSpendingPreview`) so a search finds every one when it goes live.

## 3. Files

Within `apps/web/src/features/billing/` (ADR 0004 layout). Components are grouped by area because the feature already has several.

```text
types.ts                       + preview types: UsageRow, MetaCostSummary, Wallet, WalletEntry, Spending*
api/
  billing.preview.ts           sample data + query options (preview only)
  billing.keys.ts              + keys: usage, paymentMethod, preview.*
  billing.queries.ts           + usage query (seats + numbers, from the real endpoints), payment method query
  billing.mutations.ts         + portal session (opens the gateway's portal)
hooks/
  use-billing-preview.ts       reads VITE_BILLING_PREVIEW
components/
  kit/preview-badge.tsx        the badge + tooltip
  kit/meter.tsx                one usage row (label, used / limit, bar, 80% warning)
  overview/                    the rebuilt /billing page (replaces settings/billing-page.tsx's overview tab)
    billing-overview.tsx
    current-plan-card.tsx      (from settings/current-plan.tsx + subscription-card.tsx)
    usage-card.tsx
    meta-costs-card.tsx        preview
    wallet-card.tsx            preview
    payment-details-card.tsx   payment method from the gateway + billing details (both real)
  pricing/
    compare-plans.tsx          inline table with the Monthly/Yearly toggle
  spending/                    all preview
    spending-page.tsx, summary-cards.tsx, budget-card.tsx, spend-chart.tsx, by-category.tsx,
    how-meta-charges.tsx, campaigns-table.tsx, by-number.tsx, by-country.tsx,
    wallet-panel.tsx, wallet-ledger.tsx, top-up-dialog.tsx
  settings/                    kept: invoices.tsx (+ download), profile-form.tsx, ltd-order.tsx, pay-dialog.tsx
```

Plus:

- `apps/web/src/routes/_app/_workspace/_shell/billing/spending.tsx`: a thin route (guard on `billing.view`, the preview flag check, render `SpendingPage`).
- `apps/web/src/components/ui/chart.tsx`: added with the shadcn CLI (`chart`), for the spend chart.
- `apps/web/src/lib/env.ts`: `VITE_BILLING_PREVIEW` (defaults to `true`; set it to `false` only to hide the previews).

The billing page's tabs (Overview, Invoices, Details) become one page in the design's order: current plan next to usage, compare plans, Meta costs (with the wallet) next to payment details, then invoices. Old `?tab=` links still open the page. Phase 6's `/billing/plans` and `/billing/checkout` routes stay as they are; compare plans links into them.

Seat and number counts come from the team and WhatsApp **endpoints** through `@relay/api-client`, with billing's own query keys. Billing never imports the `team` or `whatsapp` web features.

## 4. Saved card, read from the gateway

The card lives at the payment gateway. Relay stores only the customer and subscription IDs it already has (`billing_customers`, `subscriptions`), and asks the gateway for the card each time the page loads.

**Gateway (`platform/payment`).** Two methods on `Gateway`, with `ErrUnsupported` where a provider can't do it:

```go
// PaymentMethod returns the card the provider will charge next, or nil when there is none.
PaymentMethod(ctx context.Context, req PaymentMethodRequest) (*PaymentMethod, error)
// PortalURL returns a one-time link to the provider's page where the customer manages cards.
PortalURL(ctx context.Context, req PortalRequest) (string, error)

type PaymentMethod struct {
	Brand    string // visa, mastercard, …
	Last4    string
	ExpMonth int
	ExpYear  int
}
```

| Provider | `PaymentMethod`                                                                                                                                                   | `PortalURL`                                                                                                                          |
| -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| Stripe   | The subscription's `default_payment_method`, else the customer's `invoice_settings.default_payment_method`, expanded in the same call. Card brand, last 4, expiry | A Billing Portal session (`billing_portal.sessions.create`) with `return_url` = `/billing`                                           |
| Creem    | `ErrUnsupported` unless its API exposes card details (open question O2). The page then says the card is kept by Creem and offers Update (the portal)              | Creem's customer portal link (`POST /v1/customers/billing` → `customer_portal_link`; confirmed in Creem's API reference, 2026-10-30) |
| Fake     | A fixed test card, `visa •••• 4242`                                                                                                                               | A local URL back to `/billing`                                                                                                       |

**Routes.**

| Method | Path                                   | Permission     | Returns                                                                                                                                                                                                    |
| ------ | -------------------------------------- | -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| GET    | `/v1/workspace/billing/payment-method` | `billing.view` | `{ card?: { brand, last4, exp_month, exp_year }, provider?, card_details, can_manage }`. `card` is absent with no customer, when the provider failed, or when it shares no details (`card_details: false`) |
| POST   | `/v1/workspace/billing/portal-session` | `billing.pay`  | `{ url }`. The web opens it in the same tab                                                                                                                                                                |

**Rules.**

- **Nothing about the card is stored or logged.** It goes from the gateway straight into the response.
- **A gateway error never breaks the billing page.** `GET /payment-method` logs the error and returns `card: null`, and the page hides the card line.
- **The Stripe portal is set up for payment methods only.** In the Stripe dashboard (Settings › Billing › Customer portal), turn off cancelling and switching plans, so plan changes stay in Relay's flows, which know about grants, proration and lifetime deals. This is a dashboard setting, not code, so it must be done by hand in both test and live mode.
- **No cache.** The web reads the card each time the billing page opens; coming back from the portal (`/billing?from=portal`) is a fresh page load. The API keeps no cache.
- **The gateway is picked from the workspace's live subscription,** else its live order, else its most recent `billing_customers` row. With none, there is no card, and the card section shows "No card on file".

## 4a. Plan details for the pricing table

The design's plan cards show a one-line description, a feature list and a "Recommended" badge. Migration `00015_plan_details` adds them to `plans`:

| Column        | Type     | Rule                                            |
| ------------- | -------- | ----------------------------------------------- |
| `description` | `text`   | Up to 200 characters; may be empty              |
| `features`    | `text[]` | Up to 20 lines of up to 80 characters, in order |
| `recommended` | `bool`   | Shows the "Recommended" badge                   |

Like everything on a plan, they are set when a plan or a new version is created (the CRM's plan dialog has the fields) and never edited. `GET /v1/billing/plans` and the admin catalog return them. A plan with no features falls back to its limits ("5 team members", …).

The plan buttons follow the design: the current price says "Your current plan", a dearer one "Upgrade to <plan>" (filled), a cheaper one "Downgrade", the same plan at another interval "Switch to yearly/monthly". Yearly prices show their monthly equivalent with "Billed $X yearly".

## 5. Permissions

Unchanged from Phase 6: the page and the card need `billing.view`, and plan actions, the profile form and the card portal need `billing.pay`. Preview sections show to anyone who can see the page, since they do nothing.

## 6. Tracks

Four tracks that can run in parallel. Each ends with `task lint`, `task typecheck` and `task test` green, and a check in the browser in light and dark mode at desktop and phone widths.

| Track | Scope                                                                                                                                                                                                                         | Files                                                                                                                                    |
| ----- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| T1    | **Billing overview, real parts.** Single-page layout, current plan card (next charge, member since, change, cancel, resume), seats and numbers meters, compare plans with the toggle, billing details, invoices with download | `overview/` (real cards), `pricing/compare-plans.tsx`, `kit/meter.tsx`, `settings/invoices.tsx`, `billing.queries.ts`, `billing.keys.ts` |
| T2    | **Preview foundation + overview previews.** `VITE_BILLING_PREVIEW`, `use-billing-preview`, `preview-badge`, `billing.preview.ts`, preview types, then the preview usage rows, Meta costs, wallet and payment method cards     | `lib/env.ts`, `hooks/`, `kit/preview-badge.tsx`, `api/billing.preview.ts`, `types.ts`, `overview/*` (preview cards)                      |
| T3    | **Spending page.** Route, chart primitive, every section of the board on preview data, the Top up dialog (final button disabled)                                                                                              | `routes/.../billing/spending.tsx`, `components/ui/chart.tsx`, `spending/*`                                                               |
| T4    | **Card from the gateway (API).** `PaymentMethod` and `PortalURL` on Stripe, Creem and the fake gateway with tests, the two routes, `task gen`. Confirms Creem's portal path and whether it shares card details (O2)           | `platform/payment/*`, `billing/{service,handler,dto}/payment_methods*`, `routes.go`                                                      |

T1 and T2 share `overview/billing-overview.tsx`. T1 owns it, and T2 adds its cards behind the flag after T1 merges. T3 needs T2's preview data module, so T2 lands `billing.preview.ts` and the badge first, as a small first commit. T1 builds the card section against T4's generated client, so T4 lands its routes and `task gen` first, before the gateway work.

## 7. Done when

- `task lint`, `task typecheck`, `task test` and `task gen && git diff --exit-code` pass.
- No card number, expiry or portal URL appears in a log line or a database row.
- On Stripe test mode: the next-charge line and the payment card show the real test card, Update opens the Stripe portal, and a card changed there shows on `/billing` after returning.
- Real sections match the designs and show this workspace's real plan, limits, seats, numbers, invoices and profile, including the free, trial, lifetime and past-due states Phase 6 supports.
- Every preview section has its badge or banner, makes no API call (checked in the browser's network tab), and disappears with `VITE_BILLING_PREVIEW=false`.
- Phone width: cards stack, tables scroll inside their box, and there's no page-wide horizontal scroll.

## 8. Open questions

Settled: preview sections show to customers in production too, with their badges (section 2).

| #   | Question                                                                        | Default until answered                                                  |
| --- | ------------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| O1  | Are the design's plan names and limits (Starter, Growth, Pro) the real catalog? | No: compare plans shows whatever the catalog holds                      |
| O2  | Does Creem's API share card brand and last 4?                                   | No: the page says Creem keeps the card, and Update opens Creem's portal |

---

## Later: making the previews real

Kept so the next phases start from it. None of it is built in Phase 7.

| Preview                                         | Needs                                                                                                                                                                                                                                                                                                                                |
| ----------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Contacts, broadcasts, flows and API-call meters | New `plans.limits` keys (a missing key means unlimited), a billing `UsageCounter` interface filled by each feature in `internal/app`, contacts enforcement (409 on create, the import stops at the cap), `GET /v1/workspace/billing/usage`                                                                                           |
| Download all                                    | A zip job, or a CSV export                                                                                                                                                                                                                                                                                                           |
| Meta costs card, Spending page charts           | `whatsapp` syncs Meta's `pricing_analytics` (DAILY; category, pricing type, country, phone; COST + VOLUME) into `whatsapp_daily_costs` in micros every 6 hours, with a 90-day backfill. Meta calls the cost approximate and omits it for WABAs on a Solution Partner's credit line. Billing reads it through a `MetaCosts` interface |
| Monthly budget                                  | `spending_budgets`, 80% and 100% emails once each per month                                                                                                                                                                                                                                                                          |
| Campaign costs, pause marketing                 | Broadcasts                                                                                                                                                                                                                                                                                                                           |
| Wallet, top-ups, auto-recharge, ledger          | A decision to make Relay Meta's billing party (Phase 6 Q7b says no), plus broadcasts. A separate phase                                                                                                                                                                                                                               |
