package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	integrationsservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/service"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// fakeStores stands in for Shopify, WooCommerce and Google. "good" credentials pass.
type fakeStores struct {
	srv *httptest.Server
}

func newFakeStores(t *testing.T) *fakeStores {
	f := &fakeStores{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reply := func(status int, v any) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(v)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/shop.json"):
			if r.Header.Get("X-Shopify-Access-Token") != "good" {
				reply(http.StatusUnauthorized, map[string]any{"errors": "Invalid API key"})
				return
			}
			reply(http.StatusOK, map[string]any{"shop": map[string]any{"myshopify_domain": "greenleaf.myshopify.com"}})
		case strings.HasSuffix(r.URL.Path, "/wp-json/wc/v3/system_status"):
			if k, s, ok := r.BasicAuth(); !ok || k != "ck_good" || s != "cs_good" {
				reply(http.StatusUnauthorized, map[string]any{"code": "woocommerce_rest_cannot_view"})
				return
			}
			reply(http.StatusOK, map[string]any{"environment": map[string]any{}})
		case r.URL.Path == "/admin/oauth/access_token":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["code"] != "good-code" || body["client_secret"] != "shopify-secret" {
				reply(http.StatusBadRequest, map[string]any{"error": "invalid_request"})
				return
			}
			reply(http.StatusOK, map[string]any{"access_token": "good", "scope": "read_customers,read_orders"})
		case r.URL.Path == "/google/token":
			_ = r.ParseForm()
			if r.PostForm.Get("code") != "good-code" && r.PostForm.Get("refresh_token") != "refresh-1" {
				reply(http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
				return
			}
			reply(http.StatusOK, map[string]any{"access_token": "access-1", "refresh_token": "refresh-1"})
		case r.URL.Path == "/google/userinfo":
			reply(http.StatusOK, map[string]any{"email": "sheets@greenleaf.test"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// receiver is a webhook endpoint that records what it gets and answers with status.
type receiver struct {
	srv    *httptest.Server
	mu     sync.Mutex
	status int
	got    []*http.Request
	bodies [][]byte
}

func newReceiver(t *testing.T) *receiver {
	rc := &receiver{status: http.StatusOK}
	rc.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rc.mu.Lock()
		defer rc.mu.Unlock()
		rc.got, rc.bodies = append(rc.got, r), append(rc.bodies, body)
		w.WriteHeader(rc.status)
	}))
	t.Cleanup(rc.srv.Close)
	return rc
}

func (rc *receiver) setStatus(code int) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.status = code
}

func (rc *receiver) last() (*http.Request, []byte) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if len(rc.got) == 0 {
		return nil, nil
	}
	return rc.got[len(rc.got)-1], rc.bodies[len(rc.bodies)-1]
}

// newIntegrationsEnv is newAuthEnv with store and Google calls pointed at fakes.
func newIntegrationsEnv(t *testing.T) (*authEnv, *fakeStores) {
	t.Helper()
	e := newAuthEnv(t)
	stores := newFakeStores(t)
	e.a.allowPrivateTargets, e.a.shopifyURL, e.a.googleURL = true, stores.srv.URL, stores.srv.URL+"/google"
	e.a.cfg.Auth.GoogleClientID, e.a.cfg.Auth.GoogleClientSecret = "google-client", "google-secret"
	e.a.cfg.Integrations.ShopifyClientID, e.a.cfg.Integrations.ShopifyClientSecret = "shopify-client", "shopify-secret"
	rebuildIntegrations(t, e)
	return e, stores
}

// rebuildIntegrations rebuilds the integrations service and handler after a config change.
func rebuildIntegrations(t *testing.T, e *authEnv) {
	t.Helper()
	if err := e.a.buildIntegrations(); err != nil {
		t.Fatal(err)
	}
	h, err := e.a.Handler()
	if err != nil {
		t.Fatal(err)
	}
	e.h = h
}

// apiClient calls the API with an API key, the way a customer's server would.
func (e *authEnv) apiClient(key, workspace string) *browser {
	b := e.browser("relay-test-client")
	b.bearer = key
	return b.in(workspace)
}

// runDeliveries works every queued webhook_deliver job once, as River would.
func (e *authEnv) runDeliveries(final bool) int {
	e.t.Helper()
	rows, err := e.a.db.Query(context.Background(), `SELECT args FROM river_job WHERE kind = 'webhook_deliver' AND state = 'available'`)
	if err != nil {
		e.t.Fatal(err)
	}
	var jobs []struct {
		WorkspaceID uuid.UUID `json:"workspace_id"`
		DeliveryID  uuid.UUID `json:"delivery_id"`
	}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			e.t.Fatal(err)
		}
		var j struct {
			WorkspaceID uuid.UUID `json:"workspace_id"`
			DeliveryID  uuid.UUID `json:"delivery_id"`
		}
		_ = json.Unmarshal(raw, &j)
		jobs = append(jobs, j)
	}
	rows.Close()
	e.exec(`UPDATE river_job SET state = 'completed', finalized_at = now() WHERE kind = 'webhook_deliver' AND state = 'available'`)
	for _, j := range jobs {
		_ = e.a.integrations.Deliver(context.Background(), j.WorkspaceID, j.DeliveryID, final)
	}
	return len(jobs)
}

func TestAPIKeys(t *testing.T) {
	e, _ := newIntegrationsEnv(t)
	w := newTeamWorkspace(e)
	base := "/v1/workspace/integrations/api-keys"

	created := w.owner.do("POST", base, map[string]any{"name": "Website orders"})
	expect(t, created, http.StatusCreated)
	key := created.body["key"].(string)
	if !strings.HasPrefix(key, "relay_live_") {
		t.Fatalf("key %q", key)
	}
	k := created.body["api_key"].(map[string]any)
	if k["last4"] != key[len(key)-4:] || !strings.HasPrefix(key, k["prefix"].(string)) || k["created_by"].(map[string]any)["name"] == "" {
		t.Fatalf("listing doesn't match the key: %s", created.raw)
	}
	list := w.owner.do("GET", base, nil)
	expect(t, list, http.StatusOK)
	if strings.Contains(list.raw, key) || len(list.body["keys"].([]any)) != 1 {
		t.Fatalf("list leaks the key or misses it: %s", list.raw)
	}

	// The key acts as its creator in its own workspace only.
	api := e.apiClient(key, w.id)
	expect(t, api.do("GET", "/v1/workspace/me/access", nil), http.StatusOK)
	other := newTeamWorkspace(e)
	expect(t, e.apiClient(key, other.id).do("GET", "/v1/workspace/me/access", nil), http.StatusNotFound, "workspace_not_found")
	expect(t, e.apiClient(key, "").do("GET", "/v1/workspace/me/access", nil), http.StatusUnprocessableEntity, "workspace_required")
	expect(t, api.do("GET", "/v1/auth/sessions", nil), http.StatusForbidden, "api_key_not_allowed")
	// Keys can't make more keys or webhooks.
	expect(t, api.do("POST", base, map[string]any{"name": "sneaky"}), http.StatusForbidden, "session_required")
	expect(t, e.apiClient("relay_live_nope", w.id).do("GET", "/v1/workspace/me/access", nil), http.StatusUnauthorized, "invalid_api_key")

	var used *time.Time
	if err := e.a.db.QueryRow(context.Background(), `SELECT last_used_at FROM api_keys WHERE id = $1`, k["id"]).Scan(&used); err != nil || used == nil {
		t.Fatalf("last_used_at not set: %v", err)
	}

	// Roll: the old key stops at once, the new one works.
	rolled := w.owner.do("POST", base+"/"+k["id"].(string)+"/roll", nil)
	expect(t, rolled, http.StatusCreated)
	newKey := rolled.body["key"].(string)
	expect(t, api.do("GET", "/v1/workspace/me/access", nil), http.StatusUnauthorized, "invalid_api_key")
	expect(t, e.apiClient(newKey, w.id).do("GET", "/v1/workspace/me/access", nil), http.StatusOK)
	if rolled.body["api_key"].(map[string]any)["name"] != "Website orders" {
		t.Fatalf("roll lost the name: %s", rolled.raw)
	}

	newID := rolled.body["api_key"].(map[string]any)["id"].(string)
	expect(t, w.owner.do("DELETE", base+"/"+newID, nil), http.StatusNoContent)
	expect(t, e.apiClient(newKey, w.id).do("GET", "/v1/workspace/me/access", nil), http.StatusUnauthorized)
	expect(t, w.owner.do("DELETE", base+"/"+newID, nil), http.StatusNotFound, "api_key_not_found")

	// A viewer can see the catalog but not keys or webhooks.
	viewer, _ := w.join("viewer")
	expect(t, viewer.do("GET", "/v1/workspace/integrations", nil), http.StatusOK)
	expect(t, viewer.do("GET", base, nil), http.StatusForbidden, "permission_denied")
	expect(t, viewer.do("GET", "/v1/workspace/integrations/webhooks", nil), http.StatusForbidden, "permission_denied")

	// App keys mark their app connected.
	expect(t, w.owner.do("POST", base, map[string]any{"name": "Zapier", "app": "zapier"}), http.StatusCreated)
	o := w.owner.do("GET", "/v1/workspace/integrations", nil)
	expect(t, o, http.StatusOK)
	apps := o.body["apps"].([]any)
	if len(apps) != 1 || apps[0].(map[string]any)["app"] != "zapier" || o.body["api_keys"].(map[string]any)["active"].(float64) != 1 {
		t.Fatalf("overview: %s", o.raw)
	}
}

func TestWebhooks(t *testing.T) {
	e, _ := newIntegrationsEnv(t)
	w := newTeamWorkspace(e)
	rc := newReceiver(t)
	base := "/v1/workspace/integrations/webhooks"

	expect(t, w.owner.do("POST", base, map[string]any{"url": "ftp://x.test", "events": []string{"contact.created"}, "enabled": true}),
		http.StatusUnprocessableEntity)
	expect(t, w.owner.do("POST", base, map[string]any{"url": rc.srv.URL, "events": []string{"nope"}, "enabled": true}),
		http.StatusUnprocessableEntity, "unknown_event")

	created := w.owner.do("POST", base, map[string]any{
		"url": rc.srv.URL + "/hooks", "description": "CRM", "events": []string{"contact.created", "message.received"}, "enabled": true,
	})
	expect(t, created, http.StatusCreated)
	secret := created.body["secret"].(string)
	id := created.body["endpoint"].(map[string]any)["id"].(string)
	got := w.owner.do("GET", base+"/"+id+"/secret", nil)
	expect(t, got, http.StatusOK)
	if got.body["secret"] != secret || !strings.HasPrefix(secret, "whsec_") {
		t.Fatalf("secret %s vs %s", got.body["secret"], secret)
	}

	// Test event: sent now, signed, logged.
	test := w.owner.do("POST", base+"/"+id+"/test", nil)
	expect(t, test, http.StatusOK)
	if test.body["status"] != "succeeded" || test.body["event_type"] != "ping" {
		t.Fatalf("test delivery: %s", test.raw)
	}
	req, body := rc.last()
	checkSignature(t, req, body, secret)

	// Publish in a transaction: one delivery for the subscribed endpoint, none for others.
	wsID := uuid.MustParse(w.id)
	publish := func(event string) {
		err := database.WithTx(context.Background(), e.a.db, func(tx pgx.Tx) error {
			return e.a.integrations.Publish(context.Background(), tx, wsID, event, map[string]string{"contact_id": "c1"})
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	publish("contact.created")
	publish("broadcast.completed") // not subscribed
	if n := e.runDeliveries(false); n != 1 {
		t.Fatalf("%d deliveries queued, want 1", n)
	}
	req, body = rc.last()
	checkSignature(t, req, body, secret)
	if req.Header.Get("Relay-Event") != "contact.created" || req.URL.Path != "/hooks" {
		t.Fatalf("delivered %s to %s", req.Header.Get("Relay-Event"), req.URL.Path)
	}
	var ev map[string]any
	_ = json.Unmarshal(body, &ev)
	if ev["type"] != "contact.created" || ev["workspace_id"] != w.id || ev["data"].(map[string]any)["contact_id"] != "c1" {
		t.Fatalf("event body: %s", body)
	}

	// A failing receiver: retried, then failed for good, then redelivered.
	rc.setStatus(http.StatusInternalServerError)
	publish("message.received")
	e.runDeliveries(false)
	deliveries := w.owner.do("GET", base+"/"+id+"/deliveries?limit=10", nil)
	expect(t, deliveries, http.StatusOK)
	latest := deliveries.body["deliveries"].([]any)[0].(map[string]any)
	if latest["status"] != "pending" || latest["response_status"].(float64) != 500 || latest["error"] != "Got HTTP 500" {
		t.Fatalf("after a failed try: %v", latest)
	}
	var deliveryID uuid.UUID
	_ = deliveryID.UnmarshalText([]byte(latest["id"].(string)))
	_ = e.a.integrations.Deliver(context.Background(), wsID, deliveryID, true)
	expect(t, w.owner.do("POST", base+"/"+id+"/deliveries/"+deliveryID.String()+"/redeliver", nil), http.StatusAccepted)
	rc.setStatus(http.StatusOK)
	e.runDeliveries(false)
	deliveries = w.owner.do("GET", base+"/"+id+"/deliveries?limit=1", nil)
	if d := deliveries.body["deliveries"].([]any)[0].(map[string]any); d["status"] != "succeeded" || d["attempts"].(float64) != 3 {
		t.Fatalf("redelivered: %v", d)
	}
	if deliveries.body["next_cursor"] == nil {
		t.Fatal("expected another page")
	}

	list := w.owner.do("GET", base, nil)
	expect(t, list, http.StatusOK)
	ep := list.body["endpoints"].([]any)[0].(map[string]any)
	if ep["week_total"].(float64) != 3 || ep["week_succeeded"].(float64) != 3 || ep["consecutive_failures"].(float64) != 0 {
		t.Fatalf("endpoint health: %v", ep)
	}

	// Rotating the secret signs new deliveries with the new one.
	rotated := w.owner.do("POST", base+"/"+id+"/secret/rotate", nil)
	expect(t, rotated, http.StatusOK)
	expect(t, w.owner.do("POST", base+"/"+id+"/test", nil), http.StatusOK)
	req, body = rc.last()
	checkSignature(t, req, body, rotated.body["secret"].(string))

	// Turned off: no new deliveries.
	upd := w.owner.do("PATCH", base+"/"+id, map[string]any{"url": rc.srv.URL, "events": []string{"contact.created"}, "enabled": false})
	expect(t, upd, http.StatusOK)
	publish("contact.created")
	if n := e.runDeliveries(false); n != 0 {
		t.Fatalf("%d deliveries to a turned-off endpoint", n)
	}

	expect(t, w.owner.do("DELETE", base+"/"+id, nil), http.StatusNoContent)
	expect(t, w.owner.do("GET", base+"/"+id+"/secret", nil), http.StatusNotFound, "webhook_not_found")
}

func TestWebhookTurnsOffAfterRepeatedFailures(t *testing.T) {
	e, _ := newIntegrationsEnv(t)
	w := newTeamWorkspace(e)
	rc := newReceiver(t)
	rc.setStatus(http.StatusBadGateway)
	created := w.owner.do("POST", "/v1/workspace/integrations/webhooks", map[string]any{
		"url": rc.srv.URL, "events": []string{"contact.created"}, "enabled": true,
	})
	expect(t, created, http.StatusCreated)
	wsID := uuid.MustParse(w.id)
	for range 20 {
		err := database.WithTx(context.Background(), e.a.db, func(tx pgx.Tx) error {
			return e.a.integrations.Publish(context.Background(), tx, wsID, "contact.created", map[string]any{})
		})
		if err != nil {
			t.Fatal(err)
		}
		e.runDeliveries(true)
	}
	list := w.owner.do("GET", "/v1/workspace/integrations/webhooks", nil)
	ep := list.body["endpoints"].([]any)[0].(map[string]any)
	if ep["enabled"] != false || ep["disabled_reason"] == nil {
		t.Fatalf("endpoint still on: %v", ep)
	}
	// Turning it back on clears the streak.
	id := ep["id"].(string)
	upd := w.owner.do("PATCH", "/v1/workspace/integrations/webhooks/"+id, map[string]any{
		"url": rc.srv.URL, "events": []string{"contact.created"}, "enabled": true,
	})
	expect(t, upd, http.StatusOK)
	if upd.body["consecutive_failures"].(float64) != 0 || upd.body["disabled_reason"] != nil {
		t.Fatalf("after turning on: %s", upd.raw)
	}
}

func checkSignature(t *testing.T, r *http.Request, body []byte, secret string) {
	t.Helper()
	if r == nil {
		t.Fatal("receiver got nothing")
	}
	sig := r.Header.Get("Relay-Signature")
	ts, _, _ := strings.Cut(strings.TrimPrefix(sig, "t="), ",")
	var unix int64
	if err := json.Unmarshal([]byte(ts), &unix); err != nil {
		t.Fatalf("signature %q", sig)
	}
	if want := integrationsservice.Sign(secret, time.Unix(unix, 0), body); sig != want {
		t.Fatalf("signature %q, want %q", sig, want)
	}
	if r.Header.Get("Relay-Delivery") == "" || r.Header.Get("User-Agent") != "Relay-Webhooks/1.0" {
		t.Fatalf("headers: %v", r.Header)
	}
}

func TestStoreConnections(t *testing.T) {
	e, stores := newIntegrationsEnv(t)
	w := newTeamWorkspace(e)
	base := "/v1/workspace/integrations/connections"

	expect(t, w.owner.do("PUT", base+"/shopify", map[string]any{"shop_domain": "not a shop", "access_token": "good"}),
		http.StatusUnprocessableEntity, "invalid_shop_domain")
	expect(t, w.owner.do("PUT", base+"/shopify", map[string]any{"shop_domain": "greenleaf", "access_token": "bad"}),
		http.StatusUnprocessableEntity, "credentials_rejected")
	shop := w.owner.do("PUT", base+"/shopify", map[string]any{"shop_domain": "https://GreenLeaf.myshopify.com/admin", "access_token": "good"})
	expect(t, shop, http.StatusOK)
	if shop.body["status"] != "connected" || shop.body["account_label"] != "greenleaf.myshopify.com" {
		t.Fatalf("shopify: %s", shop.raw)
	}
	var sealed []byte
	if err := e.a.db.QueryRow(context.Background(),
		`SELECT credentials FROM integration_connections WHERE provider = 'shopify'`).Scan(&sealed); err != nil || strings.Contains(string(sealed), "good") {
		t.Fatalf("credentials stored in the clear (%v)", err)
	}

	woo := w.owner.do("PUT", base+"/woocommerce", map[string]any{
		"store_url": stores.srv.URL + "/", "consumer_key": "ck_good", "consumer_secret": "cs_good",
	})
	expect(t, woo, http.StatusOK)
	expect(t, w.owner.do("PUT", base+"/woocommerce", map[string]any{
		"store_url": stores.srv.URL, "consumer_key": "ck_good", "consumer_secret": "wrong",
	}), http.StatusUnprocessableEntity, "credentials_rejected")
	expect(t, w.owner.do("POST", base+"/woocommerce/test", nil), http.StatusOK)

	expect(t, w.owner.do("DELETE", base+"/shopify", nil), http.StatusNoContent)
	expect(t, w.owner.do("POST", base+"/shopify/test", nil), http.StatusNotFound, "integration_not_connected")
	expect(t, w.owner.do("DELETE", base+"/shopify", nil), http.StatusNotFound)

	o := w.owner.do("GET", "/v1/workspace/integrations", nil)
	expect(t, o, http.StatusOK)
	status := map[string]string{}
	for _, c := range o.body["connections"].([]any) {
		status[c.(map[string]any)["provider"].(string)] = c.(map[string]any)["status"].(string)
	}
	if status["shopify"] != "disconnected" || status["woocommerce"] != "connected" || o.body["google_available"] != true {
		t.Fatalf("overview: %s", o.raw)
	}
}

func TestGoogleSheetsConnect(t *testing.T) {
	e, _ := newIntegrationsEnv(t)
	w := newTeamWorkspace(e)
	auth := w.owner.do("POST", "/v1/workspace/integrations/connections/google_sheets/authorize", nil)
	expect(t, auth, http.StatusOK)
	u, _ := url.Parse(auth.body["url"].(string))
	q := u.Query()
	if q.Get("access_type") != "offline" || !strings.Contains(q.Get("scope"), "spreadsheets") || q.Get("client_id") != "google-client" {
		t.Fatalf("consent url: %s", u)
	}
	state := q.Get("state")

	// Another browser can't finish someone else's attempt.
	stranger := e.browser(chromeMac)
	r := stranger.do("GET", "/v1/integrations/google/callback?code=good-code&state="+url.QueryEscape(state), nil)
	expect(t, r, http.StatusFound)
	if !strings.Contains(r.header.Get("Location"), "integration_error=invalid_oauth_state") {
		t.Fatalf("stranger: %s", r.header.Get("Location"))
	}

	r = w.owner.do("GET", "/v1/integrations/google/callback?code=good-code&state="+url.QueryEscape(state), nil)
	expect(t, r, http.StatusFound)
	if r.header.Get("Location") != testOrigin+"/integrations?connected=google_sheets" {
		t.Fatalf("callback: %s", r.header.Get("Location"))
	}
	test := w.owner.do("POST", "/v1/workspace/integrations/connections/google_sheets/test", nil)
	expect(t, test, http.StatusOK)
	if test.body["account_label"] != "sheets@greenleaf.test" || test.body["status"] != "connected" {
		t.Fatalf("google: %s", test.raw)
	}

	// A tampered state is refused.
	r = w.owner.do("GET", "/v1/integrations/google/callback?code=good-code&state=x."+url.QueryEscape(state), nil)
	if !strings.Contains(r.header.Get("Location"), "integration_error=") {
		t.Fatalf("tampered: %s", r.header.Get("Location"))
	}
}

// shopifySign signs a callback query the way Shopify does: sorted k=v pairs, HMAC-SHA256 hex.
func shopifySign(q url.Values, secret string) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + q.Get(k)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.Join(parts, "&")))
	q.Set("hmac", hex.EncodeToString(mac.Sum(nil)))
	return q.Encode()
}

func TestShopifyOAuthConnect(t *testing.T) {
	e, _ := newIntegrationsEnv(t)
	w := newTeamWorkspace(e)
	o := w.owner.do("GET", "/v1/workspace/integrations", nil)
	if o.body["shopify_oauth"] != true || o.body["woocommerce_oauth"] != false {
		t.Fatalf("flags: %s", o.raw)
	}
	expect(t, w.owner.do("POST", "/v1/workspace/integrations/connections/shopify/authorize",
		map[string]any{"shop_domain": "not a shop"}), http.StatusUnprocessableEntity, "invalid_shop_domain")
	// start begins an attempt; each callback ends it, so every case starts again.
	start := func() string {
		auth := w.owner.do("POST", "/v1/workspace/integrations/connections/shopify/authorize", map[string]any{"shop_domain": "greenleaf"})
		expect(t, auth, http.StatusOK)
		u, _ := url.Parse(auth.body["url"].(string))
		q := u.Query()
		if u.Host != "greenleaf.myshopify.com" || u.Path != "/admin/oauth/authorize" || q.Get("client_id") != "shopify-client" ||
			q.Get("scope") != "read_customers,read_orders" || !strings.HasSuffix(q.Get("redirect_uri"), "/v1/integrations/shopify/callback") {
			t.Fatalf("install url: %s", u)
		}
		return q.Get("state")
	}
	callback := func(b *browser, params url.Values) string {
		r := b.do("GET", "/v1/integrations/shopify/callback?"+params.Encode(), nil)
		expect(t, r, http.StatusFound)
		return r.header.Get("Location")
	}
	signed := func(shop, code string) url.Values {
		v := url.Values{"code": {code}, "shop": {shop}, "state": {start()}, "timestamp": {"1700000000"}, "host": {"YWRtaW4"}}
		out, _ := url.ParseQuery(shopifySign(v, "shopify-secret"))
		return out
	}
	refused := func(name, loc, code string) {
		t.Helper()
		if !strings.Contains(loc, "integration_error="+code) {
			t.Fatalf("%s: %s", name, loc)
		}
	}

	unsigned := url.Values{"code": {"good-code"}, "shop": {"greenleaf.myshopify.com"}, "state": {start()}}
	refused("unsigned", callback(w.owner, unsigned), "invalid_oauth_state")
	forged := signed("greenleaf.myshopify.com", "good-code")
	forged.Set("hmac", strings.Repeat("0", 64))
	refused("wrong hmac", callback(w.owner, forged), "invalid_oauth_state")
	refused("other shop", callback(w.owner, signed("evil.myshopify.com", "good-code")), "invalid_oauth_state")
	refused("stranger", callback(e.browser(chromeMac), signed("greenleaf.myshopify.com", "good-code")), "invalid_oauth_state")
	refused("bad code", callback(w.owner, signed("greenleaf.myshopify.com", "bad-code")), "shopify_exchange_failed")
	if loc := callback(w.owner, signed("greenleaf.myshopify.com", "good-code")); loc != testOrigin+"/integrations?connected=shopify" {
		t.Fatalf("callback: %s", loc)
	}
	test := w.owner.do("POST", "/v1/workspace/integrations/connections/shopify/test", nil)
	expect(t, test, http.StatusOK)
	if test.body["status"] != "connected" || test.body["account_label"] != "greenleaf.myshopify.com" {
		t.Fatalf("shopify: %s", test.raw)
	}
}

func TestWooCommerceOAuthConnect(t *testing.T) {
	e, stores := newIntegrationsEnv(t)
	w := newTeamWorkspace(e)
	base := "/v1/workspace/integrations/connections/woocommerce/authorize"
	// On http, WooCommerce won't post keys back, so the one-click flow is off.
	expect(t, w.owner.do("POST", base, map[string]any{"store_url": stores.srv.URL}), http.StatusNotFound, "woocommerce_oauth_unavailable")

	e.a.cfg.Auth.APIBaseURL = "https://api.relay.test"
	rebuildIntegrations(t, e)
	auth := w.owner.do("POST", base, map[string]any{"store_url": stores.srv.URL + "/"})
	expect(t, auth, http.StatusOK)
	u, _ := url.Parse(auth.body["url"].(string))
	q := u.Query()
	if !strings.HasPrefix(u.String(), stores.srv.URL+"/wc-auth/v1/authorize?") || q.Get("scope") != "read" ||
		q.Get("callback_url") != "https://api.relay.test/v1/integrations/woocommerce/callback" ||
		q.Get("return_url") != testOrigin+"/integrations?provider=woocommerce" {
		t.Fatalf("authorize url: %s", u)
	}
	state := q.Get("user_id")
	post := func(body map[string]any) resp {
		return e.browser(chromeMac).do("POST", "/v1/integrations/woocommerce/callback", body)
	}
	keys := func(st, secret string) map[string]any {
		return map[string]any{"key_id": 7, "user_id": st, "consumer_key": "ck_good", "consumer_secret": secret, "key_permissions": "read"}
	}
	expect(t, post(keys("forged."+state, "cs_good")), http.StatusUnprocessableEntity, "invalid_oauth_state")
	expect(t, post(keys(state, "wrong")), http.StatusUnprocessableEntity, "credentials_rejected")
	expect(t, post(keys(state, "cs_good")), http.StatusOK)

	o := w.owner.do("GET", "/v1/workspace/integrations", nil)
	if o.body["woocommerce_oauth"] != true {
		t.Fatalf("flag: %s", o.raw)
	}
	test := w.owner.do("POST", "/v1/workspace/integrations/connections/woocommerce/test", nil)
	expect(t, test, http.StatusOK)
	if test.body["status"] != "connected" {
		t.Fatalf("woocommerce: %s", test.raw)
	}
}
