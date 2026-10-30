package dto

import (
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
)

type ConnectionDTO struct {
	Provider      string     `json:"provider" enum:"shopify,woocommerce,google_sheets"`
	Status        string     `json:"status" enum:"connected,error,disconnected"`
	AccountLabel  string     `json:"account_label" example:"greenleaf.myshopify.com"`
	LastCheckedAt *time.Time `json:"last_checked_at" nullable:"true"`
	LastError     *string    `json:"last_error" nullable:"true"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func ToConnectionDTO(c model.Connection) ConnectionDTO {
	return ConnectionDTO{
		Provider: c.Provider, Status: c.Status, AccountLabel: c.AccountLabel, LastCheckedAt: c.LastCheckedAt,
		LastError: c.LastError, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

type ConnectionOutput struct {
	Body ConnectionDTO
}

type ConnectInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Provider    string    `path:"provider" enum:"shopify,woocommerce"`
	Body        struct {
		ShopDomain     string `json:"shop_domain,omitempty" maxLength:"255" doc:"Shopify: your-store.myshopify.com"`
		AccessToken    string `json:"access_token,omitempty" maxLength:"256" doc:"Shopify: Admin API access token from a custom app"`
		StoreURL       string `json:"store_url,omitempty" maxLength:"2048" doc:"WooCommerce: https://your-store.com"`
		ConsumerKey    string `json:"consumer_key,omitempty" maxLength:"256" doc:"WooCommerce: REST API consumer key"`
		ConsumerSecret string `json:"consumer_secret,omitempty" maxLength:"256" doc:"WooCommerce: REST API consumer secret"`
	}
}

func (in *ConnectInput) Model() model.ConnectInput {
	b := in.Body
	return model.ConnectInput{
		ShopDomain: b.ShopDomain, AccessToken: b.AccessToken, StoreURL: b.StoreURL,
		ConsumerKey: b.ConsumerKey, ConsumerSecret: b.ConsumerSecret,
	}
}

type ProviderInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Provider    string    `path:"provider" enum:"shopify,woocommerce,google_sheets"`
}

type AuthorizeOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
	Body      struct {
		URL string `json:"url" doc:"Open this to ask the provider for access"`
	}
}

type AuthorizeShopifyInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Body        struct {
		ShopDomain string `json:"shop_domain" maxLength:"255" doc:"your-store.myshopify.com, or just your-store"`
	}
}

type AuthorizeWooInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Body        struct {
		StoreURL string `json:"store_url" maxLength:"2048" doc:"https://your-store.com"`
	}
}

type WooAuthorizeOutput struct {
	Body struct {
		URL string `json:"url" doc:"Open this to approve Relay in the store's admin"`
	}
}

// ShopifyCallbackInput keeps the raw query, because Shopify's hmac covers every parameter.
type ShopifyCallbackInput struct {
	Shop     string `query:"shop" maxLength:"255"`
	Cookie   string `cookie:"relay_int_oauth" doc:"Binds the attempt to the browser that started it"`
	RawQuery string `json:"-"`
}

func (in *ShopifyCallbackInput) Resolve(ctx huma.Context) []error {
	u := ctx.URL()
	in.RawQuery = u.RawQuery
	return nil
}

// WooCallbackInput is what WooCommerce posts after the store owner approves Relay.
type WooCallbackInput struct {
	Body struct {
		_              struct{} `additionalProperties:"true"`
		KeyID          int64    `json:"key_id,omitempty"`
		UserID         string   `json:"user_id" maxLength:"2048" doc:"The signed state Relay sent as user_id"`
		ConsumerKey    string   `json:"consumer_key" maxLength:"256"`
		ConsumerSecret string   `json:"consumer_secret" maxLength:"256"`
		KeyPermissions string   `json:"key_permissions,omitempty" maxLength:"20"`
	}
}

type GoogleCallbackInput struct {
	Code   string `query:"code" maxLength:"2048"`
	State  string `query:"state" maxLength:"1024"`
	Error  string `query:"error" maxLength:"256" doc:"Set by Google when the person cancels or it fails"`
	Cookie string `cookie:"relay_int_oauth" doc:"Binds the attempt to the browser that started it"`
}

type RedirectOutput struct {
	Status       int
	Location     string      `header:"Location"`
	CacheControl string      `header:"Cache-Control"`
	SetCookie    http.Cookie `header:"Set-Cookie"`
}
