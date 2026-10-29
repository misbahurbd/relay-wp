# Phase 6: billing

Status: **Wave 0 (contracts) built.** The schema, queries, model, DTOs, routes and the payment gateway interface are in place; every billing endpoint answers `501` until its track fills it in. Follows the rules in [CLAUDE.md](../CLAUDE.md) and the architecture in [ARCHITECTURE.md](ARCHITECTURE.md). The payment provider registry is [ADR 0006](adr/0006-payment-gateway-registry.md).

The schema is migration `00011_billing`, built from the load-tested draft v12 of the billing schema page.

---

## 1. Decisions

| #   | Question                                         | Decision                                                                                                                                                                                                                                                               |
| --- | ------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Q1  | Does an LTD buyer get access from installment 1? | Yes. Paying installment 1 creates an active grant with full lifetime limits.                                                                                                                                                                                           |
| Q2  | A missed installment?                            | The grant is suspended after the offer's `grace_days` and restored when paid. The order defaults after `max_missed`. Both are set per offer.                                                                                                                           |
| Q3  | Marketplace redemption codes?                    | No. Lifetime deals sell from Relay's own pricing page through normal checkout. A visitor signs up and creates a workspace first.                                                                                                                                       |
| Q4  | Installment total vs one-time price?             | Set per offer: `installment_offers.total_minor` may equal the one-time price or add a markup. The last installment absorbs rounding.                                                                                                                                   |
| Q5  | Is the free plan a row?                          | No. No current grant means free. Trials are per plan (`trial_days`, 0 turns them off; `trial_requires_card`). A card-less trial is a `plan_grants` row with `source = trial` that ends at the trial end. A trial with a card is a subscription in `trialing`.          |
| Q6  | Mid-period plan changes?                         | Upgrades apply now with proration. Downgrades wait for the period end through `subscriptions.pending_plan_price_id`. Both are logged in `subscription_changes`.                                                                                                        |
| Q7a | Flat or per-member pricing?                      | Flat. A plan has several prices (monthly, yearly, lifetime); a lifetime price can have installment offers. Member caps live in `plans.limits`.                                                                                                                         |
| Q7b | Who pays Meta for WhatsApp messages?             | The customer, directly. No credit ledger in this phase.                                                                                                                                                                                                                |
| Q8  | Keep `workspaces.plan`?                          | As a cache, written only in the same transaction as `plan_grants` (and `trial_ends_at` likewise). Tested per grant change, with a rollback test, and a nightly job logs drift.                                                                                         |
| Q9  | Launch providers?                                | Stripe and Creem, in USD. PayPal and SSLCommerz later, with no schema change.                                                                                                                                                                                          |
| Q10 | Which provider does a customer get?              | **Deferred.** Until then checkout offers any configured launch provider that has a `provider_prices` row for the chosen price.                                                                                                                                         |
| Q11 | Who can manage billing?                          | The owner, plus members whose role has `billing.view` (read plans, invoices, profile) or `billing.pay` (check out, change or cancel a plan, edit the billing profile). Both already exist in `shared/authctx`.                                                         |
| Q12 | Money paid on a defaulted LTD?                   | Kept, no refund. Paying the remaining balance resumes the order and restores the lifetime grant.                                                                                                                                                                       |
| Q13 | A subscriber buys an LTD?                        | The subscription is canceled at once. Unused time becomes `orders.credit_minor`; `orders.replaced_subscription_id` records it.                                                                                                                                         |
| Q14 | Tax at launch?                                   | Creem charges tax as merchant of record. Stripe sales carry no tax line until Relay registers and turns on Stripe Tax.                                                                                                                                                 |
| Q15 | Where does checkout happen?                      | Embedded in Relay: Stripe Elements. Creem depends on the findings below; if it is hosted only, Creem payments open its page.                                                                                                                                           |
| Q16 | LTD tier upgrade during installments?            | Allowed any time. The old order becomes `upgraded`, its open installments are canceled, what was paid moves to the new order as `credit_minor` (`upgraded_from_order_id`), and a new schedule covers the rest.                                                         |
| Q17 | Deleting a workspace on a paid plan?             | Blocked until the subscription is canceled. An LTD workspace must confirm it gives up the deal. Workspaces are soft-deleted (`workspaces.deleted_at`) so billing records survive.                                                                                      |
| Q18 | Discount codes at launch?                        | Yes, owned by Relay (`discounts`, `discount_redemptions`, `provider_discounts`). Relay computes the discount for one-time payments and LTDs; provider coupons mirror it only on subscriptions the provider renews. On installments it comes off the order total first. |
| Q19 | Who numbers invoices?                            | Stripe and Creem at launch; `invoices.number` stores theirs. A gap-free Relay sequence comes with PayPal or SSLCommerz.                                                                                                                                                |

## 2. Rules

### Integrity

- Billing foreign keys to `workspaces` are `ON DELETE RESTRICT`. Financial records outlive a workspace.
- Price rows are never updated: deactivate and insert a replacement. Uniqueness on prices, offers and provider prices applies to active rows only.
- External IDs are unique per provider, `UNIQUE (provider, external_…)`, and nullable until the provider returns them.
- `CHECK` on every status, provider and currency (`^[A-Z]{3}$`), and on amounts.
- One live subscription (`trialing`, `active`, `past_due`), one live order (`in_installments`, `paid`, `defaulted`) and one current grant (`active`, `suspended`) per workspace, each a partial unique index.
- `orders.paid_minor` moves in the same transaction as each paid invoice; `discount_minor + credit_minor + paid_minor <= total_minor`.
- A discount reservation bumps `discounts.redeemed_count` and inserts the redemption in **one statement** (`ReserveDiscount`), so `max_redemptions` holds under concurrency.

### Security and webhooks

- A webhook maps to a workspace only through our `checkout_sessions.id` (sent to the provider as its reference), or a provider subscription ID we stored. Never through an email address.
- The SSLCommerz success redirect never activates anything. Only the IPN plus the validation API do.
- Provider keys and webhook secrets live in config only. No table holds them.
- `payment_events.payload` is never logged or returned, and is nulled after 90 days. No query selects it: `InsertPaymentEvent` writes it and `PurgeEventPayloads` nulls it. Jobs carry the normalised event instead.
- River owns retries. `payment_events` keeps only `processed_at`.
- The Stripe Elements client secret goes straight to the browser and is never stored.
- `billing_profiles` holds personal data: never logged.

## 3. Providers

| Provider   | Currency | Renews subscriptions     | LTD installments                                         | Webhook check                     |
| ---------- | -------- | ------------------------ | -------------------------------------------------------- | --------------------------------- |
| Stripe     | USD      | Stripe                   | `provider_schedule`: subscription schedule, N iterations | `Stripe-Signature` + secret       |
| Creem      | USD      | Creem                    | `app_link`, unless Creem supports it (see findings)      | HMAC signature header             |
| PayPal     | USD      | PayPal (later)           | `provider_schedule`: billing plan, `total_cycles = N`    | Verify-signature API              |
| SSLCommerz | BDT      | Relay, River job (later) | `app_link`, or `provider_financed` (bank EMI)            | IPN, then validation API `val_id` |

`PAYMENT_PROVIDERS` lists the enabled providers, for example `stripe,creem`. In development, `stripe:fake` registers the fake gateway under the Stripe name: no network, no keys.

## 4. Code layout

```text
platform/payment/      payment.go (Gateway, Capabilities, Event, Registry, New), fake.go, stripe.go, creem.go
features/billing/
  handler/     routes.go, handler.go, <area>_handler.go, webhook_handler.go
  service/     service.go, <area>_service.go, events_service.go
  repository/  repository.go, <area>_queries.sql
  dto/         dto.go, <area>_dto.go
  model/       model.go, <area>_model.go, errors.go, job_args.go
  jobs/        jobs.go
```

## 5. Routes

Reads need `billing.view`; buying, changing a plan and editing the profile need `billing.pay`. Workspace routes take `X-Workspace-ID`.

| Method | Path                                                                      | Permission      |
| ------ | ------------------------------------------------------------------------- | --------------- |
| GET    | `/v1/billing/plans`                                                       | public          |
| POST   | `/v1/webhooks/payments/{provider}`                                        | provider-signed |
| GET    | `/v1/workspace/billing`                                                   | `billing.view`  |
| POST   | `/v1/workspace/billing/checkout-sessions`                                 | `billing.pay`   |
| GET    | `/v1/workspace/billing/checkout-sessions/{sessionId}`                     | `billing.view`  |
| POST   | `/v1/workspace/billing/discounts/preview`                                 | `billing.view`  |
| GET    | `/v1/workspace/billing/subscription`                                      | `billing.view`  |
| POST   | `/v1/workspace/billing/subscription/change-preview`                       | `billing.view`  |
| POST   | `/v1/workspace/billing/subscription/change`                               | `billing.pay`   |
| DELETE | `/v1/workspace/billing/subscription/pending-change`                       | `billing.pay`   |
| POST   | `/v1/workspace/billing/subscription/cancel`                               | `billing.pay`   |
| POST   | `/v1/workspace/billing/subscription/resume`                               | `billing.pay`   |
| POST   | `/v1/workspace/billing/orders/quote`                                      | `billing.view`  |
| GET    | `/v1/workspace/billing/orders/current`                                    | `billing.view`  |
| GET    | `/v1/workspace/billing/orders/{orderId}`                                  | `billing.view`  |
| POST   | `/v1/workspace/billing/orders/{orderId}/installments/{installmentId}/pay` | `billing.pay`   |
| POST   | `/v1/workspace/billing/orders/{orderId}/resume`                           | `billing.pay`   |
| GET    | `/v1/workspace/billing/invoices`                                          | `billing.view`  |
| GET    | `/v1/workspace/billing/invoices/{invoiceId}`                              | `billing.view`  |
| GET    | `/v1/workspace/billing/profile`                                           | `billing.view`  |
| PUT    | `/v1/workspace/billing/profile`                                           | `billing.pay`   |

## 6. Creem findings

_To be filled in by track T2 from Creem's official docs, with links: embedded or hosted checkout, native installments, webhook signature scheme, test mode, refunds, and how tax shows in its payloads._
