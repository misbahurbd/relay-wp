package model

import "github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"

// Errors returned by the integrations service.
var (
	ErrKeyNotFound             = apperr.NotFound("api_key_not_found", "API key not found")
	ErrTooManyKeys             = apperr.Conflict("too_many_api_keys", "A workspace can have 20 active API keys. Revoke one first.")
	ErrEndpointNotFound        = apperr.NotFound("webhook_not_found", "Webhook endpoint not found")
	ErrDeliveryNotFound        = apperr.NotFound("webhook_delivery_not_found", "Delivery not found, or it's still being sent")
	ErrTooManyEndpoints        = apperr.Conflict("too_many_webhooks", "A workspace can have 10 webhook endpoints.")
	ErrInvalidURL              = apperr.Validation("invalid_webhook_url", "Use a public https:// URL")
	ErrUnknownEvent            = apperr.Validation("unknown_event", "Pick events from the list")
	ErrUnknownProvider         = apperr.NotFound("unknown_provider", "This app can't be connected that way")
	ErrNotConnected            = apperr.NotFound("integration_not_connected", "This app isn't connected")
	ErrInvalidShop             = apperr.Validation("invalid_shop_domain", "Enter your store's myshopify.com address, like greenleaf.myshopify.com")
	ErrInvalidStoreURL         = apperr.Validation("invalid_store_url", "Enter your store's https:// address")
	ErrMissingFields           = apperr.Validation("missing_credentials", "Fill in every field")
	ErrBadCredentials          = apperr.Validation("credentials_rejected", "The store didn't accept these credentials. Check them and try again.")
	ErrStoreUnreachable        = apperr.Unavailable("store_unreachable", "We couldn't reach the store. Check the address and try again.")
	ErrStoreNeedsReconnect     = apperr.Conflict("store_needs_reconnect", "Relay can't receive orders from this store. Reconnect it from Integrations.")
	ErrBadStoreSignature       = apperr.Unauthorized("invalid_store_signature", "This webhook isn't signed correctly")
	ErrBadStorePayload         = apperr.Validation("invalid_store_payload", "This webhook can't be read")
	ErrGoogleUnavailable       = apperr.NotFound("google_unavailable", "Google Sheets isn't set up on this server yet.")
	ErrBadState                = apperr.Validation("invalid_oauth_state", "This sign-in link expired. Start again from Integrations.")
	ErrShopifyOAuthUnavailable = apperr.NotFound("shopify_oauth_unavailable", "One-click Shopify connections aren't set up on this server. Use an access token instead.")
	ErrShopifyExchange         = apperr.Validation("shopify_exchange_failed", "Shopify didn't confirm the connection. Try again.")
	ErrWooOAuthUnavailable     = apperr.NotFound("woocommerce_oauth_unavailable", "One-click WooCommerce connections need this server on https. Use REST API keys instead.")
	ErrGoogleExchange          = apperr.Validation("google_exchange_failed", "Google didn't confirm the connection. Try again.")
	ErrNoRefreshToken          = apperr.Validation("google_no_offline_access", "Google didn't grant offline access. Remove Relay from your Google account's connections and try again.")
)
