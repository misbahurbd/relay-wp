package model

import "github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"

// ErrNotImplemented is returned by every billing endpoint a Phase 6 track hasn't built yet.
var ErrNotImplemented = apperr.NotImplemented("not_implemented", "This billing feature isn't available yet.")

// Errors returned by the billing service.
var (
	ErrInvalidInput = apperr.Validation("invalid_input", "Some fields are invalid")

	// Catalog and checkout.
	ErrPriceNotFound           = apperr.NotFound("price_not_found", "This price isn't available.")
	ErrOfferNotFound           = apperr.NotFound("installment_offer_not_found", "This installment plan isn't available.")
	ErrProviderNotAvailable    = apperr.NotFound("payment_provider_not_available", "This payment method isn't available.")
	ErrNoProviderForPrice      = apperr.Conflict("no_payment_provider", "This price can't be bought online yet.")
	ErrCheckoutNotFound        = apperr.NotFound("checkout_not_found", "Checkout not found")
	ErrCheckoutFinished        = apperr.Conflict("checkout_finished", "This checkout has already finished. Start a new one.")
	ErrIdempotencyKeyReused    = apperr.Conflict("idempotency_key_reused", "This Idempotency-Key was used for a different checkout.")
	ErrAlreadySubscribed       = apperr.Conflict("already_subscribed", "This workspace already has a subscription. Change its plan instead.")
	ErrHasLifetimeDeal         = apperr.Conflict("has_lifetime_deal", "This workspace has a lifetime deal. Upgrade it instead.")
	ErrPaymentProviderDeclined = apperr.Validation("payment_declined", "The payment provider declined this. Try another card.")
	ErrPaymentProviderMissing  = apperr.Conflict("payment_provider_out_of_sync",
		"The payment provider has no record of this. Contact support and we'll sort it out.")
	ErrPaymentProviderRefused = apperr.Validation("payment_provider_refused", "The payment provider couldn't do this. Contact support.")
	ErrPaymentProviderDown    = apperr.Unavailable("payment_provider_unavailable", "The payment provider isn't answering. Try again in a minute.")

	// Subscriptions.
	ErrNoSubscription     = apperr.NotFound("subscription_not_found", "This workspace has no subscription.")
	ErrSamePrice          = apperr.Validation("same_price", "The subscription is already on this price.")
	ErrNoPendingChange    = apperr.Conflict("no_pending_change", "There's no scheduled plan change.")
	ErrNotCanceling       = apperr.Conflict("not_canceling", "The subscription isn't set to cancel.")
	ErrSubscriptionEnded  = apperr.Conflict("subscription_ended", "This subscription has ended.")
	ErrChangeNotSupported = apperr.Conflict("change_not_supported", "This plan can't be changed online. Contact support.")

	// Orders and installments.
	ErrNoOrder              = apperr.NotFound("order_not_found", "Order not found")
	ErrInstallmentNotFound  = apperr.NotFound("installment_not_found", "Installment not found")
	ErrInstallmentNotDue    = apperr.Conflict("installment_not_due", "This installment can't be paid now.")
	ErrOrderNotDefaulted    = apperr.Conflict("order_not_defaulted", "Only a defaulted order can be resumed.")
	ErrOrderNotUpgradable   = apperr.Conflict("order_not_upgradable", "This lifetime deal can't be upgraded to that plan.")
	ErrOverpayment          = apperr.Conflict("overpayment", "This payment is more than the order's balance.")
	ErrInstallmentsDisabled = apperr.Validation("installments_not_allowed", "This plan can't be paid in installments.")

	// Discounts. Each rejection says why the code doesn't apply.
	ErrDiscountNotFound       = apperr.Validation("discount_not_found", "This code doesn't exist.")
	ErrDiscountInactive       = apperr.Validation("discount_inactive", "This code is no longer valid.")
	ErrDiscountNotStarted     = apperr.Validation("discount_not_started", "This code isn't valid yet.")
	ErrDiscountExpired        = apperr.Validation("discount_expired", "This code has expired.")
	ErrDiscountUsedUp         = apperr.Validation("discount_used_up", "This code has been used up.")
	ErrDiscountWorkspaceLimit = apperr.Validation("discount_already_used", "This workspace has already used this code.")
	ErrDiscountWrongPlan      = apperr.Validation("discount_wrong_plan", "This code doesn't apply to this plan.")
	ErrDiscountWrongPrice     = apperr.Validation("discount_wrong_price", "This code doesn't apply to this kind of price.")
	ErrDiscountNoInstallments = apperr.Validation("discount_no_installments", "This code can't be used with installments.")
	ErrDiscountCurrency       = apperr.Validation("discount_wrong_currency", "This code is for another currency.")
	ErrDiscountCodeTaken      = apperr.Conflict("discount_code_taken", "A discount with this code already exists.")

	// Profiles, invoices, grants.
	ErrProfileNotFound = apperr.NotFound("billing_profile_not_found", "Add your billing details first.")
	ErrInvoiceNotFound = apperr.NotFound("invoice_not_found", "Invoice not found")
	ErrInvalidCursor   = apperr.Validation("invalid_cursor", "The page cursor is invalid")
	ErrPlanNotFound    = apperr.NotFound("plan_not_found", "Plan not found")
	ErrNoCardCustomer  = apperr.NotFound("no_payment_customer", "There's no card on file yet. It's saved when you first pay.")

	// Staff admin tools.
	ErrWorkspaceNotFound = apperr.NotFound("workspace_not_found", "Workspace not found")
	ErrCatalogDuplicate  = apperr.Conflict("catalog_duplicate",
		"An active row like this already exists. Deactivate it first.")
	ErrCatalogRowNotFound = apperr.NotFound("catalog_row_not_found", "Not found, or already inactive")
	ErrGrantFromPayment   = apperr.Conflict("grant_from_payment",
		"This workspace's plan comes from a subscription or lifetime deal. Cancel that first.")
	ErrNoGrant           = apperr.NotFound("grant_not_found", "This workspace is on the free plan.")
	ErrGrantNotSuspended = apperr.Conflict("grant_not_suspended", "This workspace's plan isn't suspended.")

	// Webhooks.
	ErrWebhookSignature = apperr.BadRequest("invalid_signature", "Webhook signature doesn't match")
)
