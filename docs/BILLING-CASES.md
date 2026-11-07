# Billing cases

Every case Relay can face across payment, billing and plan limit enforcement, with its status as of 2026-11-07 (branch `feat/billing-checkout`). Every mark was checked against the code at that commit.

| Mark | Meaning                                                      |
| ---- | ------------------------------------------------------------ |
| ✅   | Tested live with real Creem test payments, Cloudflare and R2 |
| 🔧   | Built: the code path or its tests exist, not run live        |
| ❓   | Unchecked: needs a live provider test to know                |
| ⚠️   | Known gap: missing or wrong today                            |

Each case ends with `Test:` and the Go test functions that prove it, or `none` when no test covers it, or `pending` when the fix is still being built. `TestBillingCasesDoc` in `apps/api/internal/features/billing/service/billing_cases_doc_test.go` fails when a ✅ or 🔧 case has no `Test:` ending, or when a name after `Test:` isn't a test function under `apps/api`. Run it with `cd apps/api && go test ./internal/features/billing/service/ -run TestBillingCasesDoc`.

## Checkout and first payment

1. ✅ A new customer buys a monthly plan. The subscription, plan, invoice and webhooks were all correct. Test: TestSubscribeActivatesSubscriptionAndGrant
2. 🔧 Buying a yearly plan. Same path as monthly; no end-to-end test. Test: none
3. ✅ Billing details are entered on the checkout page itself, then the embedded Creem checkout opens. Test: TestProfileUpdateAndBillTo
4. 🔧 The customer closes the checkout without paying; it can be reopened. An abandoned checkout expires after an hour (swept every 15 minutes). A late payment still completes it, and the expired checkout page says the payment may still arrive. Test: TestRetryResumesAHalfOpenCheckout, TestExpiredCheckoutReleasesDiscount, TestLatePaymentCompletesAnExpiredCheckout
5. 🔧 The checkout script fails to load (10 second timeout), so Relay falls back to redirecting to Creem's page. Test: none
6. 🔧 A 100% discount makes a lifetime order free; it completes inside Relay with no charge, and its invoice says nothing was charged. A 100% discount on a subscription still goes to Creem. A card trial's first invoice says the same. Test: TestFreeOrderCompletesWithoutTheProvider, TestFreeOrderInvoiceSaysNothingWasCharged, TestCardTrialInvoiceSaysNothingWasCharged
7. 🔧 A lifetime order below the provider's minimum charge is refused. Subscriptions have no minimum check. Test: TestOrderUnderProviderMinimumIsRefused
8. 🔧 Two checkouts are paid at the same time. A new subscription checkout expires the workspace's older open ones. If the second is paid anyway, Relay cancels its provider subscription now, refunds the payment once (the refund is recorded in the same transaction and sent by a job), keeps the payment on a canceled subscription's invoice marked Refunded with no receipt email, fails the session and logs at ERROR. The webhook applies cleanly, and a replay or the read-back of the same checkout changes nothing. If the provider refuses the cancel for good, Relay records it and asks again every hour (see Known limits). Test: TestDuplicateSubscriptionCheckoutIsRefunded, TestDuplicateSubscriptionCheckoutRefundsOnce, TestRefundedDuplicateGetsNoReceipt, TestRefusedDuplicateCancelIsRetriedWithoutAStoredID, TestRefusedCancelWithoutAnIDStays, TestSubscriptionAndLifetimeCheckoutsDoNotOverlap
9. 🔧 The payment fails or the card is declined at checkout. Creem sends no event for it and keeps the checkout "pending", so its embed shows the decline and lets the customer retry. Relay asks Creem before expiring the session after an hour, so it only says "you haven't been charged" once Creem confirms it. After a provider error at checkout, clicking "Continue to payment" again reuses the idempotency key and shows "The payment didn't go through"; the fix is in progress. Test: TestProviderFailureFailsTheCheckout, TestCheckoutIsNotExpiredWhileTheProviderCantBeAsked, pending
10. 🔧 The customer pays, but the success screen never shows. Activation depends only on the webhook, so the plan still activates. Test: TestCompleteTestCheckoutGoesThroughTheWebhook, TestPaidCheckoutWithoutItsWebhookCompletes
11. ⚠️ Creem defaults the billing country to Bangladesh. Relay sends Creem no country or address at all; this is a Creem dashboard setting. Test: none

## Webhooks

12. ✅ The signature is checked and events are processed in the background. All four real webhooks returned 200. Test: TestReceiveWebhookStoresAndEnqueues, TestCreemWebhookRejectsBadSignatures
13. ✅ The same payment arrives twice (checkout.completed and subscription.paid). Relay records one invoice, keyed by transaction ID. Test: TestCreemCheckoutCompletedNamesTheTransactionSubscriptionPaidUses, TestSameEventTwiceChangesNothing, TestCompleteCheckoutTwiceIsANoOp
14. 🔧 A replayed webhook is ignored (deduped by event ID). Test: TestReceiveWebhookReplayIsANoOp
15. 🔧 A webhook with a bad signature is rejected with 400 and nothing is stored. Test: TestReceiveWebhookBadSignatureStoresNothing
16. 🔧 Events arrive out of order. A subscription, payment, refund or dispute event that names something Relay hasn't stored yet fails and is retried until the event is 24 hours old, then logged at WARN and dropped. An event with no workspace reference retries for the same 24 hours. A first purchase therefore settles once `checkout.completed` lands. Test: TestOutOfOrderEventsRetryUntilOld, TestUnmatchedEventRetriesThenGivesUp
17. 🔧 An event still unprocessed after 24 hours is logged at ERROR once by the hourly job, not again on each run. Test: TestStuckEventIsReportedOnce
18. 🔧 Event payloads are nulled after 90 days (the row stays); a job purges every 6 hours. Test: TestPurgeEventPayloads
19. 🔧 The webhook arrives for a workspace or price that was deleted. This can't happen: billing rows can't be deleted, and a deactivated price still activates a paid checkout. Test: TestDeactivatedPlanStopsSellingButKeepsRenewing
20. 🔧 A webhook never arrives. Every 15 minutes Relay reads back from Creem each checkout still pending after 5 minutes, and applies a paid one exactly as its webhook would. A checkout Creem refuses to read back waits quietly until it expires, then expires with one error log and releases its discount. Every hour Relay reads back each subscription whose period ended over an hour ago with no news, which catches lost renewal, past-due and cancel webhooks. A subscription Creem leaves unchanged is read again after 2, 4, 8 and 16 hours, then daily. A read-back is stored as a payment event, so the real webhook arriving later is processed again and changes nothing. Stripe isn't read back: it retries webhooks for three days. Test: TestPaidCheckoutWithoutItsWebhookCompletes, TestLostCancelWebhookIsReconciled, TestCheckoutTheProviderRefusesToReadBackExpires, TestUnchangedOverdueSubscriptionIsReadBackLessOften

## Subscription lifecycle

21. ✅ Upgrade: it applies immediately, with a prorated charge ($50). The stored proration is Relay's estimate; Creem doesn't report what it charged. The upgrade's invoice names the new plan even when its webhook lands first. A second change from the same workspace while one runs is refused with 409 `change_in_progress` after 2 seconds, and the provider call stops after 15 seconds. Test: TestUpgradeAppliesNowWithProration, TestUpgradeInvoiceNamesTheNewPlanWhenItsWebhookWins, TestSecondPlanChangeIsRefusedWhileOneIsInProgress, TestPlanChangeGivesUpOnAProviderThatNeverAnswers
22. 🔧 Downgrade at period end: the change waits and applies at renewal. If the renewal webhook is lost, the hourly subscription check applies it. Test: TestDowngradeWaitsForPeriodEnd, TestCancelPendingChange, TestInvoiceDocumentNamesTheDowngradeARenewalStarts
23. 🔧 Downgrading while usage is above the new plan's limits, for example 8 seats on a 3-seat plan. The change dialog lists each counted limit the workspace is past ("You have 8 team seats; Starter allows 3.") and says nothing is removed. The API doesn't check, and the change is allowed. After renewal, existing members stay and new invites are refused (see 82). Test: none
24. 🔧 Renewal payment succeeds: a new invoice and PDF. `subscription.paid` also syncs the subscription it carries (status, period, cancel flag), even when its transaction isn't expanded. Test: TestRenewalPaymentSyncsTheSubscriptionItCarries, TestPaidRenewalClearsPastDueAndSavesThePeriod, TestCreemSubscriptionPaidWithoutAnExpandedTransaction
25. 🔧 Renewal payment fails: past due, a 7-day grace period, then the plan is suspended. A renewal paid after grace makes the subscription active again, and the suspended-plan alert names the real cause. The sweep also adds past due to a grant already suspended for another cause, so lifting that cause doesn't give back a plan whose renewal is unpaid. Test: TestPastDueSweepSuspendsAfterGrace, TestSyncSubscriptionPastDueSuspendsAfterGrace, TestRenewalPaidAfterGraceStaysActive, TestPaidRenewalAfterGraceRestoresThePlan, TestPastDueSweepAddsPastDueToAGrantSuspendedForAnotherCause
26. ⚠️ No dunning email for a failed renewal. Only the in-app past-due alert with "Update card". Test: none
27. 🔧 Creem's `expired`, `unpaid` and `paused` statuses are treated as past due. Test: TestCreemWebhookSubscriptionExpiredIsPastDue
28. 🔧 Cancelling, and resuming before the period ends. A late `scheduled_cancel` processed after a resume sets the cancel flag back on. Test: TestCancelAtPeriodEndAndResume
29. 🔧 After cancelling, access ends when Creem's `subscription.canceled` webhook arrives, or, if it's lost, when the hourly subscription check finds it canceled after the period end. Test: TestLostCancelWebhookIsReconciled, TestCancelNowEndsTheGrant
30. 🔧 Switching between monthly and yearly. Monthly to yearly starts a new yearly period now, less the unused time, and the dialog dates it by the quote's effective date; yearly to monthly waits for the period end. An interval switch isn't a proration, so refunding the new yearly invoice in full ends the plan and refunding the old monthly one doesn't. Test: TestQuoteChangeIntervals, TestRefundAfterAnIntervalSwitchFollowsTheYearlyPeriod
31. 🔧 Updating the card. "Update" (and "Update card" on the past-due alert) opens Creem's customer portal. Test: TestPortalSessionNeedsBillingPay, TestCreemPortalURL, TestStripePortalURL
32. 🔧 A job logs a warning for any workspace whose stored plan disagrees with its actual grant. It runs every 24 hours from boot. Test: TestCheckPlanCacheDrift

## Trials

33. 🔧 Growth has a 14-day trial with no card required. It's a Relay grant given when a workspace is created (`TRIAL_PLAN`, `plans.trial_days`), not a Creem product setting. Test: TestCardlessTrialStartsAndExpires, TestStartTrialSkipsPlansWithoutCardlessTrial
34. 🔧 The trial ends and the plan expires (a job runs every 15 minutes). Test: TestCardlessTrialStartsAndExpires
35. 🔧 One card-less trial per user account, counted over the workspaces the user created. A workspace that already had a trial isn't offered another. Today the count also includes workspaces handed to the user by an ownership transfer; the fix is in progress. Card trials stay per workspace. Test: TestOneCardlessTrialPerOwner, pending
36. 🔧 Buying during a trial charges the full first period now; the remaining trial days are lost. Test: none
37. 🔧 Reminder emails 4 days and 1 day before the trial ends (onboarding feature). Test: TestOnboardingReminders

## Lifetime deals and installments

38. 🔧 A one-time lifetime purchase. Test: TestLifetimeDealLifecycle
39. 🔧 An installment plan: due reminders, overdue, suspension, then default. The hourly pass pages by due date and ID, so young pending orders no longer hold up the installments after them. Test: TestLifetimeDealLifecycle, TestRunInstallmentsDue, TestRunInstallmentsDueCancelsAbandonedOrders, TestRunInstallmentsDueReadsPastYoungPendingOrders, TestDefaultStopsTheProviderSchedule
40. 🔧 A defaulted order resumes once the balance is paid. Test: TestLifetimeDealLifecycle
41. 🔧 Buying a lifetime deal cancels the active subscription and credits its unused time. A past-due subscription gets no credit, since its period was never paid. A lifetime deal paid after a subscription went live that the quote didn't credit cancels that subscription and refunds its last payment in full. The order keeps the credit it was quoted. Test: TestCancelForLTDCreditsUnusedTime, TestPastDueSubscriptionHasNoCredit, TestLifetimeDealCancelsTheSubscriptionLiveAtPayment
42. 🔧 A duplicate installment payment still gets a numbered invoice, so numbers stay gapless, and is refunded in the transaction that records it. The invoice says Refunded and is emailed to nobody. An installment a provider schedule charges after the order is paid off is recorded the same way and refunded in full; the order stays as it was. Test: TestPayingAnInstallmentTwiceRefundsTheSecond, TestRefundedDuplicateInstallmentGetsNoReceipt, TestOverpaidProviderInstallmentIsRefunded
43. 🔧 Refunding an installment in full makes what that payment paid owed again from today: the installment, plus every installment a balance payment settled. The hourly pass marks it due and emails a reminder (also on a provider schedule, which won't charge it again), and grace starts over. `paid_in_full_at` clears when a paid order goes back to installments. Provider charges go to the next installment the provider hasn't had refunded. A partial refund changes nothing. Test: TestRefundingAnInstallmentReopensIt, TestRefundedInstallmentGetsAFreshDueDateAndReminder, TestRefundedResumeBalanceReopensWhatItSettled, TestRefundedProviderInstallmentIsPaidWithPayNow

## Discounts

44. 🔧 A coupon at checkout. Test: TestCheckoutDiscountIsRedeemedOnPayment, TestApplyDiscountAmounts
45. 🔧 The discount is reserved during checkout and released if the checkout expires. Test: TestExpiredCheckoutReleasesDiscount, TestReserveDiscountHoldsTheCapUnderConcurrency, TestReserveDiscountHoldsPerWorkspaceLimitUnderConcurrency
46. 🔧 Creem discounts are mirrored on Creem at the first checkout that needs one, not when a provider price is linked. The mirror carries Relay's expiry, the uses left and the plan's products. A new recurring Creem product retires the mirrors and deletes them on Creem; the next use mirrors the code again under a new suffix. Creem's effect on running subscriptions when a coupon is deleted isn't documented (see Known limits). Test: TestNewRecurringProductRemirrorsProductScopedCoupons, TestCreemCouponCarriesRelaysLimits, TestProviderCouponKeepsRelaysLimits, TestCreemDeleteCoupon
47. 🔧 A limited-duration discount ends and the full price applies. No test covers it. Test: none
48. 🔧 A coupon that's used up or expired is refused at checkout start. Once reserved, the customer keeps the quoted price. Test: TestApplyDiscountRejections

## Refunds and disputes

49. 🔧 Full and partial refunds update the invoice status. Relay records a refund it asks for (and queues its job) in the transaction that records the payment, then the job calls the provider once. A refund webhook claims a requested refund only of the same amount, or one the provider already took on the same invoice when the amounts differ. A refund made in the provider's dashboard is its own refund. Creem refunds only in full: a partly refunded transaction is refused for good, failed and logged for a person to finish in Creem's dashboard. A refund already done at the provider counts as done. A Stripe 409 is retried. Test: TestRefundsUpdateInvoiceAndNeverExceedTotal, TestRequestedRefundIsNamedByItsWebhook, TestDashboardRefundNeverClaimsARequestedRefund, TestWebhookOfAnotherAmountNamesTheSentRefund, TestRefusedRefundFailsOnce, TestCreemRefund, TestCreemRefundOfARefundedTransactionIsDone, TestStripeRefundOfARefundedChargeIsDone, TestStripeErrorsMapToGatewayErrors
50. 🔧 A refund can never exceed the invoice total. Test: TestRefundsUpdateInvoiceAndNeverExceedTotal, TestConcurrentRefundsNeverExceedTotal
51. 🔧 Refunding the newest paid period invoice in full cancels the subscription. A failed renewal's invoice is skipped, and a plan change's proration isn't a period invoice. Test: TestRefundingTheLatestSubscriptionInvoiceCancelsIt, TestRefundingThePaidPeriodIgnoresAFailedRenewal, TestRefundingAnUnmarkedProrationKeepsThePlan
52. 🔧 A chargeback marks the invoice disputed. Test: TestDisputes
53. 🔧 A dispute won or lost. Stripe reports it. Creem has no dispute-closed event or dispute API, so staff record the outcome in the CRM's Disputes tab, with a reason kept as a payment event. Won puts the invoice back to paid (or refunded, by its refunds). Lost moves the invoice to `dispute_lost` for good: it leaves the open-dispute list, staff can't close it again, and Relay cancels the subscription at the provider now or stops the order's installment schedule. The plan is suspended for the dispute, a cause only staff lift, and it stays suspended through renewals and paid installments. Staff can't restore a plan whose subscription ended. The customer subscribes again, or staff grant a plan by hand. Test: TestDisputes, TestStaffClosesDisputes, TestLostDisputeIsFinal, TestLostDisputeCancelsTheSubscription, TestLostDisputeStopsTheInstallmentSchedule, TestLostDisputeOutlivesAMissedInstallment, TestLostDisputeSuspensionSurvivesRenewal, TestStaffCannotRestoreAPlanWhoseSubscriptionEnded, TestRefundKeepsAConcurrentDispute
54. ⚠️ No refund document (credit note) or refund email is sent. Test: none

## Invoices

55. ✅ Relay invoice numbers with no gaps (`RLY-2026-000001`). A replayed payment doesn't use up a number. Test: TestPaidInvoicesAreNumberedPerYearWithoutGaps, TestARolledBackPaymentGivesItsNumberBack
56. ✅ The PDF is rendered by Cloudflare, stored in R2 and emailed. This takes about 3 to 5 seconds after payment. Test: TestInvoiceDocumentRendersAndEmailsOnce
57. ✅ The download link goes straight to R2 and expires after 15 minutes. Test: TestS3SignedURL
58. ✅ Cloudflare's rate limit triggers an automatic retry. Test: TestInvoiceDocumentRenderFailureRetries
59. ✅ Prorations and the first invoice name the right plan. An invoice stores the plan price paid for, and the PDF renders from it. Test: TestInvoiceDocumentNamesThePricePaidFor, TestInvoiceDocumentTellsProrationsFromDiscounts, TestInvoiceDescriptions
60. ⚠️ Seller details (`INVOICE_SELLER_*`) aren't set, so the PDF says only "Relay". Test: none
61. ⚠️ Every invoice is titled "Invoice", though Creem is the legal seller. Check with an accountant whether "Receipt" is right. Test: none
62. 🔧 If a render keeps failing, a sweep retries it every 15 minutes with a per-invoice backoff, up to 12 claims (about five days), and logs an ERROR on the last. The sweep also re-sends a receipt email that was never sent. Test: TestSweepRequeuesInvoicesStillWithoutAPDF, TestSweepRequeuesAnInvoiceWhoseEmailWasNeverSent, TestSweepBacksOffAnInvoiceThatKeepsFailing, TestSweepGivesUpOnAnInvoiceThatNeverDelivers
63. 🔧 The billing email changes after payment. The invoice and its email use the profile copied at payment; later invoices use the new address. Test: TestProfileUpdateAndBillTo

## Tax

64. ⚠️ Tax was $0 in testing. Relay records the tax Creem reports and has no Bangladesh default of its own. Test: none
65. ⚠️ An EU customer with a VAT number (reverse charge). Relay doesn't send the VAT number to Creem, and the invoice has no reverse-charge wording. Test: none
66. 🔧 The tax ID is printed on the invoice. Test: none

## Catalog and CRM

67. ✅ Creating a plan in the CRM creates the Creem product and prices. Test: TestAdminBuildsASellableCatalog, TestAdminCreatesProviderPricesOnTheProvider, TestCreemCreatePrice
68. 🔧 A half-finished plan creation resumes after a reload: the CRM editor saves the publish and the IDs made so far before its first call, and publishing again finds what the server already created. Closing the editor drops the draft, and publishing again then creates a new plan version. A provider link that failed after the provider made its product reuses that product on retry. The editor's reload path has no test. Test: TestPlanPublishRetryCreatesNothingTwice, TestFailedProviderLinkReusesItsProduct, TestAdminProviderPriceFailuresLeaveNoLink
69. 🔧 Prices never change once sold; a new version is published instead. A database trigger refuses any change to a sold price's terms. Its test only checks that the update fails, not which error. Test: TestPlanPricesNeverChange
70. 🔧 Deactivating a plan that customers are still on: they keep it, and nobody new can buy it or switch to it. A scheduled downgrade to a plan deactivated later still applies, since the customer agreed before. Test: TestDeactivatedPlanIsOffSale, TestDeactivatedPlanStopsSellingButKeepsRenewing
71. 🔧 Staff manual grants and per-workspace limit changes. Test: TestAdminGrantsAPlanByHand, TestPlanLimitsAreLiveAndOverridable, TestSetGrantOverridesIsScopedToTheWorkspace
72. 🔧 USD only. Creating a price in any other currency is refused with `unsupported_currency` (400). Creating the same slot and amount again returns the existing price. Test: TestAdminPriceMustBeUSD

## Limit enforcement

73. 🔧 Seats: invites are blocked at the cap, pending invites count, and resending an expired invite checks the cap too. Test: TestMemberLimit, TestOverMemberLimit, TestResendingAnExpiredInviteChecksTheMemberLimit
74. 🔧 WhatsApp numbers are capped, and the cap counts every number. The usage card's meter leaves out disconnected numbers; the fix is in progress. Test: TestNumberLimit, TestOverNumberLimit, pending
75. 🔧 Contacts are capped for manual adds, for CSV imports (which add only as many as fit), and for Shopify/WooCommerce orders and carts. A store event over the cap is skipped and recorded as skipped for the contact limit. Contacts created by inbound messages are never refused, so no customer message is lost. The fix in progress: an abandoned-cart reminder is blocked for good when the cap skipped the cart's first event. Test: TestContactLimitStopsAddsAndImports, TestStoreEventsRespectTheContactLimit, pending
76. 🔧 Broadcasts per month are enforced, and the counter is safe under simultaneous sends. Test: TestConsumeStopsAtTheCap, TestConsumeIsAtomicUnderConcurrency, TestBroadcastPlanLimitScheduleAndCancel
77. 🔧 AI replies per month: the quota is ready by key, but the feature doesn't exist yet. Its meter stays on the usage card. Test: TestAIRepliesQuotaWorksByKey
78. 🔧 Chatbot flows and API calls are sold on plans. Their meters, and the AI replies meter, stay on the usage card because the features are being built. API-key requests will count against the monthly API-calls quota; the fix is in progress. Test: pending
79. 🔧 Shopify & WooCommerce (`ecommerce`) and "Roles, audit log & SSO" (`team_controls`) are sold but not checked today; only Automations is. They will be gated by plan. Priority support is a human promise with no code. Test: TestAutomationsNeedThePlan, pending
80. 🔧 A failed send gives its unit back to the month it was taken from. A broadcast that fails because its segment was deleted keeps its unit. Test: TestReleaseGivesBackToTheMonthConsumed, TestStalledBroadcastGivesItsUnitBackOnlyWhenNothingWent
81. 🔧 The usage card shows the usage API's own period, the calendar month in UTC, so Broadcasts reset on the 1st. The web change has no test. Test: TestUsageListsEveryMonthlyResource
82. 🔧 Over a limit after a downgrade, existing items are kept and only new ones are blocked. Automations are the exception today: every automations route answers 402, so they can't be viewed or turned off. Only creating and enabling will be gated; the fix is in progress. Test: TestAutomationsNeedThePlan, pending
83. 🔧 After the 7-day grace, a suspended plan uses the free plan's limits. Test: TestGrantChangesKeepThePlanCache

## Workspaces and accounts

84. 🔧 Deleting a workspace that has an active subscription. There's no delete endpoint yet; the billing guard is built but unused. Test: TestCheckDeletion, TestDeletionCheck
85. 🔧 Ownership transfer exists. Only the owner can transfer; the actor is re-checked under the row lock, so two transfers leave one owner, and a leave or a stale request from a former owner can't touch the new one. Billing belongs to the workspace and stays with it. The CRM shows the creator as owner after a transfer; the fix is in progress. Test: TestTransferOwnership, TestConcurrentTransfersLeaveOneOwner, TestFormerOwnerCantChangeTheNewOwner, TestNewOwnerCantLeaveOnAStaleRequest, pending
86. 🔧 One person owns several workspaces, each with its own plan. Creem reuses its customer for one email, so the unique key on provider and customer ID was dropped and workspaces may share a provider customer. Not run against live Creem. Test: TestWorkspacesMayShareAProviderCustomer
87. 🔧 Team members without billing permission see a locked billing page, and the API refuses them. Billing writes (checkout, plan changes, billing details, the card portal) need a browser session, not an API key. A role with only `billing.pay` gets the lock screen on both billing pages and can't buy from the UI; the fix is in progress. Test: TestSubscriptionReadsNeedBillingView, TestSubscriptionChangesNeedBillingPay, TestCheckoutPermissions, TestAPIKeysCantPay, pending

## Security and abuse

88. 🔧 The workspace comes from a header, and every billing query is scoped to it. Team and member-management writes need a session, not an API key, so a key made by the owner can't transfer ownership. Test: TestWorkspaceRoutesNeedAMember, TestSetGrantOverridesIsScopedToTheWorkspace, TestAPIKeysCantManageTheTeam
89. 🔧 Secrets are never selected outside their own queries. No test scans the queries; the checks in CLAUDE.md (no `SELECT *`, no hash in a model or log) are by grep. Tests cover that webhook payloads and secrets stay out of logs and job args. Test: TestPayloadIsNeverLogged, TestProcessPaymentEventArgsLeaveThePayloadOut, TestLogValuesHideSecrets
90. 🔧 Repeat trial abuse with new workspaces is limited to one card-less trial per user account (see 35). Test: TestOneCardlessTrialPerOwner, pending
91. 🔧 Card testing on checkout. Creating checkout sessions is limited to 20 an hour per IP, on top of the global per-IP rate limit. Creem's own card-testing defence is unchecked. Test: TestCheckoutSessionsAreRateLimitedPerIP

## Operations

92. 🔧 Webhook processing and invoice PDFs retry on failure. Test: TestHandlerErrorRollsBack, TestUnmatchedEventRetriesThenGivesUp, TestInvoiceDocumentRenderFailureRetries
93. ⚠️ Cloudflare's free plan rate-limits bursts. Consider the paid plan for production. Test: none
94. 🔧 Creem's test and live keys mixed up when going to production. The server refuses to start in production with a Creem or Stripe test key, logs each provider's mode at startup, and in production drops Creem webhooks marked test or sandbox (any other mode value still applies). Test product IDs on a live key still break live checkout, and nothing checks them. Test: TestLoadRefusesTestPaymentKeysInProduction, TestPaymentModes, TestCreemWebhookTestMode, TestProductionIgnoresTestModeWebhooks
95. ⚠️ Running Creem and Stripe together works. Moving existing customers off Creem has no path: Creem must stay enabled until its subscriptions end. Test: none

## Known limits

Relay ships with these. Each was found by the 2026-11-07 checks and left alone under the stop rule. The rule fixes only what an earlier fix caused or what loses money or gives away access, and prefers logging for staff over new jobs.

Installments and refunds:

- Pay now on an order with a refunded installment can double charge. Relay opens the installment for Pay now while a provider schedule may charge it as well. The second charge is refunded automatically, and the provider keeps its fee.
- A refunded balance payment on a provider schedule reopens every installment it settled but emails a reminder for only one.
- An installment paid early keeps its future due date when a refund reopens it, so it isn't due, and no reminder goes out, until that date.
- A provider-schedule order with no refunded installment refuses Pay now, because the provider charges the schedule.
- An invoice's status counts a refund Relay asked for but hasn't sent. A partial refund made in the provider's dashboard in that window can show the invoice as refunded in full and end the plan.
- A refund job that runs out of retries isn't sent again. The hourly report logs it once at ERROR after a day, and logs a refund the provider took but never named after three days. A person finishes it.
- The hourly workers that run two passes (subscription read-back then refused cancels, stuck events then stuck refunds) skip the second when the first fails. It waits for the next run.
- Migration 00040 marks a lost dispute only when a suspended grant has one disputed invoice. With two or more they stay disputed on the CRM list for staff to close.

Provider calls and webhooks:

- A provider that keeps refusing to cancel a subscription or schedule Relay ended keeps billing the customer. Relay records the refusal, asks again every hour and logs each time. A person fixes what the provider objects to.
- A cancel webhook leaves the pending plan change and the cancel flag set on the closed subscription. Canceled is final, so nothing reads them.
- A checkout recovered by read-back takes the time of the read as its payment time, up to 20 minutes late. Renewals keep the transaction's own time.
- A subscription with `renewal_mode='app'` is never renewed. Nothing creates one today, and the query that would have run them was deleted.
- Old invoices without a PDF were backfilled by migration 00035 with its best guess of the plan.
- The Creem docs don't say whether a refund request is idempotent, whether a payment carries a proration marker, what deleting a coupon does to running subscriptions, when a redemption counts, or whether a failed payment's ID can later succeed. Relay records each refund before asking, reads the transaction first and treats refunded as done; it tells a proration by its own rule (a period inside one already invoiced); it deletes a retired coupon mirror only after retiring it; it sets Creem's redemption cap from confirmed redemptions only; and it treats a payment ID seen before as a replay, so a failed payment's ID that later succeeds would be dropped.

## Suggested order

1. Fill in the `pending` tests for the limits fixes in progress: cases 9, 35, 74, 75, 78, 79, 82, 85, 87 and 90.
2. Run the live Creem checks still open: one email on two workspaces (86), two paid checkouts (8), and a test-mode webhook on a production key (94).
3. Add the failed-payment and refund emails (26, 54).
4. Close the installment limits above, starting with the Pay now double charge, which costs the provider fee.
5. Set the seller details and ask an accountant about the invoice title and reverse charge (60, 61, 65).
