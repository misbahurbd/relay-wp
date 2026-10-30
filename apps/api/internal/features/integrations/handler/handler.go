// Package handler serves integrations' HTTP endpoints: it decodes requests, calls the service and
// encodes responses. Routes are registered in routes.go.
package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/service"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// oauthCookie binds a Google or Shopify connect attempt to the browser that started it.
const oauthCookie = "relay_int_oauth"

// CookieConfig sets the oauth cookie's scope, as for the auth cookies.
type CookieConfig struct {
	Domain string
	Secure bool
}

// Handler turns HTTP requests into service calls.
type Handler struct {
	svc     *service.Service
	cookies CookieConfig
}

// NewHandler builds the integrations HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *service.Service, cookies CookieConfig) *Handler {
	return &Handler{svc: svc, cookies: cookies}
}

func (h *Handler) overview(ctx context.Context, in *dto.WorkspaceInput) (*dto.IntegrationsOverviewOutput, error) {
	o, err := h.svc.Overview(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return dto.ToIntegrationsOverviewOutput(o), nil
}

func (h *Handler) events(context.Context, *dto.WorkspaceInput) (*dto.WebhookEventsOutput, error) {
	out := &dto.WebhookEventsOutput{}
	out.Body.Events = make([]dto.WebhookEventDTO, 0, len(model.Events))
	for _, e := range model.Events {
		out.Body.Events = append(out.Body.Events, dto.WebhookEventDTO{Type: e.Type, Description: e.Description})
	}
	return out, nil
}

// ---------- API keys ----------

func (h *Handler) apiKeys(ctx context.Context, in *dto.WorkspaceInput) (*dto.APIKeysOutput, error) {
	keys, err := h.svc.APIKeys(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &dto.APIKeysOutput{}
	out.Body.Keys = make([]dto.APIKeyDTO, 0, len(keys))
	for _, k := range keys {
		out.Body.Keys = append(out.Body.Keys, dto.ToAPIKeyDTO(k))
	}
	return out, nil
}

func (h *Handler) createAPIKey(ctx context.Context, in *dto.CreateAPIKeyInput) (*dto.NewAPIKeyOutput, error) {
	userID, _ := authctx.UserID(ctx)
	k, err := h.svc.CreateAPIKey(ctx, in.WorkspaceID, userID, in.Body.Name, in.Body.App)
	if err != nil {
		return nil, err
	}
	k.CreatedByName = createdByName(ctx, h, in.WorkspaceID, k.ID)
	return dto.ToNewAPIKeyOutput(k), nil
}

func (h *Handler) rollAPIKey(ctx context.Context, in *dto.APIKeyInput) (*dto.NewAPIKeyOutput, error) {
	userID, _ := authctx.UserID(ctx)
	k, err := h.svc.RollAPIKey(ctx, in.WorkspaceID, userID, in.KeyID)
	if err != nil {
		return nil, err
	}
	k.CreatedByName = createdByName(ctx, h, in.WorkspaceID, k.ID)
	return dto.ToNewAPIKeyOutput(k), nil
}

// createdByName fills the creator's display name on a key just made, from the list.
func createdByName(ctx context.Context, h *Handler, workspaceID, keyID uuid.UUID) string {
	keys, err := h.svc.APIKeys(ctx, workspaceID)
	if err != nil {
		return ""
	}
	for _, k := range keys {
		if k.ID == keyID {
			return k.CreatedByName
		}
	}
	return ""
}

func (h *Handler) revokeAPIKey(ctx context.Context, in *dto.APIKeyInput) (*struct{}, error) {
	return nil, h.svc.RevokeAPIKey(ctx, in.WorkspaceID, in.KeyID)
}

// ---------- Webhooks ----------

func (h *Handler) endpoints(ctx context.Context, in *dto.WorkspaceInput) (*dto.EndpointsOutput, error) {
	list, err := h.svc.Endpoints(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &dto.EndpointsOutput{}
	out.Body.Endpoints = make([]dto.EndpointDTO, 0, len(list))
	for _, e := range list {
		out.Body.Endpoints = append(out.Body.Endpoints, dto.ToEndpointDTO(e))
	}
	return out, nil
}

func (h *Handler) createEndpoint(ctx context.Context, in *dto.CreateEndpointInput) (*dto.CreatedEndpointOutput, error) {
	userID, _ := authctx.UserID(ctx)
	e, secret, err := h.svc.CreateEndpoint(ctx, in.WorkspaceID, userID, in.Body.Model())
	if err != nil {
		return nil, err
	}
	out := &dto.CreatedEndpointOutput{}
	out.Body.Endpoint, out.Body.Secret = dto.ToEndpointDTO(e), secret
	return out, nil
}

func (h *Handler) updateEndpoint(ctx context.Context, in *dto.UpdateEndpointInput) (*dto.EndpointOutput, error) {
	e, err := h.svc.UpdateEndpoint(ctx, in.WorkspaceID, in.EndpointID, in.Body.Model())
	if err != nil {
		return nil, err
	}
	return &dto.EndpointOutput{Body: dto.ToEndpointDTO(e)}, nil
}

func (h *Handler) deleteEndpoint(ctx context.Context, in *dto.EndpointInput) (*struct{}, error) {
	return nil, h.svc.DeleteEndpoint(ctx, in.WorkspaceID, in.EndpointID)
}

func (h *Handler) endpointSecret(ctx context.Context, in *dto.EndpointInput) (*dto.SecretOutput, error) {
	secret, err := h.svc.EndpointSecret(ctx, in.WorkspaceID, in.EndpointID)
	if err != nil {
		return nil, err
	}
	out := &dto.SecretOutput{}
	out.Body.Secret = secret
	return out, nil
}

func (h *Handler) rotateSecret(ctx context.Context, in *dto.EndpointInput) (*dto.SecretOutput, error) {
	secret, err := h.svc.RotateEndpointSecret(ctx, in.WorkspaceID, in.EndpointID)
	if err != nil {
		return nil, err
	}
	out := &dto.SecretOutput{}
	out.Body.Secret = secret
	return out, nil
}

func (h *Handler) sendTest(ctx context.Context, in *dto.EndpointInput) (*dto.DeliveryOutput, error) {
	d, err := h.svc.SendTest(ctx, in.WorkspaceID, in.EndpointID)
	if err != nil {
		return nil, err
	}
	return &dto.DeliveryOutput{Body: dto.ToDeliveryDTO(d)}, nil
}

func (h *Handler) deliveries(ctx context.Context, in *dto.DeliveriesInput) (*dto.DeliveriesOutput, error) {
	limit := in.Limit
	if limit == 0 {
		limit = 25
	}
	at, id, ok := dto.DecodeCursor(in.Cursor)
	var (
		afterAt = &at
		afterID = &id
	)
	if !ok {
		afterAt, afterID = nil, nil
	}
	// One extra row says whether there's a next page.
	list, err := h.svc.Deliveries(ctx, in.WorkspaceID, in.EndpointID, afterAt, afterID, limit+1)
	if err != nil {
		return nil, err
	}
	out := &dto.DeliveriesOutput{}
	if len(list) > int(limit) {
		list = list[:limit]
		last := list[len(list)-1]
		c := dto.EncodeCursor(last.CreatedAt, last.ID)
		out.Body.NextCursor = &c
	}
	out.Body.Deliveries = make([]dto.DeliveryDTO, 0, len(list))
	for _, d := range list {
		out.Body.Deliveries = append(out.Body.Deliveries, dto.ToDeliveryDTO(d))
	}
	return out, nil
}

func (h *Handler) redeliver(ctx context.Context, in *dto.RedeliverInput) (*struct{}, error) {
	return nil, h.svc.Redeliver(ctx, in.WorkspaceID, in.EndpointID, in.DeliveryID)
}

// ---------- Connections ----------

func (h *Handler) connect(ctx context.Context, in *dto.ConnectInput) (*dto.ConnectionOutput, error) {
	userID, _ := authctx.UserID(ctx)
	c, err := h.svc.Connect(ctx, in.WorkspaceID, userID, in.Provider, in.Model())
	if err != nil {
		return nil, err
	}
	return &dto.ConnectionOutput{Body: dto.ToConnectionDTO(c)}, nil
}

func (h *Handler) testConnection(ctx context.Context, in *dto.ProviderInput) (*dto.ConnectionOutput, error) {
	c, err := h.svc.TestConnection(ctx, in.WorkspaceID, in.Provider)
	if err != nil {
		return nil, err
	}
	return &dto.ConnectionOutput{Body: dto.ToConnectionDTO(c)}, nil
}

func (h *Handler) disconnect(ctx context.Context, in *dto.ProviderInput) (*struct{}, error) {
	return nil, h.svc.Disconnect(ctx, in.WorkspaceID, in.Provider)
}

func (h *Handler) authorizeGoogle(ctx context.Context, in *dto.WorkspaceInput) (*dto.AuthorizeOutput, error) {
	userID, _ := authctx.UserID(ctx)
	url, nonce, err := h.svc.AuthorizeGoogle(in.WorkspaceID, userID)
	if err != nil {
		return nil, err
	}
	out := &dto.AuthorizeOutput{SetCookie: h.cookie(callbackPath, nonce, 900)}
	out.Body.URL = url
	return out, nil
}

func (h *Handler) authorizeShopify(ctx context.Context, in *dto.AuthorizeShopifyInput) (*dto.AuthorizeOutput, error) {
	userID, _ := authctx.UserID(ctx)
	url, nonce, err := h.svc.AuthorizeShopify(in.WorkspaceID, userID, in.Body.ShopDomain)
	if err != nil {
		return nil, err
	}
	out := &dto.AuthorizeOutput{SetCookie: h.cookie(service.ShopifyCallbackPath, nonce, 900)}
	out.Body.URL = url
	return out, nil
}

func (h *Handler) shopifyCallback(ctx context.Context, in *dto.ShopifyCallbackInput) (*dto.RedirectOutput, error) {
	location := h.svc.ShopifyCallback(ctx, in.RawQuery, in.Cookie)
	return &dto.RedirectOutput{
		Status: http.StatusFound, Location: location, CacheControl: "no-store",
		SetCookie: h.cookie(service.ShopifyCallbackPath, "", -1),
	}, nil
}

func (h *Handler) authorizeWoo(ctx context.Context, in *dto.AuthorizeWooInput) (*dto.WooAuthorizeOutput, error) {
	userID, _ := authctx.UserID(ctx)
	url, err := h.svc.AuthorizeWooCommerce(in.WorkspaceID, userID, in.Body.StoreURL)
	if err != nil {
		return nil, err
	}
	out := &dto.WooAuthorizeOutput{}
	out.Body.URL = url
	return out, nil
}

func (h *Handler) wooCallback(ctx context.Context, in *dto.WooCallbackInput) (*struct{}, error) {
	b := in.Body
	return nil, h.svc.WooCallback(ctx, b.UserID, b.ConsumerKey, b.ConsumerSecret)
}

func (h *Handler) googleCallback(ctx context.Context, in *dto.GoogleCallbackInput) (*dto.RedirectOutput, error) {
	location := h.svc.GoogleCallback(ctx, in.Code, in.State, in.Cookie, in.Error)
	return &dto.RedirectOutput{
		Status: http.StatusFound, Location: location, CacheControl: "no-store", SetCookie: h.cookie(callbackPath, "", -1),
	}, nil
}

// cookie is the connect nonce, sent back only to that provider's callback path.
func (h *Handler) cookie(path, value string, maxAge int) http.Cookie {
	return http.Cookie{ //nolint:gosec // Secure follows COOKIE_SECURE, false only on local http
		Name: oauthCookie, Value: value, Path: path, Domain: h.cookies.Domain, MaxAge: maxAge,
		HttpOnly: true, Secure: h.cookies.Secure, SameSite: http.SameSiteLaxMode,
	}
}
