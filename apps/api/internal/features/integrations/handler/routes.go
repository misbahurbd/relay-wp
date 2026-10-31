package handler

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/service"
)

// Middleware is a huma middleware (middleware.RequireAuth, middleware.RequireWorkspace).
type Middleware = func(huma.Context, func(huma.Context))

// Guards are the middlewares integrations routes use. Member, Keys and Manage run
// after Auth and check the X-Workspace-ID header; Session refuses API keys.
type Guards struct {
	Auth    Middleware
	Session Middleware
	Member  Middleware
	// Keys checks integrations.api_keys; Manage checks integrations.manage.
	Keys   Middleware
	Manage Middleware
}

// callbackPath is Google's redirect URI path: {API_BASE_URL}/v1/integrations/google/callback.
const callbackPath = "/v1/integrations/google/callback"

// Register adds the integrations routes (docs/PHASE-5-INTEGRATIONS.md). Every
// workspace route needs a session: API keys can't make keys, webhooks or connections.
func Register(api huma.API, h *Handler, g Guards, security []map[string][]string) {
	tags := []string{"integrations"}
	op := func(id, method, path, summary string, status int, mw ...Middleware) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: method, Path: path, Summary: summary, Tags: tags, DefaultStatus: status,
			Security: security, Middlewares: append(huma.Middlewares{g.Auth, g.Session}, mw...),
		}
	}
	const ws = "/v1/workspace/integrations"

	huma.Register(api, op("get-integrations", http.MethodGet, ws,
		"What each Integrations card shows", http.StatusOK, g.Member), h.overview)
	huma.Register(api, op("list-webhook-events", http.MethodGet, ws+"/events",
		"Events a webhook endpoint can subscribe to", http.StatusOK, g.Member), h.events)

	huma.Register(api, op("list-api-keys", http.MethodGet, ws+"/api-keys",
		"Active API keys (never the keys themselves)", http.StatusOK, g.Keys), h.apiKeys)
	huma.Register(api, op("create-api-key", http.MethodPost, ws+"/api-keys",
		"Make an API key; the full key is in this response only", http.StatusCreated, g.Keys), h.createAPIKey)
	huma.Register(api, op("roll-api-key", http.MethodPost, ws+"/api-keys/{keyId}/roll",
		"Replace a key: a new one, and the old one stops working now", http.StatusCreated, g.Keys), h.rollAPIKey)
	huma.Register(api, op("revoke-api-key", http.MethodDelete, ws+"/api-keys/{keyId}",
		"Revoke an API key", http.StatusNoContent, g.Keys), h.revokeAPIKey)

	huma.Register(api, op("list-webhooks", http.MethodGet, ws+"/webhooks",
		"Webhook endpoints and their delivery health", http.StatusOK, g.Manage), h.endpoints)
	huma.Register(api, op("create-webhook", http.MethodPost, ws+"/webhooks",
		"Add a webhook endpoint", http.StatusCreated, g.Manage), h.createEndpoint)
	huma.Register(api, op("update-webhook", http.MethodPatch, ws+"/webhooks/{endpointId}",
		"Change a webhook endpoint", http.StatusOK, g.Manage), h.updateEndpoint)
	huma.Register(api, op("delete-webhook", http.MethodDelete, ws+"/webhooks/{endpointId}",
		"Remove a webhook endpoint and its delivery log", http.StatusNoContent, g.Manage), h.deleteEndpoint)
	huma.Register(api, op("get-webhook-secret", http.MethodGet, ws+"/webhooks/{endpointId}/secret",
		"The endpoint's signing secret", http.StatusOK, g.Manage), h.endpointSecret)
	huma.Register(api, op("rotate-webhook-secret", http.MethodPost, ws+"/webhooks/{endpointId}/secret/rotate",
		"Replace the signing secret", http.StatusOK, g.Manage), h.rotateSecret)
	huma.Register(api, op("send-webhook-test", http.MethodPost, ws+"/webhooks/{endpointId}/test",
		"Send a ping event now and return the result", http.StatusOK, g.Manage), h.sendTest)
	huma.Register(api, op("list-webhook-deliveries", http.MethodGet, ws+"/webhooks/{endpointId}/deliveries",
		"The endpoint's delivery log, newest first (30 days)", http.StatusOK, g.Manage), h.deliveries)
	huma.Register(api, op("redeliver-webhook", http.MethodPost, ws+"/webhooks/{endpointId}/deliveries/{deliveryId}/redeliver",
		"Send a finished delivery again", http.StatusAccepted, g.Manage), h.redeliver)

	// Stores post these and sign them, so they need no session.
	public := func(id, path, summary string) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: http.MethodPost, Path: path, Summary: summary, Tags: tags,
			DefaultStatus: http.StatusOK, MaxBodyBytes: 5 << 20,
		}
	}
	huma.Register(api, public("shopify-privacy-webhook", "/v1/webhooks/shopify/privacy",
		"Shopify's customer data and erasure requests"), h.shopifyPrivacy)
	huma.Register(api, public("shopify-order-webhook", service.CommercePath+"/shopify/{connectionId}",
		"An order event from a connected Shopify store"), h.shopifyWebhook)
	huma.Register(api, public("woocommerce-order-webhook", service.CommercePath+"/woocommerce/{connectionId}",
		"An order event from a connected WooCommerce store"), h.wooWebhook)

	// The static authorize routes are registered before {provider} so the router picks them.
	huma.Register(api, op("authorize-google-sheets", http.MethodPost, ws+"/connections/google_sheets/authorize",
		"Start connecting Google Sheets: returns Google's consent URL", http.StatusOK, g.Manage), h.authorizeGoogle)
	huma.Register(api, op("authorize-shopify", http.MethodPost, ws+"/connections/shopify/authorize",
		"Start connecting a Shopify store in one click: returns its install URL", http.StatusOK, g.Manage), h.authorizeShopify)
	huma.Register(api, op("authorize-woocommerce", http.MethodPost, ws+"/connections/woocommerce/authorize",
		"Start connecting a WooCommerce store in one click: returns its approval URL", http.StatusOK, g.Manage), h.authorizeWoo)
	huma.Register(api, op("connect-integration", http.MethodPut, ws+"/connections/{provider}",
		"Connect a store; its credentials are checked first", http.StatusOK, g.Manage), h.connect)
	huma.Register(api, op("test-integration", http.MethodPost, ws+"/connections/{provider}/test",
		"Check a connection again and record the result", http.StatusOK, g.Manage), h.testConnection)
	huma.Register(api, op("disconnect-integration", http.MethodDelete, ws+"/connections/{provider}",
		"Disconnect and forget the credentials", http.StatusNoContent, g.Manage), h.disconnect)

	huma.Register(api, huma.Operation{
		OperationID: "google-sheets-callback", Method: http.MethodGet, Path: callbackPath, Tags: tags,
		Summary:       "Google's redirect: saves the connection, then sends the person to Integrations",
		DefaultStatus: http.StatusFound,
	}, h.googleCallback)
	huma.Register(api, huma.Operation{
		OperationID: "shopify-callback", Method: http.MethodGet, Path: service.ShopifyCallbackPath, Tags: tags,
		Summary:       "Shopify's redirect after install: saves the store, then sends the person to Integrations",
		DefaultStatus: http.StatusFound,
	}, h.shopifyCallback)
	huma.Register(api, huma.Operation{
		OperationID: "woocommerce-callback", Method: http.MethodPost, Path: service.WooCallbackPath, Tags: tags,
		Summary:       "WooCommerce posts the REST keys the store owner approved (server to server)",
		DefaultStatus: http.StatusOK,
	}, h.wooCallback)
}
