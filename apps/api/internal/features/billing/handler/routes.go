package handler

import (
	"net/http"

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
		OperationID: "payment-webhook", Method: http.MethodPost, Path: "/v1/webhooks/payments/{provider}",
		Summary: "A payment provider's webhook (signed by the provider)", Tags: tags, DefaultStatus: ok,
	}, h.receiveWebhook)

	huma.Register(api, ws("get-billing-overview", http.MethodGet, "",
		"The current plan, subscription and lifetime deal", ok, view), h.overview)

	huma.Register(api, ws("create-checkout-session", http.MethodPost, "/checkout-sessions",
		"Start a checkout for a price (subscription or lifetime deal)", created, pay), h.createCheckout)
	huma.Register(api, ws("get-checkout-session", http.MethodGet, "/checkout-sessions/{sessionId}",
		"A checkout session's status", ok, view), h.checkoutSession)
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
}
