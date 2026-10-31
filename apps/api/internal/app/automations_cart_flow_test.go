package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	automationsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
)

func (s *storeEnv) turnOnCart(second bool) {
	s.e.t.Helper()
	s.turnOn("cart", map[string]any{
		"delay_minutes": 30, "template_id": s.template.String(), "discount_code": "COMEBACK10", "second_reminder": second,
		"variables": []map[string]any{
			{"source": "customer_first_name", "custom": ""},
			{"source": "discount_code", "custom": ""},
			{"source": "cart_total", "custom": ""},
			{"source": "checkout_url", "custom": ""},
		},
	})
}

func checkout(consent bool) map[string]any {
	return map[string]any{
		"token": "tok1", "total_price": "86.50", "currency": "USD", "abandoned_checkout_url": "https://shop.test/recover/tok1",
		"buyer_accepts_marketing": consent, "customer": map[string]any{"first_name": "Daniel", "last_name": "Ortiz", "phone": "+13125550187"},
	}
}

// due makes this workspace's waiting runs due and runs them.
func (s *storeEnv) due() {
	s.e.t.Helper()
	s.e.exec(`UPDATE automation_runs SET next_at = now() - interval '1 minute' WHERE workspace_id = $1 AND state = 'scheduled'`, s.wsID)
	if err := s.e.a.automations.Tick(context.Background()); err != nil {
		s.e.t.Fatal(err)
	}
	s.runAutomations()
}

func TestAbandonedCartReminderAndRecovery(t *testing.T) {
	s := newStoreEnv(t)
	s.turnOnCart(false)
	s.order("checkouts/create", "c1", checkout(true))
	s.runAutomations()
	if s.automated() != 0 || s.runs(automationsmodel.StateScheduled, "") != 1 {
		t.Fatal("should wait for the delay")
	}
	var first time.Time
	read := func() (at time.Time) {
		if err := s.e.a.db.QueryRow(context.Background(), `SELECT next_at FROM automation_runs WHERE workspace_id = $1`, s.wsID).Scan(&at); err != nil {
			t.Fatal(err)
		}
		return at
	}
	first = read()
	time.Sleep(5 * time.Millisecond)
	s.order("checkouts/update", "c2", checkout(true))
	if !read().After(first) || s.count(`SELECT count(*) FROM automation_runs WHERE workspace_id = $1`, s.wsID) != 1 {
		t.Fatal("an update should restart the one wait")
	}
	s.due()
	if s.count(`SELECT count(*) FROM messages WHERE workspace_id = $1 AND type = 'template'
		AND body = 'Hi Daniel, order COMEBACK10 for 86.50 USD: https://shop.test/recover/tok1'`, s.wsID) != 1 {
		t.Fatalf("reminder not sent: automated = %d", s.automated())
	}
	bought := map[string]any{
		"id": 9, "name": "#9", "total_price": "86.50", "currency": "USD", "checkout_token": "tok1",
		"customer": map[string]any{"first_name": "Daniel", "phone": "+13125550187"},
	}
	s.order("orders/create", "o1", bought)
	s.order("orders/paid", "o2", bought)
	card := items(s.owner.do("GET", "/v1/workspace/automations", nil))[3]
	if card["recovered"].(float64) != 1 || card["recovered_minor"].(float64) != 8650 || card["sent"].(float64) != 1 {
		t.Fatalf("card: %v", card)
	}
}

func TestCartCancelsWhenTheOrderComesFirst(t *testing.T) {
	s := newStoreEnv(t)
	s.turnOnCart(true)
	s.order("checkouts/create", "c1", checkout(true))
	s.order("orders/create", "o1", map[string]any{
		"id": 9, "name": "#9", "total_price": "86.50", "currency": "USD", "checkout_token": "tok1",
	})
	s.due()
	if s.automated() != 0 || s.runs(automationsmodel.StateCancelled, "order_placed") != 1 {
		t.Fatalf("automated = %d", s.automated())
	}
}

func TestCartNeedsConsent(t *testing.T) {
	s := newStoreEnv(t)
	s.turnOnCart(false)
	s.order("checkouts/create", "c1", checkout(false))
	s.due()
	if s.automated() != 0 || s.runs(automationsmodel.StateSkipped, automationsmodel.ReasonNoConsent) != 1 {
		t.Fatalf("automated = %d", s.automated())
	}
}

func TestCartSecondReminderStopsAtTwo(t *testing.T) {
	s := newStoreEnv(t)
	s.turnOnCart(true)
	s.order("checkouts/create", "c1", checkout(true))
	s.due()
	s.runSends(true)
	if s.automated() != 1 || s.runs(automationsmodel.StateScheduled, "") != 1 {
		t.Fatal("second reminder should be waiting")
	}
	s.order("checkouts/update", "c2", checkout(true))
	var step int
	if err := s.e.a.db.QueryRow(context.Background(), `SELECT step FROM automation_runs WHERE workspace_id = $1`, s.wsID).Scan(&step); err != nil || step != 1 {
		t.Fatalf("step = %d, %v", step, err)
	}
	s.due()
	s.due()
	if s.automated() != 2 || s.runs(automationsmodel.StateDone, "") != 1 {
		t.Fatalf("automated = %d", s.automated())
	}
}

func TestPausingCancelsAWaitingCart(t *testing.T) {
	s := newStoreEnv(t)
	s.turnOnCart(false)
	s.order("checkouts/create", "c1", checkout(true))
	expect(t, s.owner.do("PUT", "/v1/workspace/automations/cart/enabled", map[string]any{"enabled": false}), http.StatusOK)
	s.due()
	if s.automated() != 0 || s.runs(automationsmodel.StateCancelled, automationsmodel.ReasonPaused) != 1 {
		t.Fatalf("automated = %d", s.automated())
	}
}

func TestShopifySubscribesToCheckouts(t *testing.T) {
	s := newStoreEnv(t)
	expect(t, s.owner.do("PUT", "/v1/workspace/integrations/connections/shopify", map[string]any{
		"shop_domain": "greenleaf.myshopify.com", "access_token": "good",
	}), http.StatusOK)
	s.e.exec(`UPDATE integration_connections SET config = config || '{"oauth": true}' WHERE id = $1`, s.shopify)
	expect(t, s.owner.do("POST", "/v1/workspace/integrations/connections/shopify/test", nil), http.StatusOK)
	var topics []string
	for _, w := range s.stores.webhooks {
		raw, _ := json.Marshal(w)
		topics = append(topics, string(raw))
	}
	if all := strings.Join(topics, " "); !strings.Contains(all, "checkouts/update") || !strings.Contains(all, "orders/paid") {
		t.Fatalf("topics: %v", topics)
	}
}
