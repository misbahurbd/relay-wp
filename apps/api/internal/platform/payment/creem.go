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
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

// Creem's hosts. A test key (creem_test_…) only works against the test host,
// so the key picks it, as Creem's own CLI does (docs/PHASE-6-BILLING.md §6).
const (
	creemLiveBaseURL = "https://api.creem.io"
	creemTestBaseURL = "https://test-api.creem.io"
	creemTestKeyPref = "creem_test_"
)

const (
	// creemSignatureHeader is the hex HMAC-SHA256 of the raw webhook body.
	creemSignatureHeader = "creem-signature"
	// creemReferenceKey is the metadata key that carries our checkout_sessions.id.
	creemReferenceKey = "relay_reference"
	// creemMinCustomPrice is the smallest custom_price Creem accepts, in cents.
	creemMinCustomPrice = 100
	// creemMaxBody caps how much of a Creem response is read.
	creemMaxBody = 1 << 20
	// creemMaxProductPages stops CreateCoupon from paging forever.
	creemMaxProductPages = 50
)

// creemGateway is Creem, the merchant of record for its sales. Checkout is
// its hosted page (which the web app may also embed by URL), and Creem renews
// subscriptions itself. It has no installments and only full refunds.
type creemGateway struct {
	apiKey        string
	webhookSecret string
	baseURL       string
	client        *http.Client
}

func newCreem(cfg config.Payment, client *http.Client) (*creemGateway, error) {
	if cfg.CreemAPIKey == "" || cfg.CreemWebhookSecret == "" {
		return nil, errors.New("CREEM_API_KEY and CREEM_WEBHOOK_SECRET are required when PAYMENT_PROVIDERS includes creem")
	}
	base := creemLiveBaseURL
	if strings.HasPrefix(cfg.CreemAPIKey, creemTestKeyPref) {
		base = creemTestBaseURL
	}
	return &creemGateway{apiKey: cfg.CreemAPIKey, webhookSecret: cfg.CreemWebhookSecret, baseURL: base, client: client}, nil
}

func (g *creemGateway) Provider() Provider { return Creem }

func (g *creemGateway) Capabilities() Capabilities {
	return Capabilities{
		EmbeddableURL: true, RenewsSubscriptions: true, Coupons: true, MerchantOfRecord: true, Currencies: []string{"USD"},
	}
}

// creemCheckoutRequest is the body of POST /v1/checkouts.
type creemCheckoutRequest struct {
	ProductID    string            `json:"product_id"`
	RequestID    string            `json:"request_id,omitempty"`
	Units        int               `json:"units,omitempty"`
	CustomPrice  int64             `json:"custom_price,omitempty"`
	DiscountCode string            `json:"discount_code,omitempty"`
	Customer     *creemCustomerIn  `json:"customer,omitempty"`
	SuccessURL   string            `json:"success_url,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// creemCustomerIn prefills the checkout: an existing customer's id, or an email. Never both.
type creemCustomerIn struct {
	ID    string `json:"id,omitempty"`
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

type creemCheckout struct {
	ID           string            `json:"id"`
	RequestID    string            `json:"request_id"`
	Status       string            `json:"status"`
	CheckoutURL  string            `json:"checkout_url"`
	Customer     creemRef          `json:"customer"`
	Subscription json.RawMessage   `json:"subscription"`
	Order        *creemOrder       `json:"order"`
	Metadata     map[string]string `json:"metadata"`
}

func (g *creemGateway) CreateCheckout(ctx context.Context, req CheckoutRequest) (Checkout, error) {
	var missing []string
	if req.Reference == uuid.Nil {
		missing = append(missing, "reference")
	}
	if req.ExternalPriceID == "" {
		// Every Creem checkout names a product, a one-time payment included.
		missing = append(missing, "external price")
	}
	body := creemCheckoutRequest{
		ProductID: req.ExternalPriceID, RequestID: req.Reference.String(), Units: 1,
		SuccessURL: req.ReturnURL,
		Metadata: map[string]string{
			creemReferenceKey: req.Reference.String(), "workspace_id": req.Customer.WorkspaceID.String(),
		},
	}
	switch req.Mode {
	case ModeSubscription:
		body.DiscountCode = req.ExternalCouponID
	case ModePayment:
		// Relay has already taken any discount off AmountMinor.
		if req.AmountMinor < creemMinCustomPrice {
			missing = append(missing, "amount of at least 100")
		}
		body.CustomPrice = req.AmountMinor
	case ModeInstallments:
		return Checkout{}, fmt.Errorf("creem: installments: %w", ErrUnsupported)
	default:
		missing = append(missing, "mode")
	}
	if len(missing) > 0 {
		return Checkout{}, fmt.Errorf("creem: checkout needs %s: %w", strings.Join(missing, ", "), ErrInvalidRequest)
	}
	switch c := req.Customer; {
	case c.ExternalID != "":
		body.Customer = &creemCustomerIn{ID: c.ExternalID}
	case c.Email != "":
		body.Customer = &creemCustomerIn{Email: c.Email, Name: c.Name}
	}

	var out creemCheckout
	if err := g.do(ctx, http.MethodPost, "/v1/checkouts", body, &out); err != nil {
		return Checkout{}, fmt.Errorf("creem: create checkout: %w", err)
	}
	if out.ID == "" || out.CheckoutURL == "" {
		return Checkout{}, fmt.Errorf("creem: create checkout: no id or checkout_url in the response: %w", ErrUnavailable)
	}
	c := Checkout{ExternalSessionID: out.ID, ExternalCustomerID: out.Customer.ID, URL: out.CheckoutURL}
	if sub, err := decodeCreemSubscription(out.Subscription); err == nil && sub != nil {
		c.ExternalSubscriptionID = sub.ID
	}
	return c, nil
}

func (g *creemGateway) ChangeSubscription(ctx context.Context, req ChangeRequest) (Subscription, error) {
	if req.ExternalSubscriptionID == "" || req.ExternalPriceID == "" {
		return Subscription{}, fmt.Errorf("creem: change needs a subscription and a price: %w", ErrInvalidRequest)
	}
	// proration-none switches the product with no charge now, so the new
	// price starts with the next renewal.
	behavior := "proration-none"
	if req.ProrateNow {
		behavior = "proration-charge-immediately"
	}
	body := map[string]string{"product_id": req.ExternalPriceID, "update_behavior": behavior}
	var out creemSubscription
	path := "/v1/subscriptions/" + url.PathEscape(req.ExternalSubscriptionID) + "/upgrade"
	if err := g.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return Subscription{}, fmt.Errorf("creem: change subscription: %w", err)
	}
	return out.normalise(), nil
}

func (g *creemGateway) CancelSubscription(ctx context.Context, req CancelRequest) (Subscription, error) {
	if req.ExternalSubscriptionID == "" {
		return Subscription{}, fmt.Errorf("creem: cancel needs a subscription: %w", ErrInvalidRequest)
	}
	base := "/v1/subscriptions/" + url.PathEscape(req.ExternalSubscriptionID)
	var (
		path string
		body any
	)
	switch req.Mode {
	case CancelNow:
		path, body = base+"/cancel", map[string]string{"mode": "immediate"}
	case CancelAtPeriodEnd:
		path, body = base+"/cancel", map[string]string{"mode": "scheduled", "onExecute": "cancel"}
	case CancelUndo:
		// Resume takes a subscription out of scheduled_cancel (or paused).
		path = base + "/resume"
	default:
		return Subscription{}, fmt.Errorf("creem: cancel mode %q: %w", req.Mode, ErrInvalidRequest)
	}
	var out creemSubscription
	if err := g.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return Subscription{}, fmt.Errorf("creem: cancel subscription: %w", err)
	}
	return out.normalise(), nil
}

// Refund refunds a transaction in full: Creem's API has no partial refunds.
// The refund's ID comes later, in the refund.created webhook.
func (g *creemGateway) Refund(ctx context.Context, req RefundRequest) (Refund, error) {
	if req.ExternalPaymentID == "" || req.AmountMinor <= 0 {
		return Refund{}, fmt.Errorf("creem: refund needs a payment and an amount: %w", ErrInvalidRequest)
	}
	var out struct {
		Status string `json:"status"`
	}
	body := map[string]string{"transaction_id": req.ExternalPaymentID}
	if err := g.do(ctx, http.MethodPost, "/v1/refunds", body, &out); err != nil {
		return Refund{}, fmt.Errorf("creem: refund: %w", err)
	}
	return Refund{
		ExternalPaymentID: req.ExternalPaymentID, AmountMinor: req.AmountMinor,
		Currency: strings.ToUpper(req.Currency), Status: creemRefundStatus(out.Status),
	}, nil
}

// creemDiscountRequest is the body of POST /v1/discounts.
type creemDiscountRequest struct {
	Name              string   `json:"name"`
	Code              string   `json:"code,omitempty"`
	Type              string   `json:"type"`
	Percentage        float64  `json:"percentage,omitempty"`
	Amount            int64    `json:"amount,omitempty"`
	Currency          string   `json:"currency,omitempty"`
	Duration          string   `json:"duration"`
	DurationInMonths  int      `json:"duration_in_months,omitempty"`
	AppliesToProducts []string `json:"applies_to_products"`
}

// CancelSchedule implements Gateway: Creem runs no installment schedules.
func (g *creemGateway) CancelSchedule(context.Context, string) error {
	return fmt.Errorf("creem: installment schedules: %w", ErrUnsupported)
}

// CreateCoupon mirrors a Relay discount as a Creem discount code. Creem needs
// the products a code applies to: req.ExternalProductIDs, or else the store's
// active recurring products, so the code never discounts a one-time checkout twice.
// ExternalCouponID is the code (what a checkout's discount_code takes) and
// ExternalPromotionCodeID is Creem's discount ID.
func (g *creemGateway) CreateCoupon(ctx context.Context, req CouponRequest) (Coupon, error) {
	if (req.PercentBps > 0) == (req.AmountOffMinor > 0) {
		return Coupon{}, fmt.Errorf("creem: coupon needs a percent or an amount: %w", ErrInvalidRequest)
	}
	body := creemDiscountRequest{Name: req.Name, Code: req.Code, Duration: req.Duration}
	if body.Name == "" {
		body.Name = req.Code
	}
	if req.PercentBps > 0 {
		body.Type, body.Percentage = "percentage", float64(req.PercentBps)/100
	} else {
		if req.Currency == "" {
			return Coupon{}, fmt.Errorf("creem: an amount-off coupon needs a currency: %w", ErrInvalidRequest)
		}
		body.Type, body.Amount, body.Currency = "fixed", req.AmountOffMinor, strings.ToUpper(req.Currency)
	}
	switch req.Duration {
	case "once", "forever":
	case "repeating":
		if req.DurationMonths <= 0 {
			return Coupon{}, fmt.Errorf("creem: a repeating coupon needs months: %w", ErrInvalidRequest)
		}
		body.DurationInMonths = req.DurationMonths
	default:
		return Coupon{}, fmt.Errorf("creem: coupon duration %q: %w", req.Duration, ErrInvalidRequest)
	}
	products := req.ExternalProductIDs
	if len(products) == 0 {
		var err error
		if products, err = g.recurringProducts(ctx); err != nil {
			return Coupon{}, fmt.Errorf("creem: create coupon: %w", err)
		}
	}
	if len(products) == 0 {
		return Coupon{}, fmt.Errorf("creem: create coupon: the store has no active recurring products: %w", ErrInvalidRequest)
	}
	body.AppliesToProducts = products

	var out struct {
		ID   string `json:"id"`
		Code string `json:"code"`
	}
	if err := g.do(ctx, http.MethodPost, "/v1/discounts", body, &out); err != nil {
		return Coupon{}, fmt.Errorf("creem: create coupon: %w", err)
	}
	return Coupon{ExternalCouponID: out.Code, ExternalPromotionCodeID: out.ID}, nil
}

// recurringProducts lists the IDs of the store's active recurring products.
func (g *creemGateway) recurringProducts(ctx context.Context) ([]string, error) {
	var ids []string
	for page := 1; page <= creemMaxProductPages; page++ {
		var out struct {
			Items []struct {
				ID          string `json:"id"`
				BillingType string `json:"billing_type"`
				Status      string `json:"status"`
			} `json:"items"`
			Pagination struct {
				NextPage *int `json:"next_page"`
			} `json:"pagination"`
		}
		q := url.Values{"page_number": {strconv.Itoa(page)}, "page_size": {"100"}, "status": {"active"}}
		if err := g.do(ctx, http.MethodGet, "/v1/products/search?"+q.Encode(), nil, &out); err != nil {
			return nil, fmt.Errorf("list products: %w", err)
		}
		for _, p := range out.Items {
			if p.BillingType == "recurring" && p.Status != "archived" {
				ids = append(ids, p.ID)
			}
		}
		if out.Pagination.NextPage == nil || len(out.Items) == 0 {
			return ids, nil
		}
	}
	return ids, nil
}

// do sends one API call and decodes a 2xx answer into out. Creem's error
// messages aren't returned: they may echo customer data.
func (g *creemGateway) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("x-api-key", g.apiKey)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w: %w", method, path, ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, creemMaxBody))
	if err != nil {
		return fmt.Errorf("%s %s: read response: %w: %w", method, path, ErrUnavailable, err)
	}
	if resp.StatusCode/100 != 2 {
		var e struct {
			TraceID string `json:"trace_id"`
		}
		_ = json.Unmarshal(data, &e)
		return fmt.Errorf("%s %s: status %d (trace %s): %w", method, path, resp.StatusCode, e.TraceID, creemStatusError(resp.StatusCode))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%s %s: decode response: %w", method, path, ErrUnavailable)
	}
	return nil
}

func creemStatusError(status int) error {
	switch {
	case status == http.StatusNotFound:
		return ErrNotFound
	case status == http.StatusPaymentRequired || status == http.StatusConflict:
		return ErrDeclined
	case status == http.StatusTooManyRequests || status >= 500:
		return ErrUnavailable
	default:
		// 400 and 422 are bad input; 401 and 403 a bad key, which a retry won't fix either.
		return ErrInvalidRequest
	}
}

// creemEnvelope is every webhook delivery. created_at is epoch milliseconds.
type creemEnvelope struct {
	ID        string          `json:"id"`
	EventType string          `json:"eventType"`
	CreatedAt int64           `json:"created_at"`
	Object    json.RawMessage `json:"object"`
}

func (g *creemGateway) ParseWebhook(_ context.Context, header http.Header, body []byte) (Event, error) {
	if !g.verify(header.Get(creemSignatureHeader), body) {
		return Event{}, fmt.Errorf("creem: %w", ErrInvalidSignature)
	}
	var env creemEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return Event{}, fmt.Errorf("creem: decode event: %w", ErrInvalidRequest)
	}
	if env.ID == "" || env.EventType == "" {
		return Event{}, fmt.Errorf("creem: event needs an id and an eventType: %w", ErrInvalidRequest)
	}
	ev := Event{
		Provider: Creem, ExternalID: env.ID, Type: EventIgnored, ProviderType: env.EventType,
		Payload: json.RawMessage(slices.Clone(body)),
	}
	if t := creemMillis(env.CreatedAt); t != nil {
		ev.OccurredAt = *t
	}
	if err := fillCreemEvent(&ev, env); err != nil {
		return Event{}, fmt.Errorf("creem: decode %s: %w", env.EventType, ErrInvalidRequest)
	}
	return ev, nil
}

func (g *creemGateway) verify(sig string, body []byte) bool {
	want, err := hex.DecodeString(strings.TrimSpace(sig))
	if err != nil || len(want) == 0 {
		return false
	}
	h := hmac.New(sha256.New, []byte(g.webhookSecret))
	h.Write(body)
	return hmac.Equal(want, h.Sum(nil))
}

// fillCreemEvent sets ev's type and objects from Creem's payload.
func fillCreemEvent(ev *Event, env creemEnvelope) error {
	switch t := env.EventType; {
	case t == "checkout.completed":
		var c creemCheckout
		if err := json.Unmarshal(env.Object, &c); err != nil {
			return err //nolint:wrapcheck // the caller wraps it
		}
		ev.Type = EventCheckoutCompleted
		ev.Reference = creemReference(c.Metadata, c.RequestID)
		sub, err := decodeCreemSubscription(c.Subscription)
		if err != nil {
			return err
		}
		if c.Order != nil {
			p := c.Order.payment()
			if p.ExternalCustomerID == "" {
				p.ExternalCustomerID = c.Customer.ID
			}
			if sub != nil {
				p.ExternalSubscriptionID = sub.ID
			}
			paid := ev.OccurredAt
			p.PaidAt = &paid
			ev.Payment = &p
		}
		if sub != nil && sub.Status != "" {
			s := sub.normalise()
			ev.Subscription = &s
			if ev.Payment != nil {
				ev.Payment.PeriodStart, ev.Payment.PeriodEnd = s.CurrentPeriodStart, s.CurrentPeriodEnd
			}
		}
	case strings.HasPrefix(t, "subscription."):
		var s creemSubscription
		if err := json.Unmarshal(env.Object, &s); err != nil {
			return err //nolint:wrapcheck // the caller wraps it
		}
		ev.Reference = creemReference(s.Metadata, "")
		sub := s.normalise()
		ev.Subscription = &sub
		switch t {
		case "subscription.paid":
			ev.Type = EventPaymentSucceeded
			if s.LastTransaction != nil {
				p := s.LastTransaction.payment()
				p.ExternalCustomerID, p.ExternalSubscriptionID = sub.ExternalCustomerID, sub.ExternalID
				if p.PaidAt == nil {
					paid := ev.OccurredAt
					p.PaidAt = &paid
				}
				ev.Payment = &p
			}
		case "subscription.canceled":
			ev.Type = EventSubscriptionCanceled
		default:
			ev.Type = EventSubscriptionUpdated
		}
	case t == "refund.created":
		var r struct {
			ID             string            `json:"id"`
			Status         string            `json:"status"`
			RefundAmount   int64             `json:"refund_amount"`
			RefundCurrency string            `json:"refund_currency"`
			Transaction    creemTransaction  `json:"transaction"`
			Checkout       *creemCheckout    `json:"checkout"`
			Subscription   creemSubscription `json:"subscription"`
		}
		if err := json.Unmarshal(env.Object, &r); err != nil {
			return err //nolint:wrapcheck // the caller wraps it
		}
		ev.Type = EventRefundUpdated
		ev.Refund = &Refund{
			ExternalID: r.ID, ExternalPaymentID: r.Transaction.ID, AmountMinor: r.RefundAmount,
			Currency: strings.ToUpper(r.RefundCurrency), Status: creemRefundStatus(r.Status),
		}
		ev.Reference = creemObjectReference(r.Checkout, r.Subscription.Metadata)
	case t == "dispute.created":
		var d struct {
			Transaction  creemTransaction  `json:"transaction"`
			Checkout     *creemCheckout    `json:"checkout"`
			Subscription creemSubscription `json:"subscription"`
			Customer     creemRef          `json:"customer"`
		}
		if err := json.Unmarshal(env.Object, &d); err != nil {
			return err //nolint:wrapcheck // the caller wraps it
		}
		ev.Type = EventDisputeOpened
		p := d.Transaction.payment()
		p.ExternalCustomerID, p.ExternalSubscriptionID, p.Status = d.Customer.ID, d.Subscription.ID, "open"
		p.PaidAt = nil
		ev.Payment = &p
		ev.Reference = creemObjectReference(d.Checkout, d.Subscription.Metadata)
	}
	return nil
}

// creemReference reads our checkout_sessions.id from metadata, else from the
// checkout's request_id. Anything that isn't a UUID is not ours.
func creemReference(metadata map[string]string, requestID string) *uuid.UUID {
	for _, s := range []string{metadata[creemReferenceKey], requestID} {
		if id, err := uuid.Parse(s); err == nil && id != uuid.Nil {
			return &id
		}
	}
	return nil
}

func creemObjectReference(c *creemCheckout, subMetadata map[string]string) *uuid.UUID {
	if c != nil {
		if ref := creemReference(c.Metadata, c.RequestID); ref != nil {
			return ref
		}
	}
	return creemReference(subMetadata, "")
}

// creemRef is a field Creem sends either as an ID or as the expanded object.
type creemRef struct{ ID string }

func (r *creemRef) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '"' {
		return json.Unmarshal(b, &r.ID) //nolint:wrapcheck // decoding a field; json.Unmarshal reports it
	}
	var o struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return err //nolint:wrapcheck // decoding a field; json.Unmarshal reports it
	}
	r.ID = o.ID
	return nil
}

type creemOrder struct {
	ID             string   `json:"id"`
	Customer       creemRef `json:"customer"`
	Transaction    creemRef `json:"transaction"`
	SubTotal       int64    `json:"sub_total"`
	TaxAmount      int64    `json:"tax_amount"`
	DiscountAmount int64    `json:"discount_amount"`
	AmountPaid     int64    `json:"amount_paid"`
	Currency       string   `json:"currency"`
}

// payment keeps TotalMinor = SubtotalMinor - DiscountMinor + TaxMinor whether
// the product's tax is inclusive or exclusive.
func (o creemOrder) payment() Payment {
	id := o.Transaction.ID
	if id == "" {
		id = o.ID
	}
	return Payment{
		ExternalID: id, ExternalCustomerID: o.Customer.ID,
		SubtotalMinor: o.AmountPaid + o.DiscountAmount - o.TaxAmount, DiscountMinor: o.DiscountAmount,
		TaxMinor: o.TaxAmount, TotalMinor: o.AmountPaid, Currency: strings.ToUpper(o.Currency),
	}
}

type creemTransaction struct {
	ID             string `json:"id"`
	AmountPaid     int64  `json:"amount_paid"`
	TaxAmount      int64  `json:"tax_amount"`
	DiscountAmount int64  `json:"discount_amount"`
	Currency       string `json:"currency"`
	Status         string `json:"status"`
	PeriodStart    int64  `json:"period_start"`
	PeriodEnd      int64  `json:"period_end"`
	CreatedAt      int64  `json:"created_at"`
}

func (t creemTransaction) payment() Payment {
	p := Payment{
		ExternalID:    t.ID,
		SubtotalMinor: t.AmountPaid + t.DiscountAmount - t.TaxAmount, DiscountMinor: t.DiscountAmount,
		TaxMinor: t.TaxAmount, TotalMinor: t.AmountPaid, Currency: strings.ToUpper(t.Currency),
		PeriodStart: creemMillis(t.PeriodStart), PeriodEnd: creemMillis(t.PeriodEnd),
	}
	if t.Status == "paid" {
		p.PaidAt = creemMillis(t.CreatedAt)
	}
	return p
}

type creemSubscription struct {
	ID       string   `json:"id"`
	Product  creemRef `json:"product"`
	Customer creemRef `json:"customer"`
	Items    []struct {
		ProductID string `json:"product_id"`
	} `json:"items"`
	Status             string            `json:"status"`
	LastTransaction    *creemTransaction `json:"last_transaction"`
	CurrentPeriodStart *time.Time        `json:"current_period_start_date"`
	CurrentPeriodEnd   *time.Time        `json:"current_period_end_date"`
	CanceledAt         *time.Time        `json:"canceled_at"`
	Metadata           map[string]string `json:"metadata"`
}

// decodeCreemSubscription reads a subscription sent as an ID or an object.
func decodeCreemSubscription(raw json.RawMessage) (*creemSubscription, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if raw[0] == '"' {
		var id string
		if err := json.Unmarshal(raw, &id); err != nil {
			return nil, err //nolint:wrapcheck // the caller wraps it
		}
		return &creemSubscription{ID: id}, nil
	}
	var s creemSubscription
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err //nolint:wrapcheck // the caller wraps it
	}
	return &s, nil
}

func (s creemSubscription) normalise() Subscription {
	out := Subscription{
		ExternalID: s.ID, ExternalCustomerID: s.Customer.ID, ExternalPriceID: s.Product.ID,
		ProviderStatus: s.Status, CurrentPeriodStart: utcPtr(s.CurrentPeriodStart),
		CurrentPeriodEnd: utcPtr(s.CurrentPeriodEnd), CanceledAt: utcPtr(s.CanceledAt),
	}
	if out.ExternalPriceID == "" && len(s.Items) > 0 {
		out.ExternalPriceID = s.Items[0].ProductID
	}
	switch s.Status {
	case "active":
		out.Status = SubscriptionActive
	case "scheduled_cancel":
		out.Status, out.CancelAtPeriodEnd = SubscriptionActive, true
	case "trialing":
		out.Status = SubscriptionTrialing
		out.TrialEnd = out.CurrentPeriodEnd
	case "past_due", "unpaid", "paused":
		out.Status = SubscriptionPastDue
	case "canceled", "expired":
		out.Status = SubscriptionCanceled
	default:
		out.Status = SubscriptionIncomplete
	}
	return out
}

func creemRefundStatus(s string) RefundStatus {
	switch s {
	case "succeeded":
		return RefundSucceeded
	case "failed", "canceled":
		return RefundFailed
	default:
		// pending and requiresAction are still in progress.
		return RefundPending
	}
}

func creemMillis(ms int64) *time.Time {
	if ms == 0 {
		return nil
	}
	t := time.UnixMilli(ms).UTC()
	return &t
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
