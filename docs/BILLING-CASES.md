# Billing cases

Every case Relay can face across payment, billing and plan limit enforcement, with its status as of 2026-11-07 (branch `feat/billing-checkout`). Every case was checked against the code and its tests on 2026-11-07; the marks below are the result.

| Mark | Meaning                                                      |
| ---- | ------------------------------------------------------------ |
| ✅   | Tested live with real Creem test payments, Cloudflare and R2 |
| 🔧   | Built: the code path or its tests exist, not run live        |
| ❓   | Unchecked: needs a live provider test to know                |
| ⚠️   | Known gap: missing or wrong today                            |

## Checkout and first payment

1. ✅ A new customer buys a monthly plan. The subscription, plan, invoice and webhooks were all correct.
2. 🔧 Buying a yearly plan. Same path as monthly; no end-to-end test.
3. ✅ Billing details are entered on the checkout page itself, then the embedded Creem checkout opens.
4. 🔧 The customer closes the checkout without paying; it can be reopened. An abandoned checkout expires after an hour (swept every 15 minutes). A late payment still completes it.
5. 🔧 The checkout script fails to load (10 second timeout), so Relay falls back to redirecting to Creem's page.
6. 🔧 A 100% discount makes a lifetime order free; it completes inside Relay with no charge. A 100% discount on a subscription still goes to Creem.
7. 🔧 A lifetime order below the provider's minimum charge is refused. Subscriptions have no minimum check.
8. ⚠️ Two checkouts are paid at the same time. The second subscription's event fails and retries (about 25 times, logging "refund it" each time), then the hourly stuck-events job logs it at ERROR. Nobody cancels the second Creem subscription, so it keeps renewing: staff must cancel it on Creem and refund by hand.
9. 🔧 The payment fails or the card is declined at checkout. Creem sends no event for it and keeps the checkout "pending", so its embed shows the decline and lets the customer retry. Relay asks Creem before expiring the session after an hour, so it only says "you haven't been charged" once Creem confirms it.
10. 🔧 The customer pays, but the success screen never shows. Activation depends only on the webhook, so the plan still activates.
11. ⚠️ Creem defaults the billing country to Bangladesh. Relay sends Creem no country or address at all; this is a Creem dashboard setting.

## Webhooks

12. ✅ The signature is checked and events are processed in the background. All four real webhooks returned 200.
13. ✅ The same payment arrives twice (checkout.completed and subscription.paid). Relay records one invoice, keyed by transaction ID.
14. 🔧 A replayed webhook is ignored (deduped by event ID).
15. 🔧 A webhook with a bad signature is rejected with 400 and nothing is stored.
16. ⚠️ Events arrive out of order. A `subscription.*` event processed before `checkout.completed` finds no subscription and is dropped, not retried. Harmless for a first purchase; a newer update is picked up by the hourly subscription check once the period ends (see 20).
17. 🔧 An event that's still unprocessed after 24 hours is logged at ERROR by an hourly job. It is reported again every hour, forever.
18. 🔧 Event payloads are deleted after 90 days.
19. 🔧 The webhook arrives for a workspace or price that was deleted. This can't happen: billing rows can't be deleted, and a deactivated price still activates a paid checkout.
20. 🔧 A webhook never arrives. Every 15 minutes Relay reads back from Creem each checkout still pending after 5 minutes, and applies a paid one exactly as its webhook would. Every hour it reads back each subscription whose period ended over an hour ago with no news, which catches lost renewal, past-due and cancel webhooks. A read-back is stored as a payment event, so the real webhook arriving later changes nothing. Stripe isn't read back: it retries webhooks for three days.

## Subscription lifecycle

21. ✅ Upgrade: it applies immediately, with a prorated charge ($50). The stored proration is Relay's estimate; Creem doesn't report what it charged.
22. 🔧 Downgrade at period end: the change waits and applies at renewal. If the renewal webhook is lost, the hourly subscription check applies it.
23. ⚠️ Downgrading while usage is above the new plan's limits, for example 8 seats on a 3-seat plan. Nothing checks or warns. After renewal, existing members stay and new invites are refused (see 82).
24. 🔧 Renewal payment succeeds: a new invoice and PDF. `subscription.paid` also syncs the subscription it carries (status, period, cancel flag), even when its transaction isn't expanded.
25. 🔧 Renewal payment fails: past due, a 7-day grace period, then the plan is suspended. A renewal paid after grace makes the subscription active again, and the suspended-plan alert names the real cause. Fixed 2026-11-07: before, the sweep suspended it again within 15 minutes.
26. ⚠️ No dunning email for a failed renewal. Only the in-app past-due alert with "Update card".
27. 🔧 Creem's `expired`, `unpaid` and `paused` statuses are treated as past due.
28. 🔧 Cancelling, and resuming before the period ends. A late `scheduled_cancel` processed after a resume sets the cancel flag back on.
29. 🔧 After cancelling, access ends when Creem's `subscription.canceled` webhook arrives, or, if it's lost, when the hourly subscription check finds it canceled after the period end.
30. 🔧 Switching between monthly and yearly. Monthly to yearly starts a new yearly period now, less the unused time; yearly to monthly waits for the period end. Fixed 2026-11-07: the dialog said yearly to monthly "starts now".
31. 🔧 Updating the card. "Update" (and "Update card" on the past-due alert) opens Creem's customer portal.
32. 🔧 A nightly job logs a warning for any workspace whose stored plan disagrees with its actual grant.

## Trials

33. 🔧 Growth has a 14-day trial with no card required. It's a Relay grant given when a workspace is created (`TRIAL_PLAN`, `plans.trial_days`), not a Creem product setting.
34. 🔧 The trial ends and the plan expires (a job runs every 15 minutes).
35. ⚠️ A workspace that already had a trial isn't offered another. Per person it isn't held: once a user's first workspace is activated, their next workspace gets a new trial.
36. 🔧 Buying during a trial charges the full first period now; the remaining trial days are lost.
37. 🔧 Reminder emails 4 days and 1 day before the trial ends (onboarding feature).

## Lifetime deals and installments

38. 🔧 A one-time lifetime purchase.
39. 🔧 An installment plan: due reminders, overdue, suspension, then default.
40. 🔧 A defaulted order resumes once it's paid.
41. 🔧 Buying a lifetime deal cancels the active subscription and credits its unused time. A past-due subscription is credited too, for a period it never paid.
42. 🔧 A duplicate installment payment still gets an invoice, and is refunded.
43. ⚠️ Refunding one installment marks its invoice refunded but doesn't lower what the order counts as paid, so the order can still reach lifetime.

## Discounts

44. 🔧 A coupon at checkout.
45. 🔧 The discount is reserved during checkout and released if the checkout expires.
46. 🔧 Creem discounts are recreated on Creem when a new provider price is linked.
47. 🔧 A limited-duration discount ends and the full price applies. No test covers it.
48. 🔧 A coupon that's used up or expired is refused at checkout start. Once reserved, the customer keeps the quoted price.

## Refunds and disputes

49. 🔧 Full and partial refunds update the invoice status.
50. 🔧 A refund can never exceed the invoice total.
51. 🔧 Refunding the latest invoice in full cancels the subscription.
52. 🔧 A chargeback marks the invoice disputed.
53. 🔧 A dispute won or lost. Stripe reports it. Creem has no dispute-closed event or dispute API, so staff record the outcome in the CRM's Disputes tab, with a reason kept as a payment event. Won puts the invoice back to paid; lost suspends the plan. ⚠️ A lost dispute doesn't cancel the subscription on the provider: staff must cancel it there.
54. ⚠️ No refund document (credit note) or refund email is sent.

## Invoices

55. ✅ Relay invoice numbers with no gaps (`RLY-2026-000001`). A replayed payment doesn't use up a number.
56. ✅ The PDF is rendered by Cloudflare, stored in R2 and emailed. This takes about 3 to 5 seconds after payment.
57. ✅ The download link goes straight to R2 and expires after 15 minutes.
58. ✅ Cloudflare's rate limit triggers an automatic retry.
59. ✅ Prorations and the first invoice name the right plan.
60. ⚠️ Seller details (`INVOICE_SELLER_*`) aren't set, so the PDF says only "Relay".
61. ⚠️ Every invoice is titled "Invoice", though Creem is the legal seller. Check with an accountant whether "Receipt" is right.
62. 🔧 If a render keeps failing, a sweep retries it every 15 minutes. An email that fails after the PDF was stored is never retried.
63. 🔧 The billing email changes after payment. The invoice and its email use the profile copied at payment; later invoices use the new address.

## Tax

64. ⚠️ Tax was $0 in testing. Relay records the tax Creem reports and has no Bangladesh default of its own.
65. ⚠️ An EU customer with a VAT number (reverse charge). Relay doesn't send the VAT number to Creem, and the invoice has no reverse-charge wording.
66. 🔧 The tax ID is printed on the invoice.

## Catalog and CRM

67. ✅ Creating a plan in the CRM creates the Creem product and prices.
68. ⚠️ A half-finished plan creation resumes only while the CRM editor stays open. After a reload, publishing again creates a new plan version and can leave an orphaned Creem product.
69. 🔧 Prices never change once sold; a new version is published instead.
70. 🔧 Deactivating a plan that customers are still on: they keep it, and nobody new can buy it or switch to it. Fixed 2026-11-07: before, its prices could still be bought by ID.
71. 🔧 Staff manual grants and per-workspace limit changes.
72. ⚠️ USD only.

## Limit enforcement

73. 🔧 Seats: invites are blocked at the cap, and pending invites count.
74. 🔧 WhatsApp numbers are capped.
75. ⚠️ Contacts are capped, including CSV imports, which add only as many as fit. Contacts created by inbound messages and by Shopify/WooCommerce orders skip the cap.
76. 🔧 Broadcasts per month are enforced, and the counter is safe under simultaneous sends.
77. 🔧 AI replies per month: the quota is ready by key, but the feature doesn't exist yet.
78. ⚠️ Chatbot flows and API calls are sold on plans, but nothing checks them. The usage card shows an "API calls 0 / 10,000" meter as if it were tracked.
79. ⚠️ Shopify & WooCommerce, team controls and priority support are sold, but not checked. Only Automations is checked.
80. 🔧 A failed send gives its unit back to the month it was taken from. A broadcast that fails because its segment was deleted keeps its unit.
81. ⚠️ The usage card says "Nov 7 – Dec 7 · resets in 30 days", but monthly limits reset on the 1st of each calendar month (UTC). Broadcasts reset Dec 1, not Dec 7. The API already returns the right period.
82. 🔧 Over a limit after a downgrade, existing items are kept and only new ones are blocked. Automations are the exception: every automations route answers 402, so they can't be viewed or turned off.
83. 🔧 After the 7-day grace, a suspended plan uses the free plan's limits.

## Workspaces and accounts

84. 🔧 Deleting a workspace that has an active subscription. There's no delete endpoint yet; the billing guard is built but unused.
85. ⚠️ There's no ownership transfer, so the owner can never leave. Billing belongs to the workspace.
86. ❓ One person owns several workspaces, each with its own plan. If Creem reuses its customer for the same email, the second workspace's checkout fails on a unique key (a 500, or a payment that never activates). Needs a live test.
87. 🔧 Team members without billing permission see a locked billing page, and the API refuses them. Gaps: `/billing/plans` shows a generic error instead, and a role with only `billing.pay` can't load the overview.

## Security and abuse

88. 🔧 The workspace comes from a header, and every billing query is scoped to it.
89. 🔧 Secrets are never selected outside their own queries.
90. ⚠️ Repeat trial abuse with new workspaces (see 35).
91. ❓ Card testing on checkout. Creem should handle it; Relay has only a global per-IP rate limit.

## Operations

92. 🔧 Webhook processing and invoice PDFs retry on failure.
93. ⚠️ Cloudflare's free plan rate-limits bursts. Consider the paid plan for production.
94. ⚠️ Creem's test and live keys mixed up when going to production. Nothing stops a test key with `APP_ENV=production`, test product IDs break live checkout, and test-mode webhooks would take real invoice numbers.
95. ⚠️ Running Creem and Stripe together works. Moving existing customers off Creem has no path: Creem must stay enabled until its subscriptions end.

## Not in the list

These bugs were found during the 2026-11-07 check:

- ⚠️ After a provider error at checkout, clicking "Continue to payment" again shows "The payment didn't go through", because the page reuses the same idempotency key.
- ⚠️ The daily installment pass stalls once 200 or more abandoned installment checkouts are under 7 days old.
- ⚠️ An overpaid installment from a provider-run schedule retries for weeks and is never refunded.
- ⚠️ Resending an expired invite skips the seat check, which can push the workspace over its cap and block every acceptance.
- ⚠️ The WhatsApp numbers meter leaves out disconnected numbers, but the cap counts them.
- ⚠️ A free ($0) order's PDF says "Paid with card via Creem".
- ⚠️ Subscriptions with `renewal_mode='app'` are never renewed: nothing runs them.
- ⚠️ A cancel webhook leaves the pending plan change and the cancel flag set on the closed subscription.
- ⚠️ A checkout recovered by read-back takes the time of the read as its payment time, up to 20 minutes late. Renewals keep the transaction's own time.

## Suggested order

1. Item 81, the reset date: the card tells customers the wrong date.
2. Items 78 and 79: plans sell features nothing enforces.
3. Item 8: the second subscription keeps renewing on Creem.
4. Item 86: test one email on two workspaces against live Creem.
5. Items 23 and 26: downgrade warnings and failed-payment emails.
6. The checkout retry bug: what customers see when a payment step fails.
