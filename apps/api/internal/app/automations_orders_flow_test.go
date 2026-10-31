package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	automationsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
)

const shopifySecret = "shopify-secret"

// storeEnv is a workspace with a number, a template and a connected Shopify store.
type storeEnv struct {
	*inboxEnv
	stores   *fakeStores
	shopify  uuid.UUID
	template uuid.UUID
}

func newStoreEnv(t *testing.T) *storeEnv {
	t.Helper()
	v := newAutomationsEnv(t)
	stores := newFakeStores(t)
	t.Cleanup(stores.srv.Close)
	e := v.e
	e.a.allowPrivateTargets, e.a.shopifyURL = true, stores.srv.URL
	e.a.cfg.Integrations.ShopifyClientID, e.a.cfg.Integrations.ShopifyClientSecret = "shopify-client", shopifySecret
	rebuildIntegrations(t, e)
	s := &storeEnv{inboxEnv: v, stores: stores, shopify: uuid.Must(uuid.NewV7()), template: uuid.Must(uuid.NewV7())}
	e.exec(`INSERT INTO integration_connections (id, workspace_id, provider, status, account_label, config)
		VALUES ($1, $2, 'shopify', 'connected', 'greenleaf', $3)`, s.shopify, v.wsID, `{"shop":"`+v.wsID.String()+`.myshopify.com","oauth":true}`)
	e.exec(`INSERT INTO message_templates (id, workspace_id, name, language, category, status, components)
		VALUES ($1, $2, 'order_confirmed', 'en_US', 'utility', 'approved', $3)`, s.template, v.wsID,
		`{"header":{"type":"none"},"body":{"text":"Hi {{1}}, order {{2}} for {{3}}: {{4}}","examples":["a","b","c","d"]},"footer":{"text":""},"buttons":[],"auth":{}}`)
	return s
}

func (s *storeEnv) turnOnOrders(trigger string) {
	s.e.t.Helper()
	s.turnOn("order", map[string]any{"trigger": trigger, "template_id": s.template.String(), "variables": []map[string]any{
		{"source": "customer_first_name", "custom": ""},
		{"source": "order_number", "custom": ""},
		{"source": "order_total", "custom": ""},
		{"source": "tracking_url", "custom": ""},
	}})
}

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (s *storeEnv) post(path string, headers map[string]string, body []byte) int {
	s.e.t.Helper()
	req := httptest.NewRequestWithContext(s.e.t.Context(), http.MethodPost, path, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.e.h.ServeHTTP(rec, req)
	return rec.Code
}

func (s *storeEnv) order(topic, delivery string, body map[string]any) int {
	s.e.t.Helper()
	raw, _ := json.Marshal(body)
	return s.post("/v1/webhooks/commerce/shopify/"+s.shopify.String(), map[string]string{
		"X-Shopify-Topic": topic, "X-Shopify-Webhook-Id": delivery, "X-Shopify-Hmac-Sha256": sign(shopifySecret, raw),
	}, raw)
}

var shopifyOrder = map[string]any{
	"id": 4821, "name": "#4821", "total_price": "86.50", "currency": "USD", "order_status_url": "https://grnlf.co/t/4821",
	"customer": map[string]any{"first_name": "Daniel", "last_name": "Ortiz", "phone": "+13125550187"},
}

func TestOrderConfirmationStartsAChatWithTheTemplate(t *testing.T) {
	s := newStoreEnv(t)
	s.turnOnOrders("shopify_order_created")
	if code := s.order("orders/create", "d1", shopifyOrder); code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	s.order("orders/create", "d1", shopifyOrder) // Redelivered.
	s.order("orders/create", "d2", shopifyOrder) // Same order, new delivery ID.
	s.runAutomations()
	if s.automated() != 1 {
		t.Fatalf("automated = %d", s.automated())
	}
	if s.count(`SELECT count(*) FROM messages WHERE workspace_id = $1 AND type = 'template'
		AND body = 'Hi Daniel, order #4821 for 86.50 USD: https://grnlf.co/t/4821'`, s.wsID) != 1 {
		t.Fatal("template not filled")
	}
	if s.count(`SELECT count(*) FROM conversations WHERE workspace_id = $1 AND source = 'automation' AND last_inbound_at IS NULL`, s.wsID) != 1 {
		t.Fatal("no new chat")
	}
	if s.count(`SELECT count(*) FROM contacts WHERE workspace_id = $1 AND phone = '+13125550187' AND name = 'Daniel Ortiz'`, s.wsID) != 1 {
		t.Fatal("no contact")
	}
	if s.runSends(true) != 1 {
		t.Fatal("not delivered")
	}
}

func TestOrderWebhookRefusesABadSignature(t *testing.T) {
	s := newStoreEnv(t)
	s.turnOnOrders("shopify_order_created")
	raw, _ := json.Marshal(shopifyOrder)
	code := s.post("/v1/webhooks/commerce/shopify/"+s.shopify.String(), map[string]string{
		"X-Shopify-Topic": "orders/create", "X-Shopify-Webhook-Id": "d1", "X-Shopify-Hmac-Sha256": sign("wrong", raw),
	}, raw)
	if code != http.StatusUnauthorized {
		t.Fatalf("status %d", code)
	}
	if s.count(`SELECT count(*) FROM commerce_events WHERE workspace_id = $1`, s.wsID) != 0 {
		t.Fatal("stored an unsigned delivery")
	}
}

func TestOrderSkipsAndFailures(t *testing.T) {
	s := newStoreEnv(t)
	s.turnOnOrders("shopify_order_paid")
	s.order("orders/create", "d1", shopifyOrder) // Not the chosen trigger.
	s.runAutomations()
	if s.count(`SELECT count(*) FROM automation_runs WHERE workspace_id = $1`, s.wsID) != 0 {
		t.Fatal("wrong topic made a run")
	}

	noPhone := map[string]any{"id": 1, "name": "#1", "total_price": "1", "currency": "USD"}
	s.order("orders/paid", "d2", noPhone)
	s.runAutomations()
	if s.runs(automationsmodel.StateSkipped, automationsmodel.ReasonNoPhone) != 1 {
		t.Fatal("no_phone")
	}

	s.e.exec(`UPDATE contacts SET opt_in = 'out' WHERE workspace_id = $1`, s.wsID)
	s.e.exec(`INSERT INTO contacts (id, workspace_id, name, phone, source, opt_in) VALUES ($1, $2, 'Out', '+13125550100', 'inbox', 'out')`, uuid.Must(uuid.NewV7()), s.wsID)
	optedOut := map[string]any{"id": 2, "name": "#2", "total_price": "1", "currency": "USD", "customer": map[string]any{"first_name": "O", "phone": "+13125550100"}}
	s.order("orders/paid", "d3", optedOut)
	s.runAutomations()
	if s.runs(automationsmodel.StateSkipped, automationsmodel.ReasonOptedOut) != 1 {
		t.Fatal("opted_out")
	}

	s.e.exec(`UPDATE message_templates SET status = 'paused' WHERE id = $1`, s.template)
	s.order("orders/paid", "d4", shopifyOrder)
	s.runAutomations()
	if s.runs(automationsmodel.StateFailed, automationsmodel.ReasonTemplateUnavailable) != 1 {
		t.Fatal("template_unavailable")
	}
	if n := s.count(`SELECT count(*) FROM conversations c WHERE workspace_id = $1
		AND NOT EXISTS (SELECT 1 FROM messages m WHERE m.conversation_id = c.id)`, s.wsID); n != 0 {
		t.Fatalf("failed send left %d empty chats", n)
	}
	list := s.owner.do("GET", "/v1/workspace/automations", nil)
	if items(list)[4]["failed"].(float64) != 1 {
		t.Fatalf("card: %s", list.raw)
	}
}

func TestWooCommerceOrder(t *testing.T) {
	s := newStoreEnv(t)
	s.turnOnOrders("woocommerce_order_created")
	put := s.owner.do("PUT", "/v1/workspace/integrations/connections/woocommerce", map[string]any{
		"store_url": s.stores.srv.URL, "consumer_key": "ck_good", "consumer_secret": "cs_good",
	})
	expect(t, put, http.StatusOK)
	if len(s.stores.webhooks) != 1 {
		t.Fatalf("webhooks: %v", s.stores.webhooks)
	}
	hook := s.stores.webhooks[0]
	secret := hook["secret"].(string)
	if !strings.Contains(hook["delivery_url"].(string), "/v1/webhooks/commerce/woocommerce/") {
		t.Fatalf("delivery_url: %v", hook)
	}
	var conn uuid.UUID
	if err := s.e.a.db.QueryRow(context.Background(), `SELECT id FROM integration_connections WHERE workspace_id = $1 AND provider = 'woocommerce'`, s.wsID).Scan(&conn); err != nil {
		t.Fatal(err)
	}
	path := "/v1/webhooks/commerce/woocommerce/" + conn.String()
	if code := s.post(path, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, []byte("webhook_id=1")); code != http.StatusOK {
		t.Fatalf("ping: %d", code)
	}
	raw, _ := json.Marshal(map[string]any{
		"id": 77, "number": "77", "total": "12.00", "currency": "USD",
		"billing": map[string]any{"first_name": "Rina", "last_name": "Das", "phone": "+13125550155"},
	})
	if code := s.post(path, map[string]string{
		"X-WC-Webhook-Topic": "order.created", "X-WC-Webhook-Delivery-ID": "w1", "X-WC-Webhook-Signature": sign(secret, raw),
	}, raw); code != http.StatusOK {
		t.Fatalf("order: %d", code)
	}
	s.runAutomations()
	if s.automated() != 1 {
		t.Fatalf("automated = %d", s.automated())
	}
	if s.count(`SELECT count(*) FROM messages WHERE workspace_id = $1 AND body LIKE '%/my-account/view-order/77/'`, s.wsID) != 1 {
		t.Fatal("no order link")
	}
}

func TestShopifyPrivacyErasesRunContext(t *testing.T) {
	s := newStoreEnv(t)
	s.turnOnOrders("shopify_order_created")
	s.order("orders/create", "d1", shopifyOrder)
	s.runAutomations()
	var shop string
	if err := s.e.a.db.QueryRow(context.Background(), `SELECT config->>'shop' FROM integration_connections WHERE id = $1`, s.shopify).Scan(&shop); err != nil {
		t.Fatal(err)
	}
	post := func(topic string, body map[string]any) int {
		raw, _ := json.Marshal(body)
		return s.post("/v1/webhooks/shopify/privacy", map[string]string{"X-Shopify-Topic": topic, "X-Shopify-Hmac-Sha256": sign(shopifySecret, raw)}, raw)
	}
	if code := post("customers/data_request", map[string]any{"shop_domain": shop}); code != http.StatusOK {
		t.Fatalf("data request: %d", code)
	}
	if code := post("customers/redact", map[string]any{"shop_domain": shop, "customer": map[string]any{"phone": "+13125550187"}}); code != http.StatusOK {
		t.Fatalf("redact: %d", code)
	}
	if s.count(`SELECT count(*) FROM automation_runs WHERE workspace_id = $1 AND context <> '{}'`, s.wsID) != 0 {
		t.Fatal("run context kept")
	}
	if code := post("shop/redact", map[string]any{"shop_domain": shop}); code != http.StatusOK {
		t.Fatalf("shop redact: %d", code)
	}
	if s.count(`SELECT count(*) FROM commerce_events WHERE connection_id = $1`, s.shopify) != 0 {
		t.Fatal("events kept")
	}
}
