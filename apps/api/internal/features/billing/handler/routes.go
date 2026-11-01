package handler

import (
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Middleware is a huma middleware (middleware.RequireAuth, middleware.RequireWorkspace, …).
type Middleware = func(huma.Context, func(huma.Context))

// Guards are the middlewares the routes need, built in internal/app.
type Guards struct {
	Auth   Middleware
	Member Middleware
	// Can returns a middleware that requires every one of perms.
	Can func(perms ...authctx.Permission) Middleware
	// Staff admits Relay staff only, for the /v1/admin routes.
	Staff Middleware
}

// Register adds the billing routes. Reads need billing.view; buying, changing
// a plan and editing billing details need billing.pay (Q11). The pricing
// catalog is public, and the payment webhook is signed by its provider.
func Register(api huma.API, h *Handler, g Guards, security []map[string][]string) {
	tags := []string{"billing"}
	view, pay := g.Can(authctx.PermBillingView), g.Can(authctx.PermBillingPay)
	ws := func(id, method, path, summary string, status int, perm Middleware) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: method, Path: "/v1/workspace/billing" + path, Summary: summary,
			Tags: tags, Security: security, DefaultStatus: status,
			Middlewares: huma.Middlewares{g.Auth, g.Member, perm},
		}
	}
	ok, created := http.StatusOK, http.StatusCreated

	huma.Register(api, huma.Operation{
		OperationID: "list-billing-plans", Method: http.MethodGet, Path: "/v1/billing/plans",
		Summary: "The pricing catalog: plans, prices, installment offers and payment providers", Tags: tags,
	}, h.listPlans)
	huma.Register(api, huma.Operation{
		OperationID: "list-billing-resources", Method: http.MethodGet, Path: "/v1/billing/resources",
		Summary: "What plans can include, in display order (ADR 0008)", Tags: tags,
	}, h.listResources)
	huma.Register(api, huma.Operation{
		OperationID: "payment-webhook", Method: http.MethodPost, Path: "/v1/webhooks/payments/{provider}",
		Summary: "A payment provider's webhook (signed by the provider)", Tags: tags, DefaultStatus: ok,
	}, h.receiveWebhook)

	huma.Register(api, ws("get-billing-overview", http.MethodGet, "",
		"The current plan, subscription and lifetime deal", ok, view), h.overview)
	huma.Register(api, ws("get-billing-usage", http.MethodGet, "/usage",
		"This month's use of each monthly plan resource (ADR 0008)", ok, view), h.usage)

	huma.Register(api, ws("create-checkout-session", http.MethodPost, "/checkout-sessions",
		"Start a checkout for a price (subscription or lifetime deal)", created, pay), h.createCheckout)
	huma.Register(api, ws("get-checkout-session", http.MethodGet, "/checkout-sessions/{sessionId}",
		"A checkout session's status", ok, view), h.checkoutSession)
	huma.Register(api, ws("complete-test-checkout", http.MethodPost, "/checkout-sessions/{sessionId}/test-complete",
		"Pay a checkout on the development test gateway (404 on a real provider)", ok, pay), h.completeTestCheckout)
	huma.Register(api, ws("preview-discount", http.MethodPost, "/discounts/preview",
		"What a discount code takes off a price", ok, view), h.previewDiscount)

	huma.Register(api, ws("get-subscription", http.MethodGet, "/subscription",
		"The live subscription, or the latest one", ok, view), h.subscription)
	huma.Register(api, ws("preview-subscription-change", http.MethodPost, "/subscription/change-preview",
		"What changing to another price would do now", ok, view), h.previewChange)
	huma.Register(api, ws("change-subscription", http.MethodPost, "/subscription/change",
		"Upgrade now (prorated) or downgrade at the period end", ok, pay), h.changePlan)
	huma.Register(api, ws("cancel-pending-subscription-change", http.MethodDelete, "/subscription/pending-change",
		"Drop a scheduled downgrade", ok, pay), h.cancelPendingChange)
	huma.Register(api, ws("cancel-subscription", http.MethodPost, "/subscription/cancel",
		"Cancel now or at the period end", ok, pay), h.cancelSubscription)
	huma.Register(api, ws("resume-subscription", http.MethodPost, "/subscription/resume",
		"Keep a subscription that was set to cancel", ok, pay), h.resumeSubscription)

	huma.Register(api, ws("quote-order", http.MethodPost, "/orders/quote",
		"What a lifetime deal costs now, after discount and credit, and its schedule", ok, view), h.quoteOrder)
	huma.Register(api, ws("get-current-order", http.MethodGet, "/orders/current",
		"The live lifetime deal with its installments", ok, view), h.currentOrder)
	huma.Register(api, ws("get-order", http.MethodGet, "/orders/{orderId}",
		"One order with its installments", ok, view), h.order)
	huma.Register(api, ws("pay-installment", http.MethodPost, "/orders/{orderId}/installments/{installmentId}/pay",
		"Start a checkout for a due installment", created, pay), h.payInstallment)
	huma.Register(api, ws("resume-order", http.MethodPost, "/orders/{orderId}/resume",
		"Start a checkout for a defaulted order's remaining balance", created, pay), h.resumeOrder)

	huma.Register(api, ws("list-invoices", http.MethodGet, "/invoices", "Invoices, newest first", ok, view),
		h.listInvoices)
	huma.Register(api, ws("get-invoice", http.MethodGet, "/invoices/{invoiceId}",
		"One invoice with its billing details", ok, view), h.invoice)

	huma.Register(api, ws("get-billing-profile", http.MethodGet, "/profile", "Billing details", ok, view), h.profile)
	huma.Register(api, ws("update-billing-profile", http.MethodPut, "/profile",
		"Replace billing details; past invoices keep theirs", ok, pay), h.updateProfile)

	huma.Register(api, ws("get-payment-method", http.MethodGet, "/payment-method",
		"The card the payment provider charges next, read from the provider", ok, view), h.paymentMethod)
	huma.Register(api, ws("create-portal-session", http.MethodPost, "/portal-session",
		"A link to the payment provider's page for changing cards", ok, pay), h.portalSession)

	registerAdmin(api, h, g, security)
}

// registerAdmin adds the staff admin routes: building the catalog, discount
// codes, and moving workspaces between plans by hand. No workspace header: the
// workspace a grant route acts on is in its body.
func registerAdmin(api huma.API, h *Handler, g Guards, security []map[string][]string) {
	tags := []string{"admin"}
	op := func(id, method, path, summary string, status int) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: method, Path: "/v1/admin" + path, Summary: summary,
			Tags: tags, Security: security, DefaultStatus: status,
			Middlewares: huma.Middlewares{g.Auth, g.Staff},
		}
	}
	ok, created, none := http.StatusOK, http.StatusCreated, http.StatusNoContent

	huma.Register(api, op("admin-get-catalog", http.MethodGet, "/billing/catalog",
		"Every plan, price, installment offer and provider price, active or not", ok), h.adminCatalog)
	huma.Register(api, op("admin-create-plan", http.MethodPost, "/billing/plans",
		"Add a plan; a code in use publishes its next version", created), h.createPlan)
	huma.Register(api, op("admin-update-plan", http.MethodPut, "/billing/plans/{id}",
		"Change a plan version in place (not its prices); new limits reach every workspace on it now", none), h.updatePlan)
	huma.Register(api, op("admin-create-price", http.MethodPost, "/billing/prices",
		"Add a price to a plan", created), h.createPrice)
	huma.Register(api, op("admin-create-installment-offer", http.MethodPost, "/billing/installment-offers",
		"Split a one-time price into monthly payments", created), h.createInstallmentOffer)
	huma.Register(api, op("admin-create-provider-price", http.MethodPost, "/billing/provider-prices",
		"Link a price or installment offer to a payment provider's price", created), h.createProviderPrice)
	for _, k := range catalogKinds {
		huma.Register(api, op("admin-deactivate-"+k.name, http.MethodPost, "/billing/"+k.path+"/{id}/deactivate",
			"Take a "+strings.ReplaceAll(k.name, "-", " ")+" off sale; what was bought keeps working", none),
			h.deactivate(k.kind))
	}

	huma.Register(api, op("admin-list-discounts", http.MethodGet, "/billing/discounts",
		"Every discount code, newest first", ok), h.adminDiscounts)
	huma.Register(api, op("admin-create-discount", http.MethodPost, "/billing/discounts",
		"Add a discount code", created), h.createDiscount)
	huma.Register(api, op("admin-deactivate-discount", http.MethodPost, "/billing/discounts/{code}/deactivate",
		"Stop a discount code from being used again", none), h.deactivateDiscount)

	huma.Register(api, op("admin-list-workspaces", http.MethodGet, "/workspaces",
		"Workspaces and the plan each is on, newest first", ok), h.adminWorkspaces)
	huma.Register(api, op("admin-grant-plan", http.MethodPost, "/workspaces/grant-plan",
		"Put a workspace on a plan by hand (not one paid through a subscription or lifetime deal)", none), h.grantPlan)
	huma.Register(api, op("admin-end-grant", http.MethodPost, "/workspaces/end-grant",
		"End a manual grant or trial; the workspace goes to the free plan", none), h.endGrant)
	huma.Register(api, op("admin-restore-grant", http.MethodPost, "/workspaces/restore-grant",
		"Reactivate a suspended plan", none), h.restoreGrant)
	huma.Register(api, op("admin-set-grant-overrides", http.MethodPut, "/workspaces/grant-overrides",
		"Set a workspace's own limits on top of its plan's; {} clears them", none), h.setGrantOverrides)
}
