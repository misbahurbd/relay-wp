package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// CommercePath is where stores post order webhooks, under API_BASE_URL.
const CommercePath = "/v1/webhooks/commerce"

// CommerceSink hears about store events. Implemented in internal/app on top of
// features/automations.
type CommerceSink interface {
	// OnOrder runs in the transaction that records the delivery, so a failure
	// makes the store send it again.
	OnOrder(ctx context.Context, tx pgx.Tx, ev model.OrderEvent) error
	// OnCheckout runs in the same kind of transaction.
	OnCheckout(ctx context.Context, tx pgx.Tx, ev model.CheckoutEvent) error
	// RedactCustomer erases what automations hold about one customer.
	RedactCustomer(ctx context.Context, workspaceID uuid.UUID, phone string) error
	// RedactStore erases what automations hold from a store's events.
	RedactStore(ctx context.Context, workspaceID uuid.UUID) error
}

// SetCommerceSink connects the sink, which is built after this service.
func (s *Service) SetCommerceSink(sink CommerceSink) { s.sink = sink }

var shopifyTopics = []string{"orders/create", "orders/paid", "checkouts/create", "checkouts/update"}

// orderTopics maps a provider's topic to what happened to the order.
var orderTopics = map[string]string{
	"orders/create": model.OrderCreated,
	"orders/paid":   model.OrderPaid,
	"order.created": model.OrderCreated,
}

func validSignature(secret string, body []byte, signature string) bool {
	got, err := base64.StdEncoding.DecodeString(signature)
	if err != nil || secret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil))
}

// CommerceWebhook verifies one store delivery and handles it once. Signature
// comes first: nothing unsigned touches the database beyond finding the store.
func (s *Service) CommerceWebhook(ctx context.Context, provider string, connectionID uuid.UUID, in model.CommerceDelivery) error {
	r := repository.NewRepository(s.pool)
	conn, err := r.GetConnectionForWebhook(ctx, gen.GetConnectionForWebhookParams{ID: connectionID, Provider: provider})
	if repository.IsNoRows(err) {
		return model.ErrBadStoreSignature
	}
	if err != nil {
		return fmt.Errorf("get connection: %w", err)
	}
	// WooCommerce pings a new webhook with an unsigned form post and no topic.
	if provider == model.ProviderWooCommerce && in.Topic == "" {
		return nil
	}
	secret, cfg, err := s.webhookSecret(ctx, conn.WorkspaceID, provider)
	if err != nil {
		return err
	}
	if !validSignature(secret, in.Body, in.Signature) {
		return model.ErrBadStoreSignature
	}
	if in.Topic == "checkouts/create" || in.Topic == "checkouts/update" {
		return s.checkout(ctx, conn.ID, conn.WorkspaceID, in)
	}
	status, known := orderTopics[in.Topic]
	if !known {
		return nil
	}
	ev, err := parseOrder(provider, in.Body)
	if err != nil {
		return err
	}
	ev.WorkspaceID, ev.Provider, ev.Status = conn.WorkspaceID, provider, status
	if ev.StatusURL == "" && cfg.StoreURL != "" {
		// WooCommerce's payload has no status link; this is the customer's order page.
		ev.StatusURL = strings.TrimRight(cfg.StoreURL, "/") + "/my-account/view-order/" + ev.OrderID + "/"
	}
	delivery := in.DeliveryID
	if delivery == "" {
		delivery = in.Topic + ":" + ev.OrderID
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		rows, err := repository.NewRepository(tx).InsertCommerceEvent(ctx, gen.InsertCommerceEventParams{
			ConnectionID: conn.ID, DeliveryID: delivery, WorkspaceID: conn.WorkspaceID, Topic: in.Topic,
		})
		if err != nil {
			return fmt.Errorf("insert commerce event: %w", err)
		}
		if len(rows) == 0 || s.sink == nil {
			return nil
		}
		return s.sink.OnOrder(ctx, tx, ev)
	})
}

// checkout records one delivery about a checkout and passes it on.
func (s *Service) checkout(ctx context.Context, connectionID, workspaceID uuid.UUID, in model.CommerceDelivery) error {
	ev, completed, err := parseShopifyCheckout(in.Body)
	if err != nil {
		return err
	}
	// An order follows a finished checkout and closes its run.
	if completed || ev.Token == "" {
		return nil
	}
	ev.WorkspaceID = workspaceID
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if in.DeliveryID != "" {
			rows, err := repository.NewRepository(tx).InsertCommerceEvent(ctx, gen.InsertCommerceEventParams{
				ConnectionID: connectionID, DeliveryID: in.DeliveryID, WorkspaceID: workspaceID, Topic: in.Topic,
			})
			if err != nil {
				return fmt.Errorf("insert commerce event: %w", err)
			}
			if len(rows) == 0 {
				return nil
			}
		}
		if s.sink == nil {
			return nil
		}
		return s.sink.OnCheckout(ctx, tx, ev)
	})
}

func (s *Service) webhookSecret(ctx context.Context, workspaceID uuid.UUID, provider string) (string, storeConfig, error) {
	var cfg storeConfig
	if provider == model.ProviderShopify {
		return s.cfg.Shopify.ClientSecret, cfg, nil
	}
	row, err := repository.NewRepository(s.pool).GetConnectionCredentials(ctx, gen.GetConnectionCredentialsParams{WorkspaceID: workspaceID, Provider: provider})
	if repository.IsNoRows(err) {
		return "", cfg, model.ErrBadStoreSignature
	}
	if err != nil {
		return "", cfg, fmt.Errorf("get connection: %w", err)
	}
	plain, err := s.box.open(row.Credentials)
	if err != nil {
		return "", cfg, err
	}
	var creds storeCredentials
	if err := json.Unmarshal(plain, &creds); err != nil {
		return "", cfg, fmt.Errorf("read credentials: %w", err)
	}
	if err := json.Unmarshal(row.Config, &cfg); err != nil {
		return "", cfg, fmt.Errorf("read config: %w", err)
	}
	return creds.WebhookSecret, cfg, nil
}

func parseOrder(provider string, body []byte) (model.OrderEvent, error) {
	if provider == model.ProviderShopify {
		return parseShopifyOrder(body)
	}
	return parseWooOrder(body)
}

func parseShopifyOrder(body []byte) (model.OrderEvent, error) {
	type person struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Phone     string `json:"phone"`
	}
	var o struct {
		ID              json.Number `json:"id"`
		Name            string      `json:"name"`
		TotalPrice      string      `json:"total_price"`
		Currency        string      `json:"currency"`
		Phone           string      `json:"phone"`
		OrderStatusURL  string      `json:"order_status_url"`
		Customer        *person     `json:"customer"`
		BillingAddress  *person     `json:"billing_address"`
		ShippingAddress *person     `json:"shipping_address"`
		CheckoutToken   string      `json:"checkout_token"`
	}
	if err := json.Unmarshal(body, &o); err != nil {
		return model.OrderEvent{}, model.ErrBadStorePayload.Wrap(err)
	}
	ev := model.OrderEvent{
		OrderID: o.ID.String(), Number: o.Name, Total: strings.TrimSpace(o.TotalPrice + " " + o.Currency),
		Phone: o.Phone, StatusURL: o.OrderStatusURL, CheckoutToken: o.CheckoutToken,
		AmountMinor: minorUnits(o.TotalPrice), Currency: o.Currency,
	}
	for _, p := range []*person{o.Customer, o.ShippingAddress, o.BillingAddress} {
		if p == nil {
			continue
		}
		if ev.Phone == "" {
			ev.Phone = p.Phone
		}
		if ev.FirstName == "" {
			ev.FirstName, ev.LastName = p.FirstName, p.LastName
		}
	}
	return ev, nil
}

// minorUnits reads a store's decimal amount, like "86.50", as 8650.
func minorUnits(amount string) int64 {
	whole, frac, _ := strings.Cut(strings.TrimSpace(amount), ".")
	frac = (frac + "00")[:2]
	n, err := strconv.ParseInt(whole+frac, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func parseShopifyCheckout(body []byte) (model.CheckoutEvent, bool, error) {
	type person struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Phone     string `json:"phone"`
	}
	var c struct {
		Token                 string  `json:"token"`
		Phone                 string  `json:"phone"`
		TotalPrice            string  `json:"total_price"`
		Currency              string  `json:"currency"`
		AbandonedCheckoutURL  string  `json:"abandoned_checkout_url"`
		BuyerAcceptsMarketing bool    `json:"buyer_accepts_marketing"`
		CompletedAt           *string `json:"completed_at"`
		Customer              *person `json:"customer"`
		ShippingAddress       *person `json:"shipping_address"`
		BillingAddress        *person `json:"billing_address"`
	}
	if err := json.Unmarshal(body, &c); err != nil {
		return model.CheckoutEvent{}, false, model.ErrBadStorePayload.Wrap(err)
	}
	ev := model.CheckoutEvent{
		Token: c.Token, Phone: c.Phone, Total: strings.TrimSpace(c.TotalPrice + " " + c.Currency),
		CheckoutURL: c.AbandonedCheckoutURL, Consent: c.BuyerAcceptsMarketing,
	}
	for _, p := range []*person{c.Customer, c.ShippingAddress, c.BillingAddress} {
		if p == nil {
			continue
		}
		if ev.Phone == "" {
			ev.Phone = p.Phone
		}
		if ev.FirstName == "" {
			ev.FirstName, ev.LastName = p.FirstName, p.LastName
		}
	}
	return ev, c.CompletedAt != nil, nil
}

func parseWooOrder(body []byte) (model.OrderEvent, error) {
	var o struct {
		ID       json.Number `json:"id"`
		Number   string      `json:"number"`
		Total    string      `json:"total"`
		Currency string      `json:"currency"`
		Billing  struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Phone     string `json:"phone"`
		} `json:"billing"`
	}
	if err := json.Unmarshal(body, &o); err != nil {
		return model.OrderEvent{}, model.ErrBadStorePayload.Wrap(err)
	}
	return model.OrderEvent{
		OrderID: o.ID.String(), Number: o.Number, Total: strings.TrimSpace(o.Total + " " + o.Currency),
		Phone: o.Billing.Phone, FirstName: o.Billing.FirstName, LastName: o.Billing.LastName,
		AmountMinor: minorUnits(o.Total), Currency: o.Currency,
	}, nil
}

// ShopifyPrivacy handles Shopify's privacy webhooks, which name a shop rather
// than one of our connections.
func (s *Service) ShopifyPrivacy(ctx context.Context, in model.CommerceDelivery) error {
	if !validSignature(s.cfg.Shopify.ClientSecret, in.Body, in.Signature) {
		return model.ErrBadStoreSignature
	}
	var p struct {
		ShopDomain string `json:"shop_domain"`
		Customer   struct {
			Phone string `json:"phone"`
		} `json:"customer"`
	}
	if err := json.Unmarshal(in.Body, &p); err != nil {
		return model.ErrBadStorePayload.Wrap(err)
	}
	r := repository.NewRepository(s.pool)
	conn, err := r.GetConnectionByShop(ctx, p.ShopDomain)
	if repository.IsNoRows(err) {
		return nil // We hold nothing for a store that isn't connected.
	}
	if err != nil {
		return fmt.Errorf("get connection: %w", err)
	}
	switch in.Topic {
	case "customers/redact":
		if s.sink == nil || p.Customer.Phone == "" {
			return nil
		}
		return s.sink.RedactCustomer(ctx, conn.WorkspaceID, p.Customer.Phone)
	case "shop/redact":
		if err := r.DeleteCommerceEvents(ctx, conn.ID); err != nil {
			return fmt.Errorf("delete commerce events: %w", err)
		}
		if s.sink == nil {
			return nil
		}
		return s.sink.RedactStore(ctx, conn.WorkspaceID)
	case "customers/data_request":
		s.log.WarnContext(ctx, "shopify customer data request needs handling by hand", "workspace_id", conn.WorkspaceID)
	}
	return nil
}

// registerStoreWebhooks subscribes the store to orders. Doing it twice changes
// nothing, so Test connection also repairs an older store.
func (s *Service) registerStoreWebhooks(ctx context.Context, workspaceID uuid.UUID, provider string, cfg storeConfig, creds storeCredentials) error {
	id, err := repository.NewRepository(s.pool).GetConnectionID(ctx, gen.GetConnectionIDParams{WorkspaceID: workspaceID, Provider: provider})
	if err != nil {
		return fmt.Errorf("get connection id: %w", err)
	}
	address := strings.TrimRight(s.cfg.APIBaseURL, "/") + CommercePath + "/" + provider + "/" + id.String()
	switch provider {
	case model.ProviderShopify:
		// A pasted token belongs to another app, whose signatures we couldn't check.
		if !cfg.OAuth {
			return nil
		}
		return s.registerShopify(ctx, cfg.Shop, creds.AccessToken, address)
	case model.ProviderWooCommerce:
		if creds.WebhookSecret == "" {
			return model.ErrStoreNeedsReconnect
		}
		return s.registerWoo(ctx, cfg.StoreURL, creds, address)
	}
	return nil
}

func (s *Service) registerShopify(ctx context.Context, shop, token, address string) error {
	base := "https://" + shop
	if s.cfg.ShopifyBaseURL != "" {
		base = s.cfg.ShopifyBaseURL
	}
	for _, topic := range shopifyTopics {
		body, err := json.Marshal(map[string]any{"webhook": map[string]string{"topic": topic, "address": address, "format": "json"}})
		if err != nil {
			return fmt.Errorf("encode webhook: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/admin/api/"+shopifyAPIVersion+"/webhooks.json", bytes.NewReader(body))
		if err != nil {
			return model.ErrInvalidShop
		}
		req.Header.Set("X-Shopify-Access-Token", token)
		req.Header.Set("Content-Type", "application/json")
		if err := s.postStore(req); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) registerWoo(ctx context.Context, storeURL string, creds storeCredentials, address string) error {
	url := strings.TrimRight(storeURL, "/") + "/wp-json/wc/v3/webhooks"
	list, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return model.ErrInvalidStoreURL
	}
	list.SetBasicAuth(creds.ConsumerKey, creds.ConsumerSecret)
	var existing []struct {
		DeliveryURL string `json:"delivery_url"`
	}
	if err := s.getJSON(list, &existing); err != nil {
		return err
	}
	for _, w := range existing {
		if w.DeliveryURL == address {
			return nil
		}
	}
	body, err := json.Marshal(map[string]string{
		"name": "Relay orders", "topic": "order.created", "delivery_url": address, "secret": creds.WebhookSecret,
	})
	if err != nil {
		return fmt.Errorf("encode webhook: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return model.ErrInvalidStoreURL
	}
	req.SetBasicAuth(creds.ConsumerKey, creds.ConsumerSecret)
	req.Header.Set("Content-Type", "application/json")
	return s.postStore(req)
}

// postStore creates a webhook. 422 means Shopify already has this one.
func (s *Service) postStore(req *http.Request) error {
	resp, err := s.client.Do(req)
	if err != nil {
		return model.ErrStoreUnreachable.Wrap(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 300 || resp.StatusCode == http.StatusUnprocessableEntity {
		return nil
	}
	return model.ErrStoreNeedsReconnect
}
