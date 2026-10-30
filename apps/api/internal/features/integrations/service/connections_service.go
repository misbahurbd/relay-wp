package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/repository"
)

// shopifyAPIVersion is the Admin API version used to check a connection.
const shopifyAPIVersion = "2025-10"

var shopDomainRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,60}\.myshopify\.com$`)

// storeCredentials is what's encrypted in integration_connections.credentials.
type storeCredentials struct {
	AccessToken    string `json:"access_token,omitempty"`
	ConsumerKey    string `json:"consumer_key,omitempty"`
	ConsumerSecret string `json:"consumer_secret,omitempty"`
	RefreshToken   string `json:"refresh_token,omitempty"`
}

// storeConfig is the connection's non-secret settings.
type storeConfig struct {
	Shop     string `json:"shop,omitempty"`
	StoreURL string `json:"store_url,omitempty"`
	Email    string `json:"email,omitempty"`
}

// Connections lists the workspace's store and app connections, disconnected ones included.
func (s *Service) Connections(ctx context.Context, workspaceID uuid.UUID) ([]model.Connection, error) {
	rows, err := repository.NewRepository(s.pool).ListConnections(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	out := make([]model.Connection, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.Connection{
			Provider: r.Provider, Status: r.Status, AccountLabel: r.AccountLabel, LastCheckedAt: r.LastCheckedAt,
			LastError: r.LastError, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Service) connection(ctx context.Context, workspaceID uuid.UUID, provider string) (model.Connection, error) {
	list, err := s.Connections(ctx, workspaceID)
	if err != nil {
		return model.Connection{}, err
	}
	for _, c := range list {
		if c.Provider == provider {
			return c, nil
		}
	}
	return model.Connection{}, model.ErrNotConnected
}

// Connect checks a store's credentials against the store and saves them, replacing any before.
func (s *Service) Connect(ctx context.Context, workspaceID, userID uuid.UUID, provider string, in model.ConnectInput) (model.Connection, error) {
	var (
		creds storeCredentials
		cfg   storeConfig
		label string
		err   error
	)
	switch provider {
	case model.ProviderShopify:
		cfg.Shop, err = normalizeShop(in.ShopDomain)
		if err != nil {
			return model.Connection{}, err
		}
		creds.AccessToken = strings.TrimSpace(in.AccessToken)
		if creds.AccessToken == "" {
			return model.Connection{}, model.ErrMissingFields
		}
		label, err = s.checkShopify(ctx, cfg.Shop, creds.AccessToken)
	case model.ProviderWooCommerce:
		cfg.StoreURL, err = s.normalizeStoreURL(in.StoreURL)
		if err != nil {
			return model.Connection{}, err
		}
		creds.ConsumerKey, creds.ConsumerSecret = strings.TrimSpace(in.ConsumerKey), strings.TrimSpace(in.ConsumerSecret)
		if creds.ConsumerKey == "" || creds.ConsumerSecret == "" {
			return model.Connection{}, model.ErrMissingFields
		}
		label, err = s.checkWooCommerce(ctx, cfg.StoreURL, creds)
	default:
		return model.Connection{}, model.ErrUnknownProvider
	}
	if err != nil {
		return model.Connection{}, err
	}
	if err := s.saveConnection(ctx, workspaceID, userID, provider, label, cfg, creds); err != nil {
		return model.Connection{}, err
	}
	return s.connection(ctx, workspaceID, provider)
}

func (s *Service) saveConnection(ctx context.Context, workspaceID, userID uuid.UUID, provider, label string, cfg storeConfig, creds storeCredentials) error {
	rawCreds, err := json.Marshal(creds) //nolint:gosec // sealed on the next line; never stored or logged in the clear
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	sealed, err := s.box.seal(rawCreds)
	if err != nil {
		return err
	}
	rawCfg, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("new id: %w", err)
	}
	err = repository.NewRepository(s.pool).UpsertConnection(ctx, gen.UpsertConnectionParams{
		ID: id, WorkspaceID: workspaceID, Provider: provider, AccountLabel: label, Config: rawCfg,
		Credentials: sealed, ConnectedBy: &userID,
	})
	if err != nil {
		return fmt.Errorf("save connection: %w", err)
	}
	return nil
}

// TestConnection checks saved credentials again and records the result.
// A rejected or unreachable store is a result, not an error: the connection shows it.
func (s *Service) TestConnection(ctx context.Context, workspaceID uuid.UUID, provider string) (model.Connection, error) {
	repo := repository.NewRepository(s.pool)
	row, err := repo.GetConnectionCredentials(ctx, gen.GetConnectionCredentialsParams{WorkspaceID: workspaceID, Provider: provider})
	if repository.IsNoRows(err) {
		return model.Connection{}, model.ErrNotConnected
	}
	if err != nil {
		return model.Connection{}, fmt.Errorf("get connection: %w", err)
	}
	var (
		cfg   storeConfig
		creds storeCredentials
	)
	plain, err := s.box.open(row.Credentials)
	if err != nil {
		return model.Connection{}, err
	}
	if err := json.Unmarshal(plain, &creds); err != nil {
		return model.Connection{}, fmt.Errorf("read credentials: %w", err)
	}
	if err := json.Unmarshal(row.Config, &cfg); err != nil {
		return model.Connection{}, fmt.Errorf("read config: %w", err)
	}

	var checkErr error
	switch provider {
	case model.ProviderShopify:
		_, checkErr = s.checkShopify(ctx, cfg.Shop, creds.AccessToken)
	case model.ProviderWooCommerce:
		_, checkErr = s.checkWooCommerce(ctx, cfg.StoreURL, creds)
	case model.ProviderGoogleSheets:
		_, checkErr = s.refreshGoogle(ctx, creds.RefreshToken)
	default:
		return model.Connection{}, model.ErrUnknownProvider
	}
	params := gen.RecordConnectionCheckParams{Status: model.StatusConnected, WorkspaceID: workspaceID, Provider: provider}
	if checkErr != nil {
		msg := checkErr.Error()
		if ae := asAppErr(checkErr); ae != nil {
			msg = ae.Message
		} else if s.log != nil {
			s.log.WarnContext(ctx, "integration check failed", "provider", provider, "err", checkErr)
			msg = model.ErrStoreUnreachable.Message
		}
		params.Status, params.LastError = model.StatusError, &msg
	}
	if err := repo.RecordConnectionCheck(ctx, params); err != nil {
		return model.Connection{}, fmt.Errorf("record check: %w", err)
	}
	return s.connection(ctx, workspaceID, provider)
}

// Disconnect forgets a connection's credentials. The row stays, marked disconnected.
func (s *Service) Disconnect(ctx context.Context, workspaceID uuid.UUID, provider string) error {
	n, err := repository.NewRepository(s.pool).DisconnectConnection(ctx, gen.DisconnectConnectionParams{
		WorkspaceID: workspaceID, Provider: provider,
	})
	if err != nil {
		return fmt.Errorf("disconnect: %w", err)
	}
	if n == 0 {
		return model.ErrNotConnected
	}
	return nil
}

// normalizeShop accepts "greenleaf", "greenleaf.myshopify.com" or a full admin URL.
func normalizeShop(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	s, _, _ = strings.Cut(s, "/")
	if s != "" && !strings.Contains(s, ".") {
		s += ".myshopify.com"
	}
	if !shopDomainRe.MatchString(s) {
		return "", model.ErrInvalidShop
	}
	return s, nil
}

func (s *Service) normalizeStoreURL(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if s.checkTargetURL(raw) != nil {
		return "", model.ErrInvalidStoreURL
	}
	u, _ := url.Parse(raw)
	u.RawQuery, u.Fragment = "", ""
	return u.String(), nil
}

func (s *Service) checkShopify(ctx context.Context, shop, token string) (string, error) {
	base := "https://" + shop
	if s.cfg.ShopifyBaseURL != "" {
		base = s.cfg.ShopifyBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/admin/api/"+shopifyAPIVersion+"/shop.json", nil)
	if err != nil {
		return "", model.ErrInvalidShop
	}
	req.Header.Set("X-Shopify-Access-Token", token)
	var body struct {
		Shop struct {
			MyshopifyDomain string `json:"myshopify_domain"`
		} `json:"shop"`
	}
	if err := s.getJSON(req, &body); err != nil {
		return "", err
	}
	if body.Shop.MyshopifyDomain != "" {
		return body.Shop.MyshopifyDomain, nil
	}
	return shop, nil
}

func (s *Service) checkWooCommerce(ctx context.Context, storeURL string, creds storeCredentials) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, storeURL+"/wp-json/wc/v3/system_status", nil)
	if err != nil {
		return "", model.ErrInvalidStoreURL
	}
	req.SetBasicAuth(creds.ConsumerKey, creds.ConsumerSecret)
	if err := s.getJSON(req, &struct{}{}); err != nil {
		return "", err
	}
	u, _ := url.Parse(storeURL)
	return u.Host, nil
}

// getJSON runs a store check: 2xx decodes into v, 401/403 means the credentials were
// rejected, and anything else means the store couldn't be reached.
func (s *Service) getJSON(req *http.Request, v any) error {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		return model.ErrStoreUnreachable.Wrap(err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return model.ErrBadCredentials
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return model.ErrStoreUnreachable.Wrap(fmt.Errorf("status %d", resp.StatusCode))
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<20)).Decode(v); err != nil {
		return model.ErrStoreUnreachable.Wrap(err)
	}
	return nil
}
