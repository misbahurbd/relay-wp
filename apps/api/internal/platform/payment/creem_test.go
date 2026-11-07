package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

var _ Gateway = (*creemGateway)(nil)

const (
	creemTestKey    = "creem_test_key"
	creemTestSecret = "whsec_creem_test" //nolint:gosec // not a credential: signs test fixtures
	// creemFixtureRef is the reference every fixture carries.
	creemFixtureRef = "0192f7a4-7c1e-7b3a-9d2e-5f8a1c3b4d6e"
)

// creemCall is one request the fake Creem API received.
type creemCall struct {
	Method, Path, Query, APIKey string
	Body                        map[string]any
}

// fakeCreemAPI is an httptest server that answers like Creem's API.
type fakeCreemAPI struct {
	t *testing.T

	mu     sync.Mutex
	calls  []creemCall
	routes map[string]func(w http.ResponseWriter, c creemCall)
}

func newCreemWithAPI(t *testing.T) (*creemGateway, *fakeCreemAPI) {
	t.Helper()
	api := &fakeCreemAPI{t: t, routes: map[string]func(http.ResponseWriter, creemCall){}}
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	g, err := newCreem(config.Payment{CreemAPIKey: creemTestKey, CreemWebhookSecret: creemTestSecret}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	g.baseURL = srv.URL
	return g, api
}

// handle answers "METHOD /path" with status and the JSON body.
func (a *fakeCreemAPI) handle(route string, status int, body string) {
	a.routes[route] = func(w http.ResponseWriter, _ creemCall) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func (a *fakeCreemAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := creemCall{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, APIKey: r.Header.Get("x-api-key")}
	if b, _ := io.ReadAll(r.Body); len(b) > 0 {
		if err := json.Unmarshal(b, &c.Body); err != nil {
			a.t.Errorf("%s %s: body is not JSON: %v", r.Method, r.URL.Path, err)
		}
	}
	a.mu.Lock()
	a.calls = append(a.calls, c)
	h, ok := a.routes[r.Method+" "+r.URL.Path]
	a.mu.Unlock()
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"trace_id":"t404","status":404,"error":"Not Found"}`)
		return
	}
	h(w, c)
}

func (a *fakeCreemAPI) last() creemCall {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.calls) == 0 {
		a.t.Fatal("no call reached the fake Creem API")
	}
	return a.calls[len(a.calls)-1]
}

func (a *fakeCreemAPI) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.calls)
}

func TestNewCreemPicksHostFromKey(t *testing.T) {
	for key, want := range map[string]string{"creem_test_abc": creemTestBaseURL, "creem_abc": creemLiveBaseURL} {
		g, err := newCreem(config.Payment{CreemAPIKey: key, CreemWebhookSecret: "s"}, http.DefaultClient)
		if err != nil {
			t.Fatal(err)
		}
		if g.baseURL != want {
			t.Errorf("key %q: base URL %q, want %q", key, g.baseURL, want)
		}
	}
	if _, err := newCreem(config.Payment{CreemAPIKey: "creem_abc"}, http.DefaultClient); err == nil {
		t.Error("a missing webhook secret must fail at startup")
	}
}

func TestCreemCapabilities(t *testing.T) {
	g, _ := newCreemWithAPI(t)
	c := g.Capabilities()
	if c.EmbeddedCheckout || c.Installments || c.PartialRefunds || c.CheckoutTrials || c.PublishableKey != "" {
		t.Errorf("Creem has no client secret, installments, partial refunds or per-checkout trials: %+v", c)
	}
	if !c.MerchantOfRecord || !c.RenewsSubscriptions || !c.Coupons {
		t.Errorf("Creem is the merchant of record, renews subscriptions and takes discount codes: %+v", c)
	}
	if !c.CouponsPerProduct {
		t.Errorf("a Creem discount covers only the products it names: %+v", c)
	}
	if !c.PaymentNeedsProduct || c.MinChargeMinor != 100 {
		t.Errorf("a Creem one-off payment charges a product, at least 100 cents: %+v", c)
	}
	if g.Provider() != Creem {
		t.Errorf("provider = %q", g.Provider())
	}
}

const creemCheckoutResponse = `{"id":"ch_1","object":"checkout","mode":"test","status":"pending",
	"product":"prod_growth","customer":"cust_9","checkout_url":"https://checkout.creem.io/ch_1"}`

func TestCreemCreateSubscriptionCheckout(t *testing.T) {
	g, api := newCreemWithAPI(t)
	api.handle("POST /v1/checkouts", http.StatusOK, creemCheckoutResponse)
	ref, ws := uuid.New(), uuid.New()

	c, err := g.CreateCheckout(context.Background(), CheckoutRequest{
		Reference: ref, Mode: ModeSubscription, ExternalPriceID: "prod_growth", AmountMinor: 2900, Currency: "USD",
		ExternalCouponID: "LAUNCH10", ReturnURL: "https://app.example.com/billing",
		Customer: Customer{WorkspaceID: ws, Email: "owner@example.com", Name: "Owner"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.ExternalSessionID != "ch_1" || c.URL != "https://checkout.creem.io/ch_1" || c.ClientSecret != "" ||
		c.ExternalCustomerID != "cust_9" {
		t.Errorf("checkout = %+v", c)
	}

	call := api.last()
	if call.APIKey != creemTestKey {
		t.Errorf("x-api-key = %q", call.APIKey)
	}
	b := call.Body
	if b["product_id"] != "prod_growth" || b["request_id"] != ref.String() || b["discount_code"] != "LAUNCH10" ||
		b["success_url"] != "https://app.example.com/billing" {
		t.Errorf("body = %v", b)
	}
	if _, ok := b["custom_price"]; ok {
		t.Error("a subscription checkout must not override the product's price")
	}
	md, _ := b["metadata"].(map[string]any)
	if md[creemReferenceKey] != ref.String() || md["workspace_id"] != ws.String() {
		t.Errorf("metadata = %v", md)
	}
	cust, _ := b["customer"].(map[string]any)
	if cust["email"] != "owner@example.com" || cust["id"] != nil {
		t.Errorf("customer = %v", cust)
	}
}

func TestCreemCreatePaymentCheckoutChargesTheReducedAmount(t *testing.T) {
	g, api := newCreemWithAPI(t)
	api.handle("POST /v1/checkouts", http.StatusOK, creemCheckoutResponse)

	_, err := g.CreateCheckout(context.Background(), CheckoutRequest{
		Reference: uuid.New(), Mode: ModePayment, ExternalPriceID: "prod_ltd", AmountMinor: 14900, Currency: "USD",
		ExternalCouponID: "IGNORED", Customer: Customer{ExternalID: "cust_9", Email: "owner@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	b := api.last().Body
	if b["custom_price"] != float64(14900) {
		t.Errorf("custom_price = %v, want 14900", b["custom_price"])
	}
	if _, ok := b["discount_code"]; ok {
		t.Error("Relay already took the discount off a one-time payment")
	}
	cust, _ := b["customer"].(map[string]any)
	if cust["id"] != "cust_9" || cust["email"] != nil {
		t.Errorf("customer = %v: send the id or the email, never both", cust)
	}
}

func TestCreemCheckoutRejects(t *testing.T) {
	g, api := newCreemWithAPI(t)
	base := CheckoutRequest{Reference: uuid.New(), ExternalPriceID: "prod_x", AmountMinor: 1000, Currency: "USD"}
	cases := []struct {
		name string
		edit func(*CheckoutRequest)
		want error
	}{
		{"installments", func(r *CheckoutRequest) { r.Mode = ModeInstallments; r.Schedule = []int64{500, 500} }, ErrUnsupported},
		{"no reference", func(r *CheckoutRequest) { r.Mode = ModePayment; r.Reference = uuid.Nil }, ErrInvalidRequest},
		{"no product", func(r *CheckoutRequest) { r.Mode = ModeSubscription; r.ExternalPriceID = "" }, ErrInvalidRequest},
		{"under Creem's minimum", func(r *CheckoutRequest) { r.Mode = ModePayment; r.AmountMinor = 99 }, ErrInvalidRequest},
		{"trial per checkout", func(r *CheckoutRequest) { r.Mode = ModeSubscription; r.TrialDays = 14 }, ErrUnsupported},
		{"no mode", func(*CheckoutRequest) {}, ErrInvalidRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			tc.edit(&req)
			if _, err := g.CreateCheckout(context.Background(), req); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
	if n := api.count(); n != 0 {
		t.Errorf("%d calls reached Creem; a rejected checkout must not", n)
	}
}

func TestCreemErrorsMapToGatewayErrors(t *testing.T) {
	cases := map[int]error{
		http.StatusBadRequest: ErrInvalidRequest, http.StatusForbidden: ErrInvalidRequest,
		http.StatusNotFound: ErrNotFound, http.StatusTooManyRequests: ErrUnavailable,
		http.StatusBadGateway: ErrUnavailable,
	}
	for status, want := range cases {
		g, api := newCreemWithAPI(t)
		api.handle("POST /v1/checkouts", status,
			`{"trace_id":"tr_1","status":400,"error":"Bad Request","message":["customer.email owner@example.com is invalid"]}`)
		_, err := g.CreateCheckout(context.Background(), CheckoutRequest{
			Reference: uuid.New(), Mode: ModeSubscription, ExternalPriceID: "prod_x", Currency: "USD",
		})
		if !errors.Is(err, want) {
			t.Errorf("status %d: err = %v, want %v", status, err, want)
			continue
		}
		if strings.Contains(err.Error(), "owner@example.com") {
			t.Errorf("status %d: the error repeats Creem's message, which may hold personal data: %v", status, err)
		}
	}
}

const creemSubscriptionResponse = `{"id":"sub_1","object":"subscription","mode":"test",
	"product":{"id":"prod_pro","name":"Pro"},"customer":"cust_9","collection_method":"charge_automatically",
	"status":"%s","current_period_start_date":"2024-11-12T11:58:38.000Z",
	"current_period_end_date":"2024-12-12T11:58:38.000Z","canceled_at":%s}`

func subscriptionResponse(status, canceledAt string) string {
	return fmt.Sprintf(creemSubscriptionResponse, status, canceledAt)
}

func TestCreemChangeSubscription(t *testing.T) {
	for prorate, behavior := range map[bool]string{true: "proration-charge-immediately", false: "proration-none"} {
		g, api := newCreemWithAPI(t)
		api.handle("POST /v1/subscriptions/sub_1/upgrade", http.StatusOK, subscriptionResponse("active", "null"))
		s, err := g.ChangeSubscription(context.Background(), ChangeRequest{
			ExternalSubscriptionID: "sub_1", ExternalPriceID: "prod_pro", ProrateNow: prorate,
		})
		if err != nil {
			t.Fatal(err)
		}
		if b := api.last().Body; b["product_id"] != "prod_pro" || b["update_behavior"] != behavior {
			t.Errorf("prorate %v: body = %v", prorate, b)
		}
		if s.ExternalID != "sub_1" || s.ExternalPriceID != "prod_pro" || s.ExternalCustomerID != "cust_9" ||
			s.Status != SubscriptionActive || s.CurrentPeriodEnd == nil {
			t.Errorf("subscription = %+v", s)
		}
	}
}

func TestCreemCancelSubscription(t *testing.T) {
	cases := []struct {
		mode       CancelMode
		path       string
		body       map[string]any
		status     string
		canceledAt string
		want       SubscriptionStatus
		atEnd      bool
	}{
		{
			CancelNow, "/v1/subscriptions/sub_1/cancel",
			map[string]any{"mode": "immediate"},
			"canceled", `"2024-11-20T10:00:00.000Z"`, SubscriptionCanceled, false,
		},
		{
			CancelAtPeriodEnd, "/v1/subscriptions/sub_1/cancel",
			map[string]any{"mode": "scheduled", "onExecute": "cancel"},
			"scheduled_cancel", "null", SubscriptionActive, true,
		},
		{CancelUndo, "/v1/subscriptions/sub_1/resume", nil, "active", "null", SubscriptionActive, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.mode), func(t *testing.T) {
			g, api := newCreemWithAPI(t)
			api.handle("POST "+tc.path, http.StatusOK, subscriptionResponse(tc.status, tc.canceledAt))
			api.handle("GET /v1/subscriptions", http.StatusOK, subscriptionResponse("active", "null"))
			s, err := g.CancelSubscription(context.Background(), CancelRequest{ExternalSubscriptionID: "sub_1", Mode: tc.mode})
			if err != nil {
				t.Fatal(err)
			}
			call := api.last()
			if call.Path != tc.path || len(call.Body) != len(tc.body) {
				t.Fatalf("call = %+v", call)
			}
			for k, v := range tc.body {
				if call.Body[k] != v {
					t.Errorf("body[%s] = %v, want %v", k, call.Body[k], v)
				}
			}
			if s.Status != tc.want || s.CancelAtPeriodEnd != tc.atEnd || s.ProviderStatus != tc.status {
				t.Errorf("subscription = %+v", s)
			}
			if (s.CanceledAt != nil) != (tc.mode == CancelNow) {
				t.Errorf("canceled_at = %v", s.CanceledAt)
			}
		})
	}

	g, _ := newCreemWithAPI(t)
	if _, err := g.CancelSubscription(context.Background(), CancelRequest{ExternalSubscriptionID: "sub_gone", Mode: CancelNow}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown subscription: err = %v, want ErrNotFound", err)
	}
}

func TestCreemRefund(t *testing.T) {
	g, api := newCreemWithAPI(t)
	api.handle("POST /v1/refunds", http.StatusOK, `{"status":"pending"}`)
	r, err := g.Refund(context.Background(), RefundRequest{ExternalPaymentID: "tran_1", AmountMinor: 3158, Currency: "usd"})
	if err != nil {
		t.Fatal(err)
	}
	if b := api.last().Body; b["transaction_id"] != "tran_1" || len(b) != 1 {
		t.Errorf("body = %v: Creem refunds a transaction in full and takes no amount", b)
	}
	if r.ExternalPaymentID != "tran_1" || r.AmountMinor != 3158 || r.Currency != "USD" || r.Status != RefundPending {
		t.Errorf("refund = %+v", r)
	}
	if _, err := g.Refund(context.Background(), RefundRequest{AmountMinor: 1}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("no payment: err = %v", err)
	}
}

func TestCreemCreateCoupon(t *testing.T) {
	g, api := newCreemWithAPI(t)
	page := 0
	api.routes["GET /v1/products/search"] = func(w http.ResponseWriter, c creemCall) {
		page++
		if !strings.Contains(c.Query, "status=active") {
			t.Errorf("query = %q", c.Query)
		}
		if page == 1 {
			_, _ = io.WriteString(w, `{"items":[{"id":"prod_growth","billing_type":"recurring","status":"active"},
				{"id":"prod_ltd","billing_type":"onetime","status":"active"}],"pagination":{"next_page":2}}`)
			return
		}
		_, _ = io.WriteString(w, `{"items":[{"id":"prod_pro","billing_type":"recurring","status":"active"}],
			"pagination":{"next_page":null}}`)
	}
	api.handle("POST /v1/discounts", http.StatusOK,
		`{"id":"dis_1","object":"discount","mode":"test","status":"active","name":"Launch","code":"LAUNCH10","type":"percentage"}`)

	c, err := g.CreateCoupon(context.Background(), CouponRequest{
		DiscountID: uuid.New(), Code: "LAUNCH10", Name: "Launch", PercentBps: 1250, Duration: "repeating", DurationMonths: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.ExternalCouponID != "LAUNCH10" || c.ExternalPromotionCodeID != "dis_1" {
		t.Errorf("coupon = %+v", c)
	}
	b := api.last().Body
	if b["type"] != "percentage" || b["percentage"] != 12.5 || b["code"] != "LAUNCH10" ||
		b["duration"] != "repeating" || b["duration_in_months"] != float64(3) {
		t.Errorf("body = %v", b)
	}
	products, _ := b["applies_to_products"].([]any)
	if len(products) != 2 || products[0] != "prod_growth" || products[1] != "prod_pro" {
		t.Errorf("applies_to_products = %v: recurring products only, from every page", products)
	}
}

// A Creem code can be typed into Creem's own checkout, so it carries Relay's
// expiry, uses left and products.
func TestCreemCouponCarriesRelaysLimits(t *testing.T) {
	g, api := newCreemWithAPI(t)
	api.handle("POST /v1/discounts", http.StatusOK, `{"id":"dis_3","code":"SPRING"}`)
	expires := time.Date(2027, 3, 31, 23, 59, 59, 0, time.FixedZone("BDT", 6*3600))
	if _, err := g.CreateCoupon(context.Background(), CouponRequest{
		Code: "SPRING", PercentBps: 1000, Duration: "once", ExternalProductIDs: []string{"prod_growth"},
		ExpiresAt: &expires, MaxRedemptions: 40,
	}); err != nil {
		t.Fatal(err)
	}
	b := api.last().Body
	if b["expiry_date"] != "2027-03-31T17:59:59Z" || b["max_redemptions"] != float64(40) {
		t.Errorf("body = %v", b)
	}
	products, _ := b["applies_to_products"].([]any)
	if len(products) != 1 || products[0] != "prod_growth" {
		t.Errorf("applies_to_products = %v", products)
	}
}

func TestCreemDeleteCoupon(t *testing.T) {
	g, api := newCreemWithAPI(t)
	api.handle("DELETE /v1/discounts/dis_3/delete", http.StatusOK, `{"id":"dis_3"}`)
	if err := g.DeleteCoupon(context.Background(), Coupon{ExternalCouponID: "SPRING", ExternalPromotionCodeID: "dis_3"}); err != nil {
		t.Fatal(err)
	}
	if c := api.last(); c.Method != http.MethodDelete || c.Path != "/v1/discounts/dis_3/delete" {
		t.Errorf("call = %s %s", c.Method, c.Path)
	}
	if err := g.DeleteCoupon(context.Background(), Coupon{ExternalPromotionCodeID: "dis_gone"}); err != nil {
		t.Errorf("a discount already gone: %v", err)
	}
	api.handle("DELETE /v1/discounts/dis_4/delete", http.StatusServiceUnavailable, `{}`)
	if err := g.DeleteCoupon(context.Background(), Coupon{ExternalPromotionCodeID: "dis_4"}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("a failed delete: %v", err)
	}
}

func TestCreemCreateFixedCoupon(t *testing.T) {
	g, api := newCreemWithAPI(t)
	api.handle("GET /v1/products/search", http.StatusOK,
		`{"items":[{"id":"prod_growth","billing_type":"recurring","status":"active"}],"pagination":{"next_page":null}}`)
	api.handle("POST /v1/discounts", http.StatusOK, `{"id":"dis_2","code":"TENOFF"}`)
	if _, err := g.CreateCoupon(context.Background(), CouponRequest{
		Code: "TENOFF", AmountOffMinor: 1000, Currency: "usd", Duration: "once",
	}); err != nil {
		t.Fatal(err)
	}
	b := api.last().Body
	if b["type"] != "fixed" || b["amount"] != float64(1000) || b["currency"] != "USD" || b["name"] != "TENOFF" {
		t.Errorf("body = %v", b)
	}
	for name, req := range map[string]CouponRequest{
		"both":                {PercentBps: 100, AmountOffMinor: 1, Currency: "USD", Duration: "once"},
		"neither":             {Duration: "once"},
		"no currency":         {AmountOffMinor: 1, Duration: "once"},
		"repeating no months": {PercentBps: 100, Duration: "repeating"},
		"bad duration":        {PercentBps: 100, Duration: "weekly"},
	} {
		if _, err := g.CreateCoupon(context.Background(), req); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%s: err = %v, want ErrInvalidRequest", name, err)
		}
	}
}

func TestCreemCreatePrice(t *testing.T) {
	g, api := newCreemWithAPI(t)
	api.handle("POST /v1/products", http.StatusOK, `{"id":"prod_growth_m","object":"product","tax_mode":"exclusive"}`)
	id, err := g.CreatePrice(context.Background(), PriceRequest{
		PlanPriceID: uuid.New(), Name: "Relay Growth (monthly)", AmountMinor: 2900, Currency: "usd", Interval: "month",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "prod_growth_m" {
		t.Fatalf("product id = %q", id)
	}
	b := api.last().Body
	want := map[string]any{
		"name": "Relay Growth (monthly)", "description": "Relay Growth (monthly)", "price": float64(2900), "currency": "USD",
		"billing_type": "recurring", "billing_period": "every-month", "tax_mode": "exclusive", "tax_category": "saas",
	}
	for k, v := range want {
		if b[k] != v {
			t.Errorf("body[%s] = %v, want %v", k, b[k], v)
		}
	}
	// A product trial would trial every checkout of it, a workspace's second one included.
	if _, ok := b["trial_period_days"]; ok {
		t.Errorf("a Creem product never carries a trial: body = %v", b)
	}

	api.handle("POST /v1/products", http.StatusOK, `{"id":"prod_growth_y"}`)
	if _, err := g.CreatePrice(context.Background(), PriceRequest{
		Name: "Relay Growth (yearly)", AmountMinor: 29000, Currency: "USD", Interval: "year",
	}); err != nil {
		t.Fatal(err)
	}
	b = api.last().Body
	if b["billing_period"] != "every-year" || b["billing_type"] != "recurring" {
		t.Errorf("yearly body = %v", b)
	}

	api.handle("POST /v1/products", http.StatusOK, `{"id":"prod_ltd"}`)
	if _, err := g.CreatePrice(context.Background(), PriceRequest{
		Name: "Relay Growth (lifetime)", AmountMinor: 29900, Currency: "USD",
	}); err != nil {
		t.Fatal(err)
	}
	b = api.last().Body
	if b["billing_type"] != "onetime" || b["billing_period"] != "once" {
		t.Errorf("one-time body = %v", b)
	}

	calls := api.count()
	for name, req := range map[string]PriceRequest{
		"no name":      {AmountMinor: 100, Currency: "USD"},
		"no amount":    {Name: "x", Currency: "USD"},
		"no currency":  {Name: "x", AmountMinor: 100},
		"bad interval": {Name: "x", AmountMinor: 100, Currency: "USD", Interval: "week"},
	} {
		if _, err := g.CreatePrice(context.Background(), req); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%s: err = %v, want ErrInvalidRequest", name, err)
		}
	}
	if api.count() != calls {
		t.Error("an invalid request reached Creem")
	}

	api.handle("POST /v1/products", http.StatusBadGateway, `{}`)
	if _, err := g.CreatePrice(context.Background(), PriceRequest{
		Name: "x", AmountMinor: 100, Currency: "USD",
	}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("5xx: err = %v, want ErrUnavailable", err)
	}
}

// signCreem signs body as Creem does: hex HMAC-SHA256 of the raw body.
func signCreem(body []byte) http.Header {
	h := hmac.New(sha256.New, []byte(creemTestSecret))
	h.Write(body)
	header := http.Header{}
	header.Set(creemSignatureHeader, hex.EncodeToString(h.Sum(nil)))
	return header
}

func creemFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "creem_"+name+".json")) //nolint:gosec // fixed test fixtures
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func parseCreemFixture(t *testing.T, name string) Event {
	t.Helper()
	g, _ := newCreemWithAPI(t)
	body := creemFixture(t, name)
	ev, err := g.ParseWebhook(context.Background(), signCreem(body), body)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Provider != Creem || ev.ExternalID == "" || ev.ProviderType == "" || ev.OccurredAt.IsZero() {
		t.Errorf("envelope = %+v", ev)
	}
	if !bytes.Equal(ev.Payload, body) {
		t.Error("Payload must be the raw verified body")
	}
	return ev
}

// parseCreemBody signs and parses an inline Creem webhook body.
func parseCreemBody(t *testing.T, body []byte) Event {
	t.Helper()
	g, _ := newCreemWithAPI(t)
	ev, err := g.ParseWebhook(context.Background(), signCreem(body), body)
	if err != nil {
		t.Fatal(err)
	}
	return ev
}

func checkReference(t *testing.T, ev Event) {
	t.Helper()
	if ev.Reference == nil || ev.Reference.String() != creemFixtureRef {
		t.Errorf("reference = %v, want %s", ev.Reference, creemFixtureRef)
	}
}

func checkTotals(t *testing.T, p *Payment, subtotal, discount, tax, total int64) {
	t.Helper()
	if p == nil {
		t.Fatal("no payment")
	}
	if p.SubtotalMinor != subtotal || p.DiscountMinor != discount || p.TaxMinor != tax || p.TotalMinor != total {
		t.Errorf("totals = %d - %d + %d = %d, want %d - %d + %d = %d",
			p.SubtotalMinor, p.DiscountMinor, p.TaxMinor, p.TotalMinor, subtotal, discount, tax, total)
	}
	if p.TotalMinor != p.SubtotalMinor-p.DiscountMinor+p.TaxMinor {
		t.Error("TotalMinor must equal SubtotalMinor - DiscountMinor + TaxMinor")
	}
}

func TestCreemWebhookCheckoutCompleted(t *testing.T) {
	ev := parseCreemFixture(t, "checkout_completed")
	if ev.Type != EventCheckoutCompleted || ev.ExternalID != "evt_5WHHcZPv7VS0YUsberIuOz" {
		t.Errorf("event = %+v", ev)
	}
	if want := time.UnixMilli(1728734327412).UTC(); !ev.OccurredAt.Equal(want) {
		t.Errorf("occurred at %v, want %v", ev.OccurredAt, want)
	}
	checkReference(t, ev)
	p := ev.Payment
	checkTotals(t, p, 2900, 290, 548, 3158)
	if p.ExternalID != "tran_5yMaWzAl3jxuGJMCOrYWwk" || p.ExternalCustomerID != "cust_1OcIK1GEuVvXZwD19tjq2z" ||
		p.ExternalSubscriptionID != "sub_6pC2lNoI8ixy1kAoEqdhWl" || p.Currency != "USD" || p.PaidAt == nil ||
		p.PeriodEnd == nil {
		t.Errorf("payment = %+v", p)
	}
	if s := ev.Subscription; s == nil || s.Status != SubscriptionActive || s.ExternalPriceID != "prod_d1AY2Sadk9YAvLI0pj97f" {
		t.Errorf("subscription = %+v", s)
	}
}

func TestCreemWebhookSubscriptionPaidWithInclusiveTax(t *testing.T) {
	ev := parseCreemFixture(t, "subscription_paid")
	if ev.Type != EventPaymentSucceeded {
		t.Errorf("type = %q", ev.Type)
	}
	checkReference(t, ev)
	p := ev.Payment
	checkTotals(t, p, 2437, 0, 463, 2900)
	if p.ExternalID != "tran_3e6Z6TzvHKdsjEgXnGDEp0" || p.ExternalCustomerID != "cust_1OcIK1GEuVvXZwD19tjq2z" ||
		p.ExternalSubscriptionID != "sub_6pC2lNoI8ixy1kAoEqdhWl" || p.PeriodStart == nil || p.PeriodEnd == nil {
		t.Errorf("payment = %+v", p)
	}
	if want := time.UnixMilli(1734004718000).UTC(); p.PeriodEnd == nil || !p.PeriodEnd.Equal(want) {
		t.Errorf("period end = %v, want %v", p.PeriodEnd, want)
	}
	if s := ev.Subscription; s == nil || s.ExternalPriceID != "prod_d1AY2Sadk9YAvLI0pj97f" || s.Status != SubscriptionActive {
		t.Errorf("subscription = %+v", s)
	}
}

// editedCreemFixture is a fixture with edit applied to its object.
func editedCreemFixture(t *testing.T, name string, edit func(obj map[string]any)) []byte {
	t.Helper()
	var env map[string]any
	if err := json.Unmarshal(creemFixture(t, name), &env); err != nil {
		t.Fatal(err)
	}
	edit(env["object"].(map[string]any))
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The first payment arrives as checkout.completed and as subscription.paid.
// Both must name the same transaction, so its invoice is recorded once.
func TestCreemCheckoutCompletedNamesTheTransactionSubscriptionPaidUses(t *testing.T) {
	cases := []struct {
		name string
		edit func(order, sub map[string]any)
		want string
	}{
		{"order transaction", func(_, _ map[string]any) {}, "tran_5yMaWzAl3jxuGJMCOrYWwk"},
		{"subscription last_transaction_id", func(order, sub map[string]any) {
			delete(order, "transaction")
			sub["last_transaction_id"] = "tran_first"
		}, "tran_first"},
		{"subscription last_transaction object", func(order, sub map[string]any) {
			delete(order, "transaction")
			sub["last_transaction"] = map[string]any{"id": "tran_first", "amount_paid": 3158, "currency": "USD", "status": "paid"}
		}, "tran_first"},
		{"no transaction anywhere", func(order, _ map[string]any) { delete(order, "transaction") }, "ord_4aDwWXjMLpes4Kj4XqNnUA"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := parseCreemBody(t, editedCreemFixture(t, "checkout_completed", func(obj map[string]any) {
				tc.edit(obj["order"].(map[string]any), obj["subscription"].(map[string]any))
			}))
			if ev.Payment == nil || ev.Payment.ExternalID != tc.want {
				t.Fatalf("payment = %+v, want external id %s", ev.Payment, tc.want)
			}
		})
	}
}

// A subscription.paid whose last_transaction isn't expanded still parses; it
// records no payment, since only the expanded transaction says what was paid.
func TestCreemSubscriptionPaidWithoutAnExpandedTransaction(t *testing.T) {
	for name, edit := range map[string]func(map[string]any){
		"id string": func(obj map[string]any) { obj["last_transaction"] = "tran_3e6Z6TzvHKdsjEgXnGDEp0" },
		"id only":   func(obj map[string]any) { delete(obj, "last_transaction") },
	} {
		t.Run(name, func(t *testing.T) {
			ev := parseCreemBody(t, editedCreemFixture(t, "subscription_paid", edit))
			if ev.Type != EventPaymentSucceeded || ev.Payment != nil {
				t.Errorf("event = %+v, payment = %+v; want payment.succeeded with no payment", ev, ev.Payment)
			}
			if ev.Subscription == nil || ev.Subscription.ExternalID != "sub_6pC2lNoI8ixy1kAoEqdhWl" {
				t.Errorf("subscription = %+v", ev.Subscription)
			}
		})
	}
}

func TestCreemWebhookSubscriptionChanges(t *testing.T) {
	ev := parseCreemFixture(t, "subscription_scheduled_cancel")
	if ev.Type != EventSubscriptionUpdated || ev.Subscription == nil ||
		ev.Subscription.Status != SubscriptionActive || !ev.Subscription.CancelAtPeriodEnd {
		t.Errorf("scheduled cancel = %+v, %+v", ev, ev.Subscription)
	}
	checkReference(t, ev)

	ev = parseCreemFixture(t, "subscription_canceled")
	if ev.Type != EventSubscriptionCanceled || ev.Subscription == nil ||
		ev.Subscription.Status != SubscriptionCanceled || ev.Subscription.CanceledAt == nil {
		t.Errorf("canceled = %+v, %+v", ev, ev.Subscription)
	}
	checkReference(t, ev)
}

// expired means a renewal is unpaid while Creem retries it: past due, with
// Relay's grace window, never the end of the subscription.
func TestCreemWebhookSubscriptionExpiredIsPastDue(t *testing.T) {
	ev := parseCreemBody(t, []byte(`{"id":"evt_exp","eventType":"subscription.expired","created_at":1731412725123,
		"object":{"id":"sub_1","object":"subscription","product":"prod_1","customer":"cust_1","status":"expired",
		"metadata":{"relay_reference":"`+creemFixtureRef+`"}}}`))
	if ev.Type != EventSubscriptionUpdated || ev.Subscription == nil || ev.Subscription.Status != SubscriptionPastDue ||
		ev.Subscription.ProviderStatus != "expired" {
		t.Errorf("expired = %+v, %+v", ev, ev.Subscription)
	}
	checkReference(t, ev)
}

func TestCreemWebhookRefundAndDispute(t *testing.T) {
	ev := parseCreemFixture(t, "refund_created")
	r := ev.Refund
	if ev.Type != EventRefundUpdated || r == nil || r.ExternalID != "ref_3DB9NQFvk18TJwSqd0N6bd" ||
		r.ExternalPaymentID != "tran_5yMaWzAl3jxuGJMCOrYWwk" || r.AmountMinor != 3158 || r.Currency != "USD" ||
		r.Status != RefundSucceeded {
		t.Errorf("refund = %+v", r)
	}
	checkReference(t, ev)

	ev = parseCreemFixture(t, "dispute_created")
	if ev.Type != EventDisputeOpened {
		t.Errorf("type = %q", ev.Type)
	}
	checkTotals(t, ev.Payment, 2610, 0, 548, 3158)
	if ev.Payment.ExternalID != "tran_5yMaWzAl3jxuGJMCOrYWwk" || ev.Payment.ExternalSubscriptionID != "sub_6pC2lNoI8ixy1kAoEqdhWl" {
		t.Errorf("payment = %+v", ev.Payment)
	}
	checkReference(t, ev)
}

func TestCreemWebhookIgnoresOtherEvents(t *testing.T) {
	ev := parseCreemFixture(t, "credits_granted")
	if ev.Type != EventIgnored || ev.ProviderType != "credits.granted" || ev.Reference != nil {
		t.Errorf("event = %+v", ev)
	}
}

func TestCreemWebhookRejectsBadSignatures(t *testing.T) {
	g, _ := newCreemWithAPI(t)
	body := creemFixture(t, "checkout_completed")
	signed := signCreem(body)

	tampered := bytes.Replace(body, []byte(`"amount_paid": 3158`), []byte(`"amount_paid": 1`), 1)
	if bytes.Equal(tampered, body) {
		t.Fatal("the tampered fixture is unchanged")
	}
	other := http.Header{}
	other.Set(creemSignatureHeader, strings.Repeat("ab", sha256.Size))
	cases := map[string]struct {
		header http.Header
		body   []byte
	}{
		"tampered body": {signed, tampered},
		"no signature":  {http.Header{}, body},
		"wrong secret":  {other, body},
		"not hex":       {http.Header{http.CanonicalHeaderKey(creemSignatureHeader): {"not-hex"}}, body},
		"empty body":    {signed, nil},
	}
	for name, tc := range cases {
		if _, err := g.ParseWebhook(context.Background(), tc.header, tc.body); !errors.Is(err, ErrInvalidSignature) {
			t.Errorf("%s: err = %v, want ErrInvalidSignature", name, err)
		}
	}

	// Signed, but not an event.
	junk := []byte(`{"object":{}}`)
	if _, err := g.ParseWebhook(context.Background(), signCreem(junk), junk); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("no id: err = %v, want ErrInvalidRequest", err)
	}
}

func TestCreemPaymentMethodIsUnsupported(t *testing.T) {
	g, api := newCreemWithAPI(t)
	if _, err := g.Card(context.Background(), CardRequest{ExternalCustomerID: "cust_9"}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
	if api.count() != 0 {
		t.Fatal("no call should reach Creem")
	}
}

func TestCreemPortalURL(t *testing.T) {
	g, api := newCreemWithAPI(t)
	api.handle("POST /v1/customers/billing", http.StatusOK, `{"customer_portal_link":"https://creem.io/my-orders/login/x"}`)
	url, err := g.PortalURL(context.Background(), PortalRequest{ExternalCustomerID: "cust_9", ReturnURL: "http://app.test/billing"})
	if err != nil || url != "https://creem.io/my-orders/login/x" {
		t.Fatalf("url = %q, %v", url, err)
	}
	if b := api.last().Body; b["customer_id"] != "cust_9" {
		t.Errorf("body = %v", b)
	}

	api.handle("POST /v1/customers/billing", http.StatusOK, `{}`)
	if _, err := g.PortalURL(context.Background(), PortalRequest{ExternalCustomerID: "cust_9"}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("empty link: err = %v", err)
	}
	if _, err := g.PortalURL(context.Background(), PortalRequest{}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("no customer: err = %v", err)
	}
}

// creemFixtureObject is the object inside a webhook fixture: what Creem's API
// returns when the same object is read back.
func creemFixtureObject(t *testing.T, name string) string {
	t.Helper()
	var env struct {
		Object json.RawMessage `json:"object"`
	}
	if err := json.Unmarshal(creemFixture(t, name), &env); err != nil {
		t.Fatal(err)
	}
	return string(env.Object)
}

func TestCreemFetchCheckout(t *testing.T) {
	g, api := newCreemWithAPI(t)
	api.handle("GET /v1/checkouts", http.StatusOK, creemFixtureObject(t, "checkout_completed"))
	ev, err := g.FetchCheckout(context.Background(), "ch_1")
	if err != nil {
		t.Fatal(err)
	}
	if api.last().Query != "checkout_id=ch_1" {
		t.Errorf("query = %q", api.last().Query)
	}
	if ev.Type != EventCheckoutCompleted || ev.ExternalID != "fetch_ch_1" || len(ev.Payload) == 0 {
		t.Errorf("event = %+v", ev)
	}
	checkReference(t, ev)
	if p := ev.Payment; p == nil || p.ExternalID != "tran_5yMaWzAl3jxuGJMCOrYWwk" || p.PaidAt == nil {
		t.Errorf("payment = %+v", p)
	}
	again, err := g.FetchCheckout(context.Background(), "ch_1")
	if err != nil || again.ExternalID != ev.ExternalID {
		t.Errorf("second fetch: %q, %v; want the same event ID", again.ExternalID, err)
	}

	api.handle("GET /v1/checkouts", http.StatusOK, `{"id":"ch_2","status":"pending","metadata":{}}`)
	if ev, err := g.FetchCheckout(context.Background(), "ch_2"); err != nil || ev.Type != EventIgnored {
		t.Errorf("pending checkout: %+v, %v", ev, err)
	}
	api.handle("GET /v1/checkouts", http.StatusNotFound, `{"trace_id":"t"}`)
	if _, err := g.FetchCheckout(context.Background(), "ch_3"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown checkout: %v", err)
	}
}

func TestCreemFetchSubscription(t *testing.T) {
	g, api := newCreemWithAPI(t)
	api.handle("GET /v1/subscriptions", http.StatusOK, creemFixtureObject(t, "subscription_paid"))
	ev, err := g.FetchSubscription(context.Background(), "sub_1")
	if err != nil {
		t.Fatal(err)
	}
	if api.last().Query != "subscription_id=sub_1" {
		t.Errorf("query = %q", api.last().Query)
	}
	if ev.Type != EventPaymentSucceeded || ev.Payment == nil || ev.Subscription == nil {
		t.Fatalf("paid subscription: %+v", ev)
	}
	again, _ := g.FetchSubscription(context.Background(), "sub_1")
	if again.ExternalID != ev.ExternalID {
		t.Errorf("same state, new event ID: %q then %q", ev.ExternalID, again.ExternalID)
	}

	api.handle("GET /v1/subscriptions", http.StatusOK, creemFixtureObject(t, "subscription_canceled"))
	canceled, err := g.FetchSubscription(context.Background(), "sub_1")
	if err != nil || canceled.Type != EventSubscriptionCanceled || canceled.ExternalID == ev.ExternalID {
		t.Errorf("canceled subscription: %+v, %v", canceled, err)
	}

	api.handle("GET /v1/subscriptions", http.StatusOK, creemFixtureObject(t, "subscription_scheduled_cancel"))
	sched, err := g.FetchSubscription(context.Background(), "sub_1")
	if err != nil || sched.Subscription == nil || !sched.Subscription.CancelAtPeriodEnd {
		t.Errorf("scheduled cancel: %+v, %v", sched, err)
	}
}

// Creem doesn't document canceling a canceled subscription, so a cancel reads
// the subscription first and leaves one already canceled alone.
func TestCreemCancelOfACanceledSubscriptionIsDone(t *testing.T) {
	g, api := newCreemWithAPI(t)
	api.handle("GET /v1/subscriptions", http.StatusOK, subscriptionResponse("canceled", `"2024-11-20T10:00:00.000Z"`))
	api.handle("POST /v1/subscriptions/sub_1/cancel", http.StatusBadRequest, `{"trace_id":"t","status":400}`)
	s, err := g.CancelSubscription(context.Background(), CancelRequest{ExternalSubscriptionID: "sub_1", Mode: CancelNow})
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != SubscriptionCanceled {
		t.Errorf("subscription = %+v, want canceled", s)
	}
	if c := api.last(); c.Method != http.MethodGet {
		t.Errorf("last call = %s %s, want no cancel of a canceled subscription", c.Method, c.Path)
	}
}

// A cancel Creem refuses counts as done once a read shows the subscription
// canceled (a cancel that raced ours).
func TestCreemCancelRefusedButCanceledIsDone(t *testing.T) {
	g, api := newCreemWithAPI(t)
	reads := 0
	api.routes["GET /v1/subscriptions"] = func(w http.ResponseWriter, _ creemCall) {
		status := "active"
		if reads++; reads > 1 {
			status = "canceled"
		}
		_, _ = io.WriteString(w, subscriptionResponse(status, "null"))
	}
	api.handle("POST /v1/subscriptions/sub_1/cancel", http.StatusBadRequest, `{"trace_id":"t","status":400}`)
	s, err := g.CancelSubscription(context.Background(), CancelRequest{ExternalSubscriptionID: "sub_1", Mode: CancelNow})
	if err != nil || s.Status != SubscriptionCanceled {
		t.Fatalf("cancel = %+v, %v; want canceled", s, err)
	}

	// Still active after the refusal: the refusal stands, and it is permanent.
	g, api = newCreemWithAPI(t)
	api.handle("GET /v1/subscriptions", http.StatusOK, subscriptionResponse("active", "null"))
	api.handle("POST /v1/subscriptions/sub_1/cancel", http.StatusBadRequest, `{"trace_id":"t","status":400}`)
	_, err = g.CancelSubscription(context.Background(), CancelRequest{ExternalSubscriptionID: "sub_1", Mode: CancelNow})
	if !errors.Is(err, ErrInvalidRequest) || !Permanent(err) {
		t.Fatalf("refused cancel: err = %v, want a permanent ErrInvalidRequest", err)
	}
}
