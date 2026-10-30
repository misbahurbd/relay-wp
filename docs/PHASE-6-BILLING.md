# Phase 6: billing

Status: **Wave 2 (merged and wired) built.** Every track is merged on `feat/billing-connect`, the workers, plan limits, trials and discounts CLI are wired in, and the contract changes the tracks asked for are settled (section 7). Follows the rules in [CLAUDE.md](../CLAUDE.md) and the architecture in [ARCHITECTURE.md](ARCHITECTURE.md). The payment provider registry is [ADR 0006](adr/0006-payment-gateway-registry.md).

The schema is migration `00013_billing`, built from the load-tested draft v12 of the billing schema page.

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
| POST   | `/v1/workspace/billing/checkout-sessions/{sessionId}/test-complete`       | `billing.pay`   |
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

From Creem's official docs, read 2026-10-29. `platform/payment/creem.go` sets its `Capabilities` from these answers.

| Question            | Finding                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | Capability                                                                      |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| Embedded or hosted  | Both use the same URL. `POST /v1/checkouts` returns a `checkout_url` ([create checkout](https://docs.creem.io/api-reference/endpoint/create-checkout)). The page can open it, or embed it as an iframe with `https://www.creem.io/embed.js` (`Creem.openCheckout` / `Creem.mount`). There is no client secret: the URL itself is safe for the browser, and the browser's `onComplete` is UX only ([embedded checkout](https://docs.creem.io/features/checkout/embedded-checkout)).                                                                                                                                                                                           | `EmbeddedCheckout: false`; `Checkout.URL` is set                                |
| Native installments | None. Products are one-time or recurring, with no payment-plan or N-payment schedule ([create product](https://docs.creem.io/api-reference/endpoint/create-product)). "Split payments" are revenue splits between sellers, not installments ([split payments](https://docs.creem.io/features/split-payments)).                                                                                                                                                                                                                                                                                                                                                               | `Installments: false` → `app_link`; `ModeInstallments` returns `ErrUnsupported` |
| Subscriptions       | Creem renews them. Upgrade with `POST /v1/subscriptions/{id}/upgrade` and `update_behavior` `proration-charge-immediately` \| `proration-charge` \| `proration-none` ([upgrade](https://docs.creem.io/api-reference/endpoint/upgrade-subscription)). Cancel with `POST /v1/subscriptions/{id}/cancel`, `mode` `immediate` \| `scheduled` ([cancel](https://docs.creem.io/api-reference/endpoint/cancel-subscription)).`POST /v1/subscriptions/{id}/resume` undoes a `scheduled_cancel` ([resume](https://docs.creem.io/api-reference/endpoint/resume-subscription)). Trial length is set on the product, not per checkout ([trials](https://docs.creem.io/features/trials)). | `RenewsSubscriptions: true`                                                     |
| Webhook signature   | Header `creem-signature`: hex HMAC-SHA256 of the raw body, keyed with the webhook secret. No timestamp in the signature, so there is no replay window; `payment_events` dedupes on the event `id`, which automatic retries keep. Five attempts over 24 hours ([webhooks](https://docs.creem.io/code/webhooks)).                                                                                                                                                                                                                                                                                                                                                              | —                                                                               |
| Test mode           | A separate environment at `https://test-api.creem.io` with its own keys; using the wrong host fails ([test mode](https://docs.creem.io/getting-started/test-mode)). Test keys start `creem_test_`, live keys `creem_`; Creem's CLI picks the environment from that prefix ([CLI](https://docs.creem.io/code/cli)). The gateway does the same, so no extra setting is needed. Test card `4111 1111 1111 1111`.                                                                                                                                                                                                                                                                | —                                                                               |
| Refunds             | `POST /v1/refunds` takes only `transaction_id` and refunds it **in full**; the response is just a `status` (`pending`, `requiresAction`, `succeeded`, `failed`, `canceled`) ([refund a payment](https://docs.creem.io/api-reference/endpoint/refund-payment)). Partial refunds exist only in Creem's dashboard ([refunds and cancellations](https://docs.creem.io/features/subscriptions/refunds-and-cancellations)). The refund's ID arrives later in `refund.created`.                                                                                                                                                                                                     | `PartialRefunds: false`                                                         |
| Tax                 | Creem is the merchant of record and adds VAT/GST/sales tax by the customer's location ([merchant of record](https://docs.creem.io/merchant-of-record/what-is)). Each product is `tax_mode` `inclusive` or `exclusive`. Payloads carry `tax_amount` on the order and on each transaction, next to `sub_total`/`amount` and `amount_paid` (for example 1000 + 210 tax = 1210 paid); a discounted order adds `discount_amount` ([webhooks](https://docs.creem.io/code/webhooks)).                                                                                                                                                                                               | `MerchantOfRecord: true`                                                        |
| Discounts           | `POST /v1/discounts`: `type` `percentage` \| `fixed` (`amount` in cents plus `currency`), `duration` `once` \| `repeating` (`duration_in_months`) \| `forever`, and a **required** `applies_to_products` list ([create discount code](https://docs.creem.io/api-reference/endpoint/create-discount-code), [discounts](https://docs.creem.io/features/discounts)). A checkout takes `discount_code` to prefill it.                                                                                                                                                                                                                                                            | `Coupons: true`                                                                 |
| One-time amounts    | A checkout always names a `product_id`; `custom_price` (cents, 100 to 99,999,999) overrides a one-time product's price ([create checkout](https://docs.creem.io/api-reference/endpoint/create-checkout)).                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | —                                                                               |

How the gateway maps these:

- **Reference.** `CreateCheckout` sends our `checkout_sessions.id` as `request_id` and as `metadata.relay_reference`. Creem copies checkout metadata to the subscription, so `checkout.completed`, `subscription.*`, `refund.created` and `dispute.created` all carry it back as `Event.Reference`.
- **One-time payments** (`ModePayment`) need `ExternalPriceID` set to a one-time Creem product, and charge `AmountMinor` through `custom_price`. Relay has already taken any discount off, so no discount code is sent.
- **Payments.** `Payment.ExternalID` is the Creem transaction (`tran_…`), which is what `Refund` takes. `TotalMinor` is `amount_paid`, `TaxMinor` is `tax_amount`, `DiscountMinor` is `discount_amount`, and `SubtotalMinor = TotalMinor + DiscountMinor - TaxMinor`, so the invariant holds for both tax modes.
- **Coupons.** `CreateCoupon` creates a Creem discount with Relay's code and applies it to the store's active **recurring** products only, so the code can't be typed onto a one-time checkout that Relay has already discounted. `Coupon.ExternalCouponID` is the Creem code (what a checkout's `discount_code` takes); `ExternalPromotionCodeID` is the Creem discount ID.
- **Events.** `checkout.completed` → `checkout.completed`; `subscription.paid` → `payment.succeeded` (it also fires for the first payment, so invoices must dedupe on `external_payment_id`); `subscription.canceled` → `subscription.canceled`; the other `subscription.*` events → `subscription.updated`; `refund.created` → `refund.updated`; `dispute.created` → `dispute.opened`. Creem sends no checkout-expired, payment-failed or dispute-closed events: a failed renewal shows as `subscription.past_due` / `unpaid`. Everything else (credits, …) is `ignored`.
- **Statuses.** `scheduled_cancel` is `active` with `cancel_at_period_end`; `unpaid` and `paused` are `past_due`; `expired` is `canceled`.
- **Not reported.** Creem returns no proration amount from an upgrade (`ProrationMinor` stays 0), no checkout expiry, and no idempotency key support (the `request_id` is only echoed).

## 7. Wave 2 decisions

The contract changes and open questions the tracks reported, as settled when they were merged.

| Topic                         | Decision                                                                                                                                                                                                                                                                                                               |
| ----------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Paying a defaulted balance    | The `checkout_sessions` CHECK stays: `PaymentCheckoutInput.InstallmentID` is required, and a balance payment names the earliest open installment with the whole balance as the amount.                                                                                                                                 |
| Installment reminders         | The daily pass emails the billing contact (through `send_email`) a link to `/billing`, whose Pay now starts a fresh checkout. It no longer makes a checkout itself, since that would expire after `CheckoutTTL`.                                                                                                       |
| Provider-run schedules        | `Gateway.CancelSchedule` stops one. Billing calls it when an order is upgraded or defaults, in the same transaction, so a failure retries. A `subscription.canceled` for a finished schedule matches no subscription and is ignored, so the lifetime grant stays.                                                      |
| Creem                         | `CouponRequest.ExternalProductIDs` lists the provider's active recurring prices. Payment-mode checkouts send the price's provider product when it has one. `Capabilities.EmbeddableURL` (`embeddable_url` in the catalog) is true for Creem.                                                                           |
| Trials                        | A new workspace trials the catalog plan `TRIAL_PLAN` (default `growth`) for its `trial_days`, as a trial grant. `TRIAL_DAYS` is gone. No such plan, or a card-required trial, means the workspace starts on free. A workspace gets one trial: a card trial is skipped once it has had a subscription or a trial grant. |
| Free plan                     | No grant is free: the `free` plan row ([ADR 0008](adr/0008-plan-resource-catalog.md)), seeded with 1 member and 1 WhatsApp number (lowered from 5 members in Phase 7 so every paid plan gives more) and editable in the CRM.`workspaces.plan` defaults to `free` (migration `00014`), and workspaces without a grant were reset to it.                                                                      |
| Seats                         | With billing wired in, the plan's member cap is the seat limit; `workspaces.seat_limit` is only the fallback without billing. A plan with no member cap reports `limit: -1` and the web shows no meter.                                                                                                                |
| Testing checkouts             | `POST .../checkout-sessions/{sessionId}/test-complete` pays a checkout on the fake gateway through the signed-webhook path. It is 404 on any real provider; fake gateways are allowed only in development.                                                                                                             |
| Abandoned orders              | Kept (T4): the daily pass cancels a pending order and its installments after 7 days.                                                                                                                                                                                                                                   |
| Lost disputes                 | The grant the disputed invoice paid for (its subscription or order) is suspended. Support restores it once settled.                                                                                                                                                                                                    |
| Refund over the invoice total | Kept (T5): logged at ERROR and not recorded, for a person to check.                                                                                                                                                                                                                                                    |
| Provider-side price changes   | `subscription.updated` naming another of our provider prices moves the subscription to that price.                                                                                                                                                                                                                     |

## 8. Staff admin tools (temporary)

A stopgap so staff can build the catalog and fix a workspace's plan without SQL. It is its own
app, `apps/crm` ([ADR 0007](adr/0007-staff-crm-app.md); `task dev:crm`, port 3020), with tabs for
plans and prices, discount codes and workspaces. The API admits a signed-in, verified
account whose email is in `ADMIN_EMAILS`; anyone else gets 403 `staff_only`. Empty turns it off.
Replace it with a real back office (roles, an audit log) before there are several staff.

- **Catalog**: add a plan (a used code publishes its next version), prices, installment offers on
  one-time prices, and provider links (the provider's price or product ID). Rows are never edited:
  deactivate one and add its replacement. A price is on sale only once it has an active provider
  link.
- **Discount codes**: the same create, list and deactivate as `relay billing discounts`.
- **Workspaces**: list with the current grant; grant a plan by hand (`source = manual`, a reason
  is required, an end date is optional), end a manual grant or trial, or restore a suspended
  grant. A plan from a subscription or lifetime deal is refused (`grant_from_payment`): cancel
  that first, or billing would put it back on the next payment.

| Method | Path                                                                                  |
| ------ | ------------------------------------------------------------------------------------- |
| GET    | `/v1/admin/billing/catalog`                                                           |
| POST   | `/v1/admin/billing/plans`                                                             |
| POST   | `/v1/admin/billing/prices`                                                            |
| POST   | `/v1/admin/billing/installment-offers`                                                |
| POST   | `/v1/admin/billing/provider-prices`                                                   |
| POST   | `/v1/admin/billing/{plans,prices,installment-offers,provider-prices}/{id}/deactivate` |
| GET    | `/v1/admin/billing/discounts`                                                         |
| POST   | `/v1/admin/billing/discounts`                                                         |
| POST   | `/v1/admin/billing/discounts/{code}/deactivate`                                       |
| GET    | `/v1/admin/workspaces?search=`                                                        |
| POST   | `/v1/admin/workspaces/grant-plan`                                                     |
| POST   | `/v1/admin/workspaces/end-grant`                                                      |
| POST   | `/v1/admin/workspaces/restore-grant`                                                  |

Admin routes take the workspace in the body, never the URL or the `X-Workspace-ID` header: staff
are not members.

## 9. Review fixes

- **Retried and late payments complete.** A declined Stripe attempt no longer ends the checkout
  (`payment_intent.payment_failed` and a failed first invoice are ignored; the customer can retry).
  A session stays open as long as the provider's checkout (`Checkout.ExpiresAt`, 23 hours on
  Stripe). A payment that still lands on a failed or expired session completes it and takes its
  discount back, unless the code was used up meanwhile.
- **Downgrades wait for the period end.** The provider bills the lower price from the next
  period, and reports it at once; that report no longer moves the plan before `pending_change_at`.
- **Card trials start when the card is saved.** A $0 first invoice with a pending SetupIntent is
  ignored; `customer.subscription.updated` clearing `pending_setup_intent` on a trialing
  subscription completes the checkout as `trialing`. A provider update never makes an
  `incomplete` subscription live: only a completed checkout does.
- **A second payment for something already paid is refunded.** Two Pay windows for one
  installment, or a balance settled meanwhile: the payment gets its own invoice and a full refund
  (idempotency key `duplicate:<payment id>`), and the webhook succeeds.
