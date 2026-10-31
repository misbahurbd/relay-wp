package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
)

// shopifyScopes are what syncing customers, reading orders and following checkouts need.
const shopifyScopes = "read_customers,read_orders,read_checkouts"

// ShopifyCallbackPath is the Shopify app's redirect URL path, under API_BASE_URL.
const ShopifyCallbackPath = "/v1/integrations/shopify/callback"

// ShopifyConfig is the Shopify app used for one-click store connections.
type ShopifyConfig struct {
	ClientID     string
	ClientSecret string
}

// AuthorizeShopify returns the store's install URL and the nonce the handler puts
// in a cookie. Without a Shopify app, stores connect with an access token instead.
func (s *Service) AuthorizeShopify(workspaceID, userID uuid.UUID, shopDomain string) (authURL, nonce string, err error) {
	if s.cfg.Shopify.ClientID == "" {
		return "", "", model.ErrShopifyOAuthUnavailable
	}
	shop, err := normalizeShop(shopDomain)
	if err != nil {
		return "", "", err
	}
	state, nonce, err := s.newState(workspaceID, userID, model.ProviderShopify, shop, true)
	if err != nil {
		return "", "", err
	}
	q := url.Values{
		"client_id": {s.cfg.Shopify.ClientID}, "scope": {shopifyScopes},
		"redirect_uri": {s.shopifyRedirectURL()}, "state": {state},
	}
	return "https://" + shop + "/admin/oauth/authorize?" + q.Encode(), nonce, nil
}

func (s *Service) shopifyRedirectURL() string {
	return strings.TrimRight(s.cfg.APIBaseURL, "/") + ShopifyCallbackPath
}

// ShopifyCallback finishes a store install from the raw callback query and returns
// where to send the person: Integrations with ?connected=shopify or ?integration_error=<code>.
func (s *Service) ShopifyCallback(ctx context.Context, rawQuery, nonce string) string {
	back := s.integrationsPage()
	fail := func(err error) string {
		return s.connectFailure(back, model.ProviderShopify, err, model.ErrShopifyExchange.Code)
	}
	q, err := url.ParseQuery(rawQuery)
	if err != nil || !s.shopifySigned(q) {
		return fail(model.ErrBadState)
	}
	st, err := s.readState(q.Get("state"), model.ProviderShopify, nonce)
	if err != nil {
		return fail(err)
	}
	// The shop Shopify names must be the one the person asked to connect.
	if shop := q.Get("shop"); shop != st.Target || !shopDomainRe.MatchString(shop) {
		return fail(model.ErrBadState)
	}
	code := q.Get("code")
	if code == "" {
		return fail(model.ErrShopifyExchange)
	}
	token, err := s.exchangeShopify(ctx, st.Target, code)
	if err != nil {
		return fail(err)
	}
	label, err := s.checkShopify(ctx, st.Target, token)
	if err != nil {
		return fail(err)
	}
	cfg, creds := storeConfig{Shop: st.Target, OAuth: true}, storeCredentials{AccessToken: token}
	if err := s.saveConnection(ctx, st.WorkspaceID, st.UserID, model.ProviderShopify, label, cfg, creds); err != nil {
		return fail(err)
	}
	s.setUpStoreWebhooks(ctx, st.WorkspaceID, model.ProviderShopify, cfg, creds)
	return back + "?connected=" + model.ProviderShopify
}

// shopifySigned checks Shopify's hmac: every other parameter, sorted, as k=v joined
// by &, signed with the app's secret (shopify.dev, "Verify a request").
func (s *Service) shopifySigned(q url.Values) bool {
	got, err := hex.DecodeString(q.Get("hmac"))
	if err != nil || len(got) == 0 || s.cfg.Shopify.ClientSecret == "" {
		return false
	}
	keys := make([]string, 0, len(q))
	for k := range q {
		if k != "hmac" && k != "signature" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + strings.Join(q[k], ",")
	}
	mac := hmac.New(sha256.New, []byte(s.cfg.Shopify.ClientSecret))
	mac.Write([]byte(strings.Join(parts, "&")))
	return hmac.Equal(got, mac.Sum(nil))
}

// exchangeShopify trades the install code for an offline Admin API access token.
func (s *Service) exchangeShopify(ctx context.Context, shop, code string) (string, error) {
	base := "https://" + shop
	if s.cfg.ShopifyBaseURL != "" {
		base = s.cfg.ShopifyBaseURL
	}
	body, err := json.Marshal(map[string]string{
		"client_id": s.cfg.Shopify.ClientID, "client_secret": s.cfg.Shopify.ClientSecret, "code": code,
	})
	if err != nil {
		return "", fmt.Errorf("shopify token body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/admin/oauth/access_token", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("shopify token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		return "", model.ErrStoreUnreachable.Wrap(err)
	}
	defer func() { _ = resp.Body.Close() }()
	// A used, expired or foreign code is a 4xx: the person should simply try again.
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return "", model.ErrShopifyExchange
	}
	if resp.StatusCode != http.StatusOK {
		return "", model.ErrStoreUnreachable.Wrap(fmt.Errorf("shopify token: status %d", resp.StatusCode))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<20)).Decode(&tok); err != nil || tok.AccessToken == "" {
		return "", model.ErrShopifyExchange
	}
	return tok.AccessToken, nil
}
