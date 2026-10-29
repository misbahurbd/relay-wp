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
	if c.EmbeddedCheckout || c.Installments || c.PartialRefunds || c.PublishableKey != "" {
		t.Errorf("Creem has no client secret, installments or partial refunds: %+v", c)
	}
	if !c.MerchantOfRecord || !c.RenewsSubscriptions || !c.Coupons {
		t.Errorf("Creem is the merchant of record, renews subscriptions and takes discount codes: %+v", c)
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
