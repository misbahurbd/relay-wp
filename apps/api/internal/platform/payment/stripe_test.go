package payment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v87"
	"github.com/stripe/stripe-go/v87/webhook"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

var _ Gateway = (*stripeGateway)(nil)

const stripeTestWebhookSecret = "whsec_test_relay" //nolint:gosec // test value, not a credential

// fixtureReference is relay_reference in every testdata/stripe_*.json fixture.
var fixtureReference = uuid.MustParse("0199a0b2-7c3e-7d4a-9b1e-2f6a3c5d8e01")

// stripeCall is one request the fake Stripe API received.
type stripeCall struct {
	Method, Path, IdempotencyKey string
	Form                         url.Values
}

// stripeResponse answers one route: a status and a JSON body.
type stripeResponse struct {
	Status int
	Body   string
}

// fakeStripeAPI is an httptest stand-in for api.stripe.com. Routes are keyed
// by "METHOD /v1/path"; an unknown route answers 404 resource_missing.
type fakeStripeAPI struct {
	mu     sync.Mutex
	routes map[string]stripeResponse
	calls  []stripeCall
}

func (f *fakeStripeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	f.calls = append(f.calls, stripeCall{
		Method: r.Method, Path: r.URL.Path, IdempotencyKey: r.Header.Get("Idempotency-Key"), Form: r.Form,
	})
	resp, ok := f.routes[r.Method+" "+r.URL.Path]
	f.mu.Unlock()
	if !ok {
		resp = stripeResponse{http.StatusNotFound, `{"error":{"type":"invalid_request_error","code":"resource_missing",` +
			`"message":"No such object"}}`}
	}
	if resp.Status == 0 {
		resp.Status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.Status)
	_, _ = w.Write([]byte(resp.Body))
}

func (f *fakeStripeAPI) on(route, body string) { f.respond(route, http.StatusOK, body) }

func (f *fakeStripeAPI) respond(route string, status int, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes[route] = stripeResponse{status, body}
}

// call returns the only request made to route, failing the test otherwise.
func (f *fakeStripeAPI) call(t *testing.T, route string) stripeCall {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	var found []stripeCall
	for _, c := range f.calls {
		if c.Method+" "+c.Path == route {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s called %d times, want 1 (calls: %v)", route, len(found), f.routesCalled())
	}
	return found[0]
}

func (f *fakeStripeAPI) routesCalled() []string {
	out := make([]string, 0, len(f.calls))
	for _, c := range f.calls {
		out = append(out, c.Method+" "+c.Path)
	}
	return out
}

func (f *fakeStripeAPI) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

var stripeTestNow = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

// newTestStripe builds the gateway against a fake Stripe API, with no retries.
func newTestStripe(t *testing.T) (*stripeGateway, *fakeStripeAPI) {
	t.Helper()
	api := &fakeStripeAPI{routes: map[string]stripeResponse{}}
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	g, err := newStripe(config.Payment{ //nolint:gosec // test keys against a fake API
		StripeSecretKey: "sk_test_relay", StripePublishableKey: "pk_test_relay", StripeWebhookSecret: stripeTestWebhookSecret,
	}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	g.sc = newStripeClient(g.secretKey, g.client, stripe.String(srv.URL), stripe.Int64(0))
	g.now = func() time.Time { return stripeTestNow }
	return g, api
}

func wantField(t *testing.T, c stripeCall, key, want string) {
	t.Helper()
	if got := c.Form.Get(key); got != want {
		t.Errorf("%s %s: %s = %q, want %q", c.Method, c.Path, key, got, want)
	}
}

func TestNewStripeNeedsEveryKey(t *testing.T) {
	_, err := newStripe(config.Payment{StripeSecretKey: "sk", StripePublishableKey: "pk"}, http.DefaultClient)
	if err == nil {
		t.Fatal("missing webhook secret: want an error at startup")
	}
}

func TestStripeCapabilities(t *testing.T) {
	g, _ := newTestStripe(t)
	if g.Provider() != Stripe {
		t.Fatalf("provider = %q", g.Provider())
	}
	c := g.Capabilities()
	if !c.EmbeddedCheckout || !c.RenewsSubscriptions || !c.Installments || !c.Coupons || !c.PartialRefunds ||
		c.MerchantOfRecord || c.PublishableKey != "pk_test_relay" || len(c.Currencies) != 1 || c.Currencies[0] != "USD" {
		t.Fatalf("capabilities = %+v", c)
	}
}

func TestStripeCheckoutPaymentCreatesCustomerAndPaymentIntent(t *testing.T) {
	g, api := newTestStripe(t)
	api.on("POST /v1/customers", `{"id":"cus_new","object":"customer"}`)
	api.on("POST /v1/payment_intents", `{"id":"pi_1","object":"payment_intent","client_secret":"pi_1_secret_x",`+
		`"amount":49900,"currency":"usd","status":"requires_payment_method"}`)
	ws := uuid.New()
	req := CheckoutRequest{
		Reference: uuid.New(), IdempotencyKey: "ck1", Mode: ModePayment, AmountMinor: 49900, Currency: "USD",
		Description: "Lifetime, Growth",
		Customer: Customer{
			WorkspaceID: ws, Name: "Ada", Email: "ada@example.com",
			Address: Address{Line1: "1 Main St", City: "Dhaka", Country: "BD"}, TaxIDType: "eu_vat", TaxID: "DE123",
		},
	}
	c, err := g.CreateCheckout(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if c.ExternalSessionID != "pi_1" || c.ClientSecret != "pi_1_secret_x" || c.ExternalCustomerID != "cus_new" ||
		c.ExternalSubscriptionID != "" || !c.ExpiresAt.Equal(stripeTestNow.Add(23*time.Hour)) {
		t.Fatalf("checkout = %+v", c)
	}

	cu := api.call(t, "POST /v1/customers")
	wantField(t, cu, "email", "ada@example.com")
	wantField(t, cu, "address[country]", "BD")
	wantField(t, cu, "tax_id_data[0][type]", "eu_vat")
	wantField(t, cu, "metadata[relay_workspace_id]", ws.String())
	if cu.IdempotencyKey != "ck1:customer" {
		t.Errorf("customer idempotency key = %q", cu.IdempotencyKey)
	}
	pi := api.call(t, "POST /v1/payment_intents")
	wantField(t, pi, "amount", "49900")
	wantField(t, pi, "currency", "usd")
	wantField(t, pi, "customer", "cus_new")
	wantField(t, pi, "automatic_payment_methods[enabled]", "true")
	wantField(t, pi, "metadata[relay_reference]", req.Reference.String())
	if pi.IdempotencyKey != "ck1:payment_intent" {
		t.Errorf("payment intent idempotency key = %q", pi.IdempotencyKey)
	}
}

func TestStripeCheckoutSubscriptionWithCoupon(t *testing.T) {
	g, api := newTestStripe(t)
	api.on("POST /v1/subscriptions", `{"id":"sub_1","object":"subscription","status":"incomplete",`+
		`"latest_invoice":{"id":"in_1","object":"invoice","confirmation_secret":{"client_secret":"pi_2_secret_y","type":"payment_intent"}}}`)
	req := CheckoutRequest{
		Reference: uuid.New(), IdempotencyKey: "ck2", Mode: ModeSubscription, ExternalPriceID: "price_growth_month",
		AmountMinor: 2320, Currency: "USD", ExternalCouponID: "coupon_20", Customer: Customer{ExternalID: "cus_1"},
	}
	c, err := g.CreateCheckout(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if c.ExternalSessionID != "sub_1" || c.ExternalSubscriptionID != "sub_1" || c.ClientSecret != "pi_2_secret_y" ||
		c.ExternalCustomerID != "cus_1" {
		t.Fatalf("checkout = %+v", c)
	}
	if api.callCount() != 1 {
		t.Fatalf("calls = %v, want only the subscription (customer exists)", api.routesCalled())
	}
	s := api.call(t, "POST /v1/subscriptions")
	wantField(t, s, "customer", "cus_1")
	wantField(t, s, "items[0][price]", "price_growth_month")
	wantField(t, s, "payment_behavior", "default_incomplete")
	wantField(t, s, "payment_settings[save_default_payment_method]", "on_subscription")
	wantField(t, s, "discounts[0][coupon]", "coupon_20")
	wantField(t, s, "metadata[relay_reference]", req.Reference.String())
	wantField(t, s, "expand[0]", "latest_invoice.confirmation_secret")
}

func TestStripeCheckoutTrialUsesSetupIntent(t *testing.T) {
	g, api := newTestStripe(t)
	api.on("POST /v1/subscriptions", `{"id":"sub_1","object":"subscription","status":"trialing",`+
		`"latest_invoice":{"id":"in_0","object":"invoice"},"pending_setup_intent":{"id":"seti_1","object":"setup_intent","client_secret":"seti_1_secret_z"}}`)
	c, err := g.CreateCheckout(context.Background(), CheckoutRequest{
		Reference: uuid.New(), Mode: ModeSubscription, ExternalPriceID: "price_growth_month", Currency: "USD",
		TrialDays: 14, Customer: Customer{ExternalID: "cus_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.ClientSecret != "seti_1_secret_z" {
		t.Fatalf("client secret = %q", c.ClientSecret)
	}
	s := api.call(t, "POST /v1/subscriptions")
	wantField(t, s, "trial_period_days", "14")
}

func TestStripeCheckoutInstallmentsBuildsSchedule(t *testing.T) {
	g, api := newTestStripe(t)
	api.on("GET /v1/prices/price_ltd_3x", `{"id":"price_ltd_3x","object":"price","unit_amount":10000,"currency":"usd",`+
		`"product":"prod_ltd","recurring":{"interval":"month","interval_count":1}}`)
	api.on("POST /v1/subscriptions", `{"id":"sub_9","object":"subscription","status":"incomplete",`+
		`"latest_invoice":{"id":"in_9","object":"invoice","confirmation_secret":{"client_secret":"pi_9_secret"}}}`)
	api.on("POST /v1/subscription_schedules", `{"id":"sub_sched_9","object":"subscription_schedule",`+
		`"phases":[{"start_date":1761739200,"end_date":1764417600}]}`)
	api.on("POST /v1/subscription_schedules/sub_sched_9", `{"id":"sub_sched_9","object":"subscription_schedule"}`)
	req := CheckoutRequest{
		Reference: uuid.New(), IdempotencyKey: "ck3", Mode: ModeInstallments, ExternalPriceID: "price_ltd_3x",
		AmountMinor: 10000, Currency: "USD", Schedule: []int64{10000, 10000, 10002},
		ExternalCouponID: "coupon_ignored", Customer: Customer{ExternalID: "cus_1"},
	}
	c, err := g.CreateCheckout(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if c.ExternalSubscriptionID != "sub_9" || c.ExternalScheduleID != "sub_sched_9" || c.ClientSecret != "pi_9_secret" {
		t.Fatalf("checkout = %+v", c)
	}

	s := api.call(t, "POST /v1/subscriptions")
	wantField(t, s, "items[0][price]", "price_ltd_3x")
	wantField(t, s, "discounts[0][coupon]", "") // the discount is already off the schedule
	wantField(t, api.call(t, "POST /v1/subscription_schedules"), "from_subscription", "sub_9")

	up := api.call(t, "POST /v1/subscription_schedules/sub_sched_9")
	wantField(t, up, "end_behavior", "cancel")
	wantField(t, up, "phases[0][start_date]", "1761739200")
	wantField(t, up, "phases[0][items][0][price]", "price_ltd_3x")
	wantField(t, up, "phases[0][duration][interval]", "month")
	wantField(t, up, "phases[0][duration][interval_count]", "2")
	wantField(t, up, "phases[1][items][0][price_data][unit_amount]", "10002")
	wantField(t, up, "phases[1][items][0][price_data][product]", "prod_ltd")
	wantField(t, up, "phases[1][items][0][price_data][recurring][interval]", "month")
	wantField(t, up, "phases[1][duration][interval_count]", "1")
	wantField(t, up, "phases[1][metadata][relay_reference]", req.Reference.String())
	wantField(t, up, "metadata[relay_schedule]", "sub_sched_9")
	if up.IdempotencyKey != "ck3:schedule_phases" {
		t.Errorf("schedule update idempotency key = %q", up.IdempotencyKey)
	}
}

func TestStripeCheckoutInstallmentsFirstAmountOffPrice(t *testing.T) {
	g, api := newTestStripe(t)
	api.on("GET /v1/prices/price_ltd", `{"id":"price_ltd","object":"price","unit_amount":10000,"currency":"usd",`+
		`"product":"prod_ltd","recurring":{"interval":"month","interval_count":1}}`)
	api.on("POST /v1/subscriptions", `{"id":"sub_9","object":"subscription",`+
		`"latest_invoice":{"id":"in_9","object":"invoice","confirmation_secret":{"client_secret":"pi_9_secret"}}}`)
	api.on("POST /v1/subscription_schedules", `{"id":"sub_sched_9","object":"subscription_schedule","phases":[{"start_date":1}]}`)
	api.on("POST /v1/subscription_schedules/sub_sched_9", `{"id":"sub_sched_9","object":"subscription_schedule"}`)
	_, err := g.CreateCheckout(context.Background(), CheckoutRequest{
		Reference: uuid.New(), Mode: ModeInstallments, ExternalPriceID: "price_ltd", Currency: "USD",
		Schedule: []int64{7000, 10000}, Customer: Customer{ExternalID: "cus_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := api.call(t, "POST /v1/subscriptions")
	wantField(t, s, "items[0][price]", "")
	wantField(t, s, "items[0][price_data][unit_amount]", "7000")
	up := api.call(t, "POST /v1/subscription_schedules/sub_sched_9")
	wantField(t, up, "phases[0][items][0][price_data][unit_amount]", "7000")
	wantField(t, up, "phases[1][items][0][price]", "price_ltd")
}

func TestStripeCheckoutRejectsBadRequests(t *testing.T) {
	g, api := newTestStripe(t)
	base := CheckoutRequest{Reference: uuid.New(), Currency: "USD", Customer: Customer{ExternalID: "cus_1"}}
	cases := map[string]func(r *CheckoutRequest){
		"no reference":             func(r *CheckoutRequest) { r.Mode, r.AmountMinor, r.Reference = ModePayment, 100, uuid.Nil },
		"no amount":                func(r *CheckoutRequest) { r.Mode = ModePayment },
		"subscription no price":    func(r *CheckoutRequest) { r.Mode = ModeSubscription },
		"installments one":         func(r *CheckoutRequest) { r.Mode, r.ExternalPriceID, r.Schedule = ModeInstallments, "p", []int64{1} },
		"installments zero":        func(r *CheckoutRequest) { r.Mode, r.ExternalPriceID, r.Schedule = ModeInstallments, "p", []int64{1, 0} },
		"unknown mode":             func(r *CheckoutRequest) { r.Mode = "barter" },
		"payment without currency": func(r *CheckoutRequest) { r.Mode, r.AmountMinor, r.Currency = ModePayment, 100, "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := base
			mutate(&req)
			if _, err := g.CreateCheckout(context.Background(), req); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("err = %v, want ErrInvalidRequest", err)
			}
		})
	}
	if n := api.callCount(); n != 0 {
		t.Fatalf("bad requests reached Stripe %d times", n)
	}
}

func TestStripeInstallmentsNeedRecurringPrice(t *testing.T) {
	g, api := newTestStripe(t)
	api.on("GET /v1/prices/price_once", `{"id":"price_once","object":"price","unit_amount":10000,"product":"prod_ltd"}`)
	_, err := g.CreateCheckout(context.Background(), CheckoutRequest{
		Reference: uuid.New(), Mode: ModeInstallments, ExternalPriceID: "price_once", Currency: "USD",
		Schedule: []int64{5000, 5000}, Customer: Customer{ExternalID: "cus_1"},
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("err = %v, want ErrInvalidRequest", err)
	}
}

const stripeSubscriptionJSON = `{"id":"sub_1","object":"subscription","customer":"cus_1","status":"active",` +
	`"cancel_at_period_end":%t,"canceled_at":%s,"items":{"object":"list","data":[{"id":"si_1","object":"subscription_item",` +
	`"current_period_start":1761696000,"current_period_end":1764374400,"price":{"id":"%s","object":"price"}}]}%s}`

func TestStripeChangeSubscriptionProratesUpgrades(t *testing.T) {
	g, api := newTestStripe(t)
	api.on("GET /v1/subscriptions/sub_1", fmt.Sprintf(stripeSubscriptionJSON, false, "null", "price_starter", ""))
	api.on("POST /v1/subscriptions/sub_1", fmt.Sprintf(stripeSubscriptionJSON, false, "null", "price_growth",
		`,"latest_invoice":{"id":"in_p","object":"invoice","billing_reason":"subscription_update","total":1450}`))

	s, err := g.ChangeSubscription(context.Background(), ChangeRequest{
		ExternalSubscriptionID: "sub_1", ExternalPriceID: "price_growth", ProrateNow: true, IdempotencyKey: "chg1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.ExternalPriceID != "price_growth" || s.Status != SubscriptionActive || s.ProrationMinor != 1450 ||
		s.CurrentPeriodEnd == nil || !s.CurrentPeriodEnd.Equal(time.Unix(1764374400, 0)) {
		t.Fatalf("subscription = %+v", s)
	}
	up := api.call(t, "POST /v1/subscriptions/sub_1")
	wantField(t, up, "items[0][id]", "si_1")
	wantField(t, up, "items[0][price]", "price_growth")
	wantField(t, up, "proration_behavior", "always_invoice")
	wantField(t, up, "payment_behavior", "error_if_incomplete")
	if up.IdempotencyKey != "chg1:change" {
		t.Errorf("idempotency key = %q", up.IdempotencyKey)
	}
}

func TestStripeChangeSubscriptionDowngradeDoesNotProrate(t *testing.T) {
	g, api := newTestStripe(t)
	api.on("GET /v1/subscriptions/sub_1", fmt.Sprintf(stripeSubscriptionJSON, false, "null", "price_growth", ""))
	api.on("POST /v1/subscriptions/sub_1", fmt.Sprintf(stripeSubscriptionJSON, false, "null", "price_starter",
		`,"latest_invoice":{"id":"in_old","object":"invoice","billing_reason":"subscription_cycle","total":2900}`))
	s, err := g.ChangeSubscription(context.Background(), ChangeRequest{ExternalSubscriptionID: "sub_1", ExternalPriceID: "price_starter"})
	if err != nil {
		t.Fatal(err)
	}
	if s.ProrationMinor != 0 {
		t.Fatalf("proration = %d, want 0", s.ProrationMinor)
	}
	up := api.call(t, "POST /v1/subscriptions/sub_1")
	wantField(t, up, "proration_behavior", "none")
	wantField(t, up, "payment_behavior", "")
}

func TestStripeCancelSubscription(t *testing.T) {
	g, api := newTestStripe(t)
	api.on("DELETE /v1/subscriptions/sub_1", strings.Replace(
		fmt.Sprintf(stripeSubscriptionJSON, false, "1761700000", "price_growth", ""), `"active"`, `"canceled"`, 1))
	api.on("POST /v1/subscriptions/sub_1", fmt.Sprintf(stripeSubscriptionJSON, true, "null", "price_growth", ""))
	ctx := context.Background()

	s, err := g.CancelSubscription(ctx, CancelRequest{ExternalSubscriptionID: "sub_1", Mode: CancelNow})
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != SubscriptionCanceled || s.CanceledAt == nil {
		t.Fatalf("cancel now = %+v", s)
	}
	s, err = g.CancelSubscription(ctx, CancelRequest{ExternalSubscriptionID: "sub_1", Mode: CancelAtPeriodEnd})
	if err != nil {
		t.Fatal(err)
	}
	if !s.CancelAtPeriodEnd {
		t.Fatalf("cancel at period end = %+v", s)
	}
	wantField(t, api.call(t, "POST /v1/subscriptions/sub_1"), "cancel_at_period_end", "true")

	if _, err := g.CancelSubscription(ctx, CancelRequest{ExternalSubscriptionID: "sub_1", Mode: CancelUndo}); err != nil {
		t.Fatal(err)
	}
	if _, err := g.CancelSubscription(ctx, CancelRequest{ExternalSubscriptionID: "sub_1", Mode: "later"}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("bad mode: err = %v", err)
	}
}

func TestStripeRefund(t *testing.T) {
	g, api := newTestStripe(t)
	api.on("POST /v1/refunds", `{"id":"re_1","object":"refund","amount":1000,"currency":"usd","payment_intent":"pi_1","status":"pending"}`)
	r, err := g.Refund(context.Background(), RefundRequest{
		ExternalPaymentID: "pi_1", AmountMinor: 1000, Currency: "USD", Reason: "requested_by_customer", IdempotencyKey: "rf1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r != (Refund{ExternalID: "re_1", ExternalPaymentID: "pi_1", AmountMinor: 1000, Currency: "USD", Status: RefundPending}) {
		t.Fatalf("refund = %+v", r)
	}
	c := api.call(t, "POST /v1/refunds")
	wantField(t, c, "payment_intent", "pi_1")
	wantField(t, c, "amount", "1000")
	wantField(t, c, "reason", "requested_by_customer")
	if c.IdempotencyKey != "rf1:refund" {
		t.Errorf("idempotency key = %q", c.IdempotencyKey)
	}
}

func TestStripeRefundOfInvoiceFindsItsPaymentIntent(t *testing.T) {
	g, api := newTestStripe(t)
	api.on("GET /v1/invoice_payments", `{"object":"list","has_more":false,"url":"/v1/invoice_payments","data":[`+
		`{"id":"inpay_1","object":"invoice_payment","status":"paid","payment":{"type":"payment_intent","payment_intent":"pi_inv"}}]}`)
	api.on("POST /v1/refunds", `{"id":"re_2","object":"refund","amount":500,"currency":"usd","payment_intent":"pi_inv","status":"succeeded"}`)
	r, err := g.Refund(context.Background(), RefundRequest{
		ExternalPaymentID: "in_1", AmountMinor: 500, Currency: "USD", Reason: "goodwill credit",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != RefundSucceeded || r.ExternalPaymentID != "pi_inv" {
		t.Fatalf("refund = %+v", r)
	}
	wantField(t, api.call(t, "GET /v1/invoice_payments"), "invoice", "in_1")
	c := api.call(t, "POST /v1/refunds")
	wantField(t, c, "payment_intent", "pi_inv")
	wantField(t, c, "reason", "")
	wantField(t, c, "metadata[relay_reason]", "goodwill credit")
}

func TestStripeCreateCoupon(t *testing.T) {
	g, api := newTestStripe(t)
	api.on("POST /v1/coupons", `{"id":"coupon_abc","object":"coupon"}`)
	id := uuid.New()
	c, err := g.CreateCoupon(context.Background(), CouponRequest{
		DiscountID: id, Code: "LAUNCH", Name: "Launch", PercentBps: 1250, Duration: "repeating", DurationMonths: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.ExternalCouponID != "coupon_abc" {
		t.Fatalf("coupon = %+v", c)
	}
	call := api.call(t, "POST /v1/coupons")
	wantField(t, call, "percent_off", "12.5000")
	wantField(t, call, "duration", "repeating")
	wantField(t, call, "duration_in_months", "3")
	wantField(t, call, "metadata[relay_discount_id]", id.String())
	if call.IdempotencyKey != "coupon:"+id.String() {
		t.Errorf("idempotency key = %q", call.IdempotencyKey)
	}

	g2, api2 := newTestStripe(t)
	api2.on("POST /v1/coupons", `{"id":"coupon_def","object":"coupon"}`)
	if _, err := g2.CreateCoupon(context.Background(), CouponRequest{
		DiscountID: uuid.New(), AmountOffMinor: 500, Currency: "USD", Duration: "once",
	}); err != nil {
		t.Fatal(err)
	}
	call = api2.call(t, "POST /v1/coupons")
	wantField(t, call, "amount_off", "500")
	wantField(t, call, "currency", "usd")
	wantField(t, call, "percent_off", "")

	for name, req := range map[string]CouponRequest{
		"both":         {PercentBps: 1000, AmountOffMinor: 100, Currency: "USD", Duration: "once"},
		"neither":      {Duration: "once"},
		"no months":    {PercentBps: 1000, Duration: "repeating"},
		"bad duration": {PercentBps: 1000, Duration: "sometimes"},
		"no currency":  {AmountOffMinor: 100, Duration: "once"},
	} {
		if _, err := g.CreateCoupon(context.Background(), req); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%s: err = %v, want ErrInvalidRequest", name, err)
		}
	}
}

func TestStripeErrorsMapToGatewayErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"declined", http.StatusPaymentRequired, `{"error":{"type":"card_error","code":"card_declined",` +
			`"decline_code":"insufficient_funds","message":"Your card was declined.",` +
			`"payment_intent":{"id":"pi_1","client_secret":"pi_1_secret_leak"}}}`, ErrDeclined},
		{"not found", http.StatusNotFound, `{"error":{"type":"invalid_request_error","code":"resource_missing",` +
			`"message":"No such subscription: 'sub_x'"}}`, ErrNotFound},
		{"invalid", http.StatusBadRequest, `{"error":{"type":"invalid_request_error","code":"parameter_missing",` +
			`"message":"Missing required param: price."}}`, ErrInvalidRequest},
		{"idempotency", http.StatusBadRequest, `{"error":{"type":"idempotency_error","message":"Keys for idempotent ` +
			`requests can only be used with the same parameters"}}`, ErrInvalidRequest},
		{"rate limited", http.StatusTooManyRequests, `{"error":{"type":"invalid_request_error","code":"rate_limit",` +
			`"message":"Too many requests"}}`, ErrUnavailable},
		{
			"server error", http.StatusInternalServerError, `{"error":{"type":"api_error","message":"Something went wrong"}}`,
			ErrUnavailable,
		},
		{
			"bad key", http.StatusUnauthorized, `{"error":{"type":"invalid_request_error","message":"Invalid API Key"}}`,
			ErrUnavailable,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, api := newTestStripe(t)
			api.respond("DELETE /v1/subscriptions/sub_1", tc.status, tc.body)
			_, err := g.CancelSubscription(context.Background(), CancelRequest{ExternalSubscriptionID: "sub_1", Mode: CancelNow})
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("error leaks a client secret: %v", err)
			}
		})
	}
}

func TestStripeUnreachableIsUnavailable(t *testing.T) {
	g, _ := newTestStripe(t)
	srv := httptest.NewServer(http.NotFoundHandler())
	dead := srv.URL
	srv.Close()
	g.sc = newStripeClient(g.secretKey, g.client, stripe.String(dead), stripe.Int64(0))
	_, err := g.CancelSubscription(context.Background(), CancelRequest{ExternalSubscriptionID: "sub_1", Mode: CancelNow})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

// signStripe signs body as Stripe would, at ts, with secret.
func signStripe(body []byte, secret string, ts time.Time) http.Header {
	sp := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: body, Secret: secret, Timestamp: ts})
	h := http.Header{}
	h.Set("Stripe-Signature", sp.Header)
	return h
}

func readStripeFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "stripe_"+name+".json")) //nolint:gosec // fixed test fixture names
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestStripeWebhookFixtures(t *testing.T) {
	g, api := newTestStripe(t)
	api.on("GET /v1/invoice_payments", `{"object":"list","has_more":false,"url":"/v1/invoice_payments","data":[`+
		`{"id":"inpay_1","object":"invoice_payment","status":"paid","payment":{"type":"payment_intent","payment_intent":"pi_inv_1"}}]}`)
	occurred := time.Unix(1761696000, 0).UTC()

	cases := []struct {
		fixture string
		want    EventType
		ref     bool
		check   func(t *testing.T, ev Event)
	}{
		{"payment_intent_succeeded", EventCheckoutCompleted, true, func(t *testing.T, ev Event) {
			p := ev.Payment
			if p == nil || p.ExternalID != "pi_ltd_1" || p.TotalMinor != 49900 || p.Currency != "USD" ||
				p.ExternalCustomerID != "cus_1" || p.PaidAt == nil {
				t.Fatalf("payment = %+v", p)
			}
		}},
		// A declined attempt leaves the checkout open: the customer can retry.
		{"payment_intent_payment_failed", EventIgnored, true, func(t *testing.T, ev Event) {
			if ev.Payment == nil || ev.Payment.FailureCode != "insufficient_funds" || ev.Payment.PaidAt != nil {
				t.Fatalf("payment = %+v", ev.Payment)
			}
		}},
		{"payment_intent_canceled", EventCheckoutExpired, true, nil},
		{"payment_intent_succeeded_foreign", EventIgnored, false, func(t *testing.T, ev Event) {
			if ev.Payment != nil {
				t.Fatal("an ignored event carries no payment")
			}
		}},
		{"invoice_paid_subscription_create", EventCheckoutCompleted, true, func(t *testing.T, ev Event) {
			p := ev.Payment
			if p == nil || p.ExternalID != "pi_inv_1" || p.InvoiceNumber != "RELAY-0001" || p.ExternalSubscriptionID != "sub_1" ||
				p.ExternalScheduleID != "sub_sched_1" || p.SubtotalMinor != 2900 || p.DiscountMinor != 580 ||
				p.TotalMinor != 2320 || p.TotalMinor != p.SubtotalMinor-p.DiscountMinor+p.TaxMinor ||
				p.PeriodEnd == nil || p.PaidAt == nil || p.ReceiptURL == "" {
				t.Fatalf("payment = %+v", p)
			}
		}},
		{"invoice_paid", EventPaymentSucceeded, true, nil},
		{"invoice_payment_failed_subscription_create", EventIgnored, true, func(t *testing.T, ev Event) {
			if ev.Payment == nil || ev.Payment.ExternalID != "in_1" || ev.Payment.PaidAt != nil {
				t.Fatalf("payment = %+v", ev.Payment)
			}
		}},
		{"invoice_payment_failed", EventPaymentFailed, true, nil},
		{"customer_subscription_updated", EventSubscriptionUpdated, true, func(t *testing.T, ev Event) {
			s := ev.Subscription
			if s == nil || s.ExternalID != "sub_1" || s.Status != SubscriptionActive || !s.CancelAtPeriodEnd ||
				s.ExternalPriceID != "price_growth_month" || s.CurrentPeriodEnd == nil {
				t.Fatalf("subscription = %+v", s)
			}
		}},
		{"customer_subscription_trial_card_saved", EventCheckoutCompleted, true, func(t *testing.T, ev Event) {
			s, p := ev.Subscription, ev.Payment
			if s == nil || s.Status != SubscriptionTrialing || p == nil || p.ExternalID != "in_trial_1" ||
				p.TotalMinor != 0 || p.Currency != "USD" || p.ExternalSubscriptionID != "sub_trial_1" {
				t.Fatalf("subscription = %+v, payment = %+v", s, p)
			}
		}},
		{"customer_subscription_deleted", EventSubscriptionCanceled, true, func(t *testing.T, ev Event) {
			if ev.Subscription == nil || ev.Subscription.Status != SubscriptionCanceled || ev.Subscription.CanceledAt == nil {
				t.Fatalf("subscription = %+v", ev.Subscription)
			}
		}},
		{"customer_subscription_incomplete_expired", EventCheckoutExpired, true, func(t *testing.T, ev Event) {
			if ev.Subscription == nil || ev.Subscription.ProviderStatus != "incomplete_expired" {
				t.Fatalf("subscription = %+v", ev.Subscription)
			}
		}},
		{"refund_updated", EventRefundUpdated, false, func(t *testing.T, ev Event) {
			want := Refund{ExternalID: "re_1", ExternalPaymentID: "pi_ltd_1", AmountMinor: 1000, Currency: "USD", Status: RefundSucceeded}
			if ev.Refund == nil || *ev.Refund != want {
				t.Fatalf("refund = %+v", ev.Refund)
			}
		}},
		{"charge_dispute_created", EventDisputeOpened, false, func(t *testing.T, ev Event) {
			if ev.Payment == nil || ev.Payment.ExternalID != "pi_ltd_1" || ev.Payment.TotalMinor != 49900 {
				t.Fatalf("payment = %+v", ev.Payment)
			}
		}},
		{"charge_dispute_closed", EventDisputeClosed, false, func(t *testing.T, ev Event) {
			if ev.Payment == nil || ev.Payment.Status != "lost" {
				t.Fatalf("payment = %+v", ev.Payment)
			}
		}},
		{"customer_created", EventIgnored, false, nil},
	}
	covered := map[EventType]bool{}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			body := readStripeFixture(t, tc.fixture)
			ev, err := g.ParseWebhook(context.Background(), signStripe(body, stripeTestWebhookSecret, time.Now()), body)
			if err != nil {
				t.Fatal(err)
			}
			if ev.Type != tc.want || ev.Provider != Stripe || ev.ExternalID == "" || ev.ProviderType == "" ||
				!ev.OccurredAt.Equal(occurred) {
				t.Fatalf("event = %+v, want type %s", ev, tc.want)
			}
			if tc.ref && (ev.Reference == nil || *ev.Reference != fixtureReference) {
				t.Fatalf("reference = %v, want %v", ev.Reference, fixtureReference)
			}
			if !tc.ref && ev.Reference != nil {
				t.Fatalf("reference = %v, want none", ev.Reference)
			}
			if !bytes.Equal(ev.Payload, body) {
				t.Fatal("payload should be the raw verified body")
			}
			if tc.check != nil {
				tc.check(t, ev)
			}
		})
		covered[tc.want] = true
	}
	for _, typ := range []EventType{
		EventCheckoutCompleted, EventCheckoutExpired, EventPaymentSucceeded, EventPaymentFailed,
		EventSubscriptionUpdated, EventSubscriptionCanceled, EventRefundUpdated, EventDisputeOpened, EventDisputeClosed,
		EventIgnored,
	} {
		if !covered[typ] {
			t.Errorf("no fixture for %s", typ)
		}
	}
}

func TestStripeWebhookRejectsTamperedOrStale(t *testing.T) {
	g, api := newTestStripe(t)
	body := readStripeFixture(t, "payment_intent_succeeded")
	tampered := bytes.Replace(body, []byte(`"amount_received": 49900`), []byte(`"amount_received": 1`), 1)
	if bytes.Equal(tampered, body) {
		t.Fatal("fixture changed: tampering did nothing")
	}
	cases := map[string]struct {
		header http.Header
		body   []byte
	}{
		"tampered body":   {signStripe(body, stripeTestWebhookSecret, time.Now()), tampered},
		"wrong secret":    {signStripe(body, "whsec_other", time.Now()), body},
		"stale signature": {signStripe(body, stripeTestWebhookSecret, time.Now().Add(-time.Hour)), body},
		"missing header":  {http.Header{}, body},
		"garbled header":  {http.Header{"Stripe-Signature": {"t=abc,v1=zz"}}, body},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ev, err := g.ParseWebhook(context.Background(), tc.header, tc.body)
			if !errors.Is(err, ErrInvalidSignature) {
				t.Fatalf("err = %v, want ErrInvalidSignature", err)
			}
			if ev.Payload != nil {
				t.Fatal("a rejected webhook returns no payload")
			}
		})
	}
	if api.callCount() != 0 {
		t.Fatal("a rejected webhook must not call Stripe")
	}
}

func TestStripeWebhookRejectsSignedGarbage(t *testing.T) {
	g, _ := newTestStripe(t)
	body := []byte(`{"not":"an event"}`)
	if _, err := g.ParseWebhook(context.Background(), signStripe(body, stripeTestWebhookSecret, time.Now()), body); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("err = %v, want ErrInvalidRequest", err)
	}
}
