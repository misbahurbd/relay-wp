# 0006. Payment gateway registry

- **Status:** accepted
- **Date:** 2026-10-29

## Context

Billing ([PHASE-6-BILLING.md](../PHASE-6-BILLING.md)) launches with two payment providers, Stripe and Creem, and the schema already allows PayPal and SSLCommerz later. Every provider does the same jobs:

1. Start a checkout for a subscription, a one-time payment or an installment schedule.
2. Change or cancel a subscription it renews, and refund a payment.
3. Mirror a Relay discount as its own coupon, for subscriptions it renews.
4. Verify a webhook and turn it into one normalised event.

What differs is everything underneath: Stripe uses Elements with a client secret and signs webhooks with `Stripe-Signature`; Creem may only offer hosted checkout and signs with its own HMAC header; SSLCommerz has no subscriptions and needs a validation API call after its IPN.

The implementation is chosen per request: by the `{provider}` path value on `POST /v1/webhooks/payments/{provider}`, and by the `provider` column on a checkout session, subscription or order. A provider that is not configured must not be offered at checkout, and its webhook route answers `404`.

`CLAUDE.md` section 2 allows a factory only when all three of its conditions hold. All three do here:

1. There are two real implementations today (Stripe and Creem), plus a test fake.
2. The implementation is selected by config (`PAYMENT_PROVIDERS`) and then by a key (the provider name).
3. They are built once at startup.

## Decision

- `platform/payment/payment.go` declares the `Gateway` interface, `Capabilities`, the normalised `Event` types, the `Registry` and `New(cfg, dev)`. It is infrastructure: it knows nothing about plans, grants or workspaces.
- `New` reads `PAYMENT_PROVIDERS` (a comma-separated list) and builds each listed provider with a plain `switch`, once at startup. One shared `http.Client` is passed to every gateway.
- An unknown provider, a duplicate, or a listed provider with missing keys fails at startup. A provider that is not listed is not in the registry: checkout never offers it and its webhook route answers `404 payment_provider_not_available`.
- One file per implementation: `stripe.go`, `creem.go`, and `fake.go`. `paypal.go` and `sslcommerz.go` come later with one `case` each.
- `<provider>:fake` (for example `stripe:fake`) registers the fake gateway under that provider's name, so local development and flow tests write the same rows production does. It is refused outside development.
- `internal/app` builds the registry and passes it to the billing service. Handlers and services look a gateway up by name; they never build one.
- Provider secrets (API keys, webhook secrets) live only in config and inside the gateway. No table stores them and no event or log line carries them.

## Consequences

- Adding PayPal means one file and one `case`. The billing rules (grants, installments, discounts, proration) stay in `features/billing` and are written once.
- Each provider's quirks stay in its own file: Stripe coupons don't apply to PaymentIntents, Creem is the merchant of record, SSLCommerz needs a second validation call. `Capabilities` tells the billing service what a provider can do, so the service never switches on a provider name.
- The fake gateway lets every billing test, and local development, run with no network and no keys.
- What to watch: keep the interface small. A need only one provider has belongs in that provider's file, or in a `Capabilities` flag, not in a new method for everyone.
