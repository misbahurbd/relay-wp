// Package model holds billing's domain types, constants and errors, shared by every layer of the
// feature. Amounts are integer minor units (cents) everywhere; never floats.
package model

// FreePlan is the plan code a workspace has when it has no current grant (Q5).
// workspaces.plan caches it like any other plan code.
const FreePlan = "free"

// Price billing types (plan_prices.billing_type).
const (
	BillingRecurring = "recurring"
	BillingOneTime   = "one_time"
)

// Intervals (plan_prices.interval for recurring prices, installment_offers.interval).
const (
	IntervalMonth = "month"
	IntervalYear  = "year"
)

// How a provider collects installments (provider_prices.collection).
const (
	CollectionProviderSchedule = "provider_schedule" // the provider runs the N-payment schedule
	CollectionAppLink          = "app_link"          // Relay emails a pay link for each installment
	CollectionProviderFinanced = "provider_financed" // a bank pays in full and finances the customer (SSLCommerz EMI)
)

// Who renews a subscription (subscriptions.renewal_mode).
const (
	RenewalProvider = "provider"
	RenewalApp      = "app" // a River job renews it (SSLCommerz)
)
