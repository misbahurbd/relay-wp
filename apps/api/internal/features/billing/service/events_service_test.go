package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database/dbtest"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// payloadMarker sits in every webhook body the tests send, in a field the
// normalised event doesn't have, so finding it in job args or log output means
// the raw payload leaked.
const payloadMarker = "payload-marker-7f3a"

const fakeSecret = "whsec_test"

// recordingQueue is an Enqueuer that keeps the job args it was given.
type recordingQueue struct {
	mu   sync.Mutex
	jobs []river.JobArgs
}

func (q *recordingQueue) InsertTx(_ context.Context, _ pgx.Tx, args river.JobArgs, _ *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = append(q.jobs, args)
	return &rivertype.JobInsertResult{Job: &rivertype.JobRow{}}, nil
}

func (q *recordingQueue) all() []river.JobArgs {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]river.JobArgs(nil), q.jobs...)
}

// syncBuffer is a bytes.Buffer safe for concurrent log writes.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

type eventsEnv struct {
	t     *testing.T
	ctx   context.Context
	pool  *pgxpool.Pool
	svc   *Service
	fake  *payment.Fake
	queue *recordingQueue
	logs  *syncBuffer
	repo  repository.Repository
	run   string
}

// id makes a provider ID unique to this test, as the database is shared.
func (e *eventsEnv) id(name string) string { return name + "_" + e.run }

func newEventsEnv(t *testing.T) *eventsEnv {
	t.Helper()
	pool := dbtest.New(t)
	fake := payment.NewFake(payment.Stripe, fakeSecret)
	reg, err := payment.NewRegistry(fake)
	if err != nil {
		t.Fatal(err)
	}
	logs := &syncBuffer{}
	log := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	q := &recordingQueue{}
	return &eventsEnv{
		t: t, ctx: context.Background(), pool: pool, fake: fake, queue: q, logs: logs,
		svc: New(pool, reg, q, Config{}, log), repo: repository.NewRepository(pool), run: uuid.NewString()[:8],
	}
}

// webhook is ev as the fake provider delivers it, with payloadMarker added to
// the raw body (and signed over).
func (e *eventsEnv) webhook(ev payment.Event) (http.Header, []byte) {
	e.t.Helper()
	if ev.OccurredAt.IsZero() {
		ev.OccurredAt = time.Now().UTC()
	}
	raw, err := json.Marshal(ev)
	if err != nil {
		e.t.Fatal(err)
	}
	body := append([]byte(`{"raw_only":"`+payloadMarker+`",`), raw[1:]...)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(fakeSecret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	header := http.Header{}
	header.Set(payment.FakeSignatureHeader, "t="+ts+",v1="+hex.EncodeToString(mac.Sum(nil)))
	return header, body
}

// deliver passes ev to ReceiveWebhook as the fake provider would.
func (e *eventsEnv) deliver(ev payment.Event) error {
	e.t.Helper()
	header, body := e.webhook(ev)
	return e.svc.ReceiveWebhook(e.ctx, "stripe", header, body)
}

// receive delivers ev and returns the job it enqueued.
func (e *eventsEnv) receive(ev payment.Event) model.ProcessPaymentEventArgs {
	e.t.Helper()
	before := len(e.queue.all())
	if err := e.deliver(ev); err != nil {
		e.t.Fatalf("receive webhook: %v", err)
	}
	jobs := e.queue.all()
	if len(jobs) != before+1 {
		e.t.Fatalf("enqueued %d jobs, want 1", len(jobs)-before)
	}
	return jobs[len(jobs)-1].(model.ProcessPaymentEventArgs)
}

// process runs the job as the worker would: through the job's JSON.
func (e *eventsEnv) process(args model.ProcessPaymentEventArgs) error {
	e.t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		e.t.Fatal(err)
	}
	var decoded model.ProcessPaymentEventArgs
	if err := json.Unmarshal(raw, &decoded); err != nil {
		e.t.Fatal(err)
	}
	return e.svc.ProcessPaymentEvent(e.ctx, decoded.PaymentEventID, decoded.Event)
}

func (e *eventsEnv) mustProcess(args model.ProcessPaymentEventArgs) {
	e.t.Helper()
	if err := e.process(args); err != nil {
		e.t.Fatalf("process payment event: %v", err)
	}
}

type storedEvent struct {
	count       int
	workspaceID *uuid.UUID
	processed   bool
	hasPayload  bool
}

func (e *eventsEnv) stored(externalID string) storedEvent {
	e.t.Helper()
	var s storedEvent
	err := e.pool.QueryRow(e.ctx, `SELECT count(*), max(workspace_id::text)::uuid, COALESCE(bool_or(processed_at IS NOT NULL), false),
		COALESCE(bool_or(payload IS NOT NULL), false) FROM payment_events WHERE provider = 'stripe' AND external_event_id = $1`,
		externalID).Scan(&s.count, &s.workspaceID, &s.processed, &s.hasPayload)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		e.t.Fatal(err)
	}
	return s
}

func (e *eventsEnv) workspace() uuid.UUID {
	e.t.Helper()
	ws := uuid.New()
	if _, err := e.pool.Exec(e.ctx, `INSERT INTO workspaces (id, name) VALUES ($1, 'Events test')`, ws); err != nil {
		e.t.Fatal(err)
	}
	return ws
}

// price seeds a plan with a one-time price.
func (e *eventsEnv) price() uuid.UUID {
	e.t.Helper()
	planID, err := e.repo.CreatePlan(e.ctx, gen.CreatePlanParams{
		Code: "t5_" + uuid.NewString()[:8], Version: 1, Name: "T5", Limits: []byte(`{}`),
	})
	if err != nil {
		e.t.Fatal(err)
	}
	priceID, err := e.repo.CreatePlanPrice(e.ctx, gen.CreatePlanPriceParams{
		PlanID: planID, BillingType: "one_time", Currency: "USD", AmountMinor: 30000,
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return priceID
}

// invoice seeds a paid order with one paid invoice of totalMinor and returns
// the invoice's ID and provider payment ID.
func (e *eventsEnv) invoice(ws uuid.UUID, totalMinor int64) (uuid.UUID, string) {
	e.t.Helper()
	customer, err := e.repo.EnsureBillingCustomer(e.ctx, gen.EnsureBillingCustomerParams{WorkspaceID: ws, Provider: "stripe"})
	if err != nil {
		e.t.Fatal(err)
	}
	orderID, err := e.repo.CreateOrder(e.ctx, gen.CreateOrderParams{
		WorkspaceID: ws, PlanPriceID: e.price(), BillingCustomerID: customer.ID, Provider: "stripe",
		Status: "pending", TotalMinor: totalMinor, Currency: "USD",
	})
	if err != nil {
		e.t.Fatal(err)
	}
	payID := "pay_" + uuid.NewString()
	now := time.Now()
	id, err := e.repo.CreateInvoice(e.ctx, gen.CreateInvoiceParams{
		WorkspaceID: ws, OrderID: &orderID, Provider: "stripe", ExternalPaymentID: &payID, Status: model.InvoicePaid,
		SubtotalMinor: totalMinor, TotalMinor: totalMinor, Currency: "USD", BillTo: []byte(`{}`), PaidAt: &now,
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return id, payID
}

func (e *eventsEnv) invoiceStatus(id uuid.UUID) string {
	e.t.Helper()
	var s string
	if err := e.pool.QueryRow(e.ctx, `SELECT status FROM invoices WHERE id = $1`, id).Scan(&s); err != nil {
		e.t.Fatal(err)
	}
	return s
}

func (e *eventsEnv) refunds(invoiceID uuid.UUID) (n int, sum int64) {
	e.t.Helper()
	err := e.pool.QueryRow(e.ctx, `SELECT count(*), COALESCE(sum(amount_minor), 0)::bigint FROM refunds
		WHERE invoice_id = $1 AND status <> 'failed'`, invoiceID).Scan(&n, &sum)
	if err != nil {
		e.t.Fatal(err)
	}
	return n, sum
}

func refundEvent(payID, refundID string, amount int64, status payment.RefundStatus) payment.Event {
	return payment.Event{
		ExternalID: "evt_" + uuid.NewString(), Type: payment.EventRefundUpdated, ProviderType: "charge.refund.updated",
		Refund:     &payment.Refund{ExternalID: refundID, ExternalPaymentID: payID, AmountMinor: amount, Currency: "USD", Status: status},
		OccurredAt: time.Now().UTC(),
	}
}

func TestReceiveWebhookStoresAndEnqueues(t *testing.T) {
	e := newEventsEnv(t)
	ev := refundEvent("pay_x", e.id("re_x"), 100, payment.RefundSucceeded)
	args := e.receive(ev)

	got := e.stored(ev.ExternalID)
	if got.count != 1 || !got.hasPayload || got.processed {
		t.Fatalf("stored = %+v, want one unprocessed row with its payload", got)
	}
	if args.PaymentEventID == uuid.Nil || args.Event.ExternalID != ev.ExternalID || args.Event.Provider != payment.Stripe {
		t.Errorf("job args = %+v", args)
	}
	var stored string
	if err := e.pool.QueryRow(e.ctx, `SELECT payload::text FROM payment_events WHERE id = $1`,
		args.PaymentEventID).Scan(&stored); err != nil || !strings.Contains(stored, payloadMarker) {
		t.Fatalf("stored payload lacks the raw body: %v", err)
	}
	raw, _ := json.Marshal(args)
	if args.Event.Payload != nil || strings.Contains(string(raw), payloadMarker) {
		t.Errorf("job args carry the payload: %s", raw)
	}
}

func TestReceiveWebhookReplayIsANoOp(t *testing.T) {
	e := newEventsEnv(t)
	header, body := e.webhook(refundEvent("pay_x", e.id("re_x"), 100, payment.RefundSucceeded))
	for range 2 {
		if err := e.svc.ReceiveWebhook(e.ctx, "stripe", header, body); err != nil {
			t.Fatalf("receive: %v", err)
		}
	}
	var ev payment.Event
	_ = json.Unmarshal(body, &ev)
	if got := e.stored(ev.ExternalID); got.count != 1 {
		t.Errorf("stored %d rows, want 1", got.count)
	}
	if n := len(e.queue.all()); n != 1 {
		t.Errorf("enqueued %d jobs, want 1", n)
	}
}

func TestReceiveWebhookBadSignatureStoresNothing(t *testing.T) {
	e := newEventsEnv(t)
	ev := refundEvent("pay_x", e.id("re_x"), 100, payment.RefundSucceeded)
	header, body := e.webhook(ev)
	var decoded payment.Event
	_ = json.Unmarshal(body, &decoded)

	tampered := bytes.Replace(body, []byte(`"amount_minor":100`), []byte(`"amount_minor":999`), 1)
	unsigned := header.Clone()
	unsigned.Del(payment.FakeSignatureHeader)
	for name, tc := range map[string]struct {
		header map[string][]string
		body   []byte
	}{
		"tampered body": {header, tampered},
		"no signature":  {unsigned, body},
	} {
		if err := e.svc.ReceiveWebhook(e.ctx, "stripe", tc.header, tc.body); !errors.Is(err, model.ErrWebhookSignature) {
			t.Errorf("%s: err = %v, want ErrWebhookSignature", name, err)
		}
	}
	if got := e.stored(decoded.ExternalID); got.count != 0 {
		t.Errorf("stored %d rows, want none", got.count)
	}
	if n := len(e.queue.all()); n != 0 {
		t.Errorf("enqueued %d jobs, want none", n)
	}
}

func TestReceiveWebhookUnknownProvider(t *testing.T) {
	e := newEventsEnv(t)
	header, body := e.webhook(refundEvent("pay_x", e.id("re_x"), 100, payment.RefundSucceeded))
	for _, p := range []string{"creem", "paypal", "nope"} {
		if err := e.svc.ReceiveWebhook(e.ctx, p, header, body); !errors.Is(err, model.ErrProviderNotAvailable) {
			t.Errorf("%s: err = %v, want ErrProviderNotAvailable", p, err)
		}
	}
}

func TestIgnoredEventIsProcessedWithoutAJob(t *testing.T) {
	e := newEventsEnv(t)
	ev := payment.Event{ExternalID: "evt_" + uuid.NewString(), Type: payment.EventIgnored, ProviderType: "customer.created"}
	if err := e.deliver(ev); err != nil {
		t.Fatal(err)
	}
	if got := e.stored(ev.ExternalID); got.count != 1 || !got.processed {
		t.Errorf("stored = %+v, want one processed row", got)
	}
	if n := len(e.queue.all()); n != 0 {
		t.Errorf("enqueued %d jobs, want none", n)
	}
}

func TestRefundsUpdateInvoiceAndNeverExceedTotal(t *testing.T) {
	e := newEventsEnv(t)
	ws := e.workspace()
	invoiceID, payID := e.invoice(ws, 1000)

	first := e.receive(refundEvent(payID, e.id("re_1"), 400, payment.RefundSucceeded))
	e.mustProcess(first)
	if got := e.stored(first.Event.ExternalID); !got.processed || got.workspaceID == nil || *got.workspaceID != ws {
		t.Fatalf("event after processing = %+v, want processed in workspace %s", got, ws)
	}
	if n, sum := e.refunds(invoiceID); n != 1 || sum != 400 || e.invoiceStatus(invoiceID) != model.InvoicePartiallyRefunded {
		t.Fatalf("after first refund: %d refunds, %d refunded, invoice %s", n, sum, e.invoiceStatus(invoiceID))
	}

	// Running the same job again (River retry, duplicate job) changes nothing.
	e.mustProcess(first)
	if n, sum := e.refunds(invoiceID); n != 1 || sum != 400 {
		t.Fatalf("replayed job: %d refunds, %d refunded; want 1, 400", n, sum)
	}

	// 400 + 700 is more than the total: not recorded.
	e.mustProcess(e.receive(refundEvent(payID, e.id("re_2"), 700, payment.RefundSucceeded)))
	if n, sum := e.refunds(invoiceID); n != 1 || sum != 400 || e.invoiceStatus(invoiceID) != model.InvoicePartiallyRefunded {
		t.Fatalf("over-refund: %d refunds, %d refunded, invoice %s", n, sum, e.invoiceStatus(invoiceID))
	}

	// The rest refunds the invoice in full.
	e.mustProcess(e.receive(refundEvent(payID, e.id("re_3"), 600, payment.RefundPending)))
	if n, sum := e.refunds(invoiceID); n != 2 || sum != 1000 || e.invoiceStatus(invoiceID) != model.InvoiceRefunded {
		t.Fatalf("full refund: %d refunds, %d refunded, invoice %s", n, sum, e.invoiceStatus(invoiceID))
	}

	// That refund failing puts the invoice back to partially refunded.
	e.mustProcess(e.receive(refundEvent(payID, e.id("re_3"), 600, payment.RefundFailed)))
	if n, sum := e.refunds(invoiceID); n != 1 || sum != 400 || e.invoiceStatus(invoiceID) != model.InvoicePartiallyRefunded {
		t.Fatalf("failed refund: %d refunds, %d refunded, invoice %s", n, sum, e.invoiceStatus(invoiceID))
	}
}

func TestSameEventTwiceChangesNothing(t *testing.T) {
	e := newEventsEnv(t)
	ws := e.workspace()
	invoiceID, payID := e.invoice(ws, 1000)

	header, body := e.webhook(refundEvent(payID, e.id("re_1"), 300, payment.RefundSucceeded))
	for range 2 {
		if err := e.svc.ReceiveWebhook(e.ctx, "stripe", header, body); err != nil {
			t.Fatal(err)
		}
		for _, j := range e.queue.all() {
			e.mustProcess(j.(model.ProcessPaymentEventArgs))
		}
	}
	if n := len(e.queue.all()); n != 1 {
		t.Errorf("enqueued %d jobs, want 1", n)
	}
	if n, sum := e.refunds(invoiceID); n != 1 || sum != 300 || e.invoiceStatus(invoiceID) != model.InvoicePartiallyRefunded {
		t.Errorf("%d refunds, %d refunded, invoice %s; want 1, 300, partially_refunded", n, sum, e.invoiceStatus(invoiceID))
	}
}

func TestConcurrentProcessingAppliesOnce(t *testing.T) {
	e := newEventsEnv(t)
	ws := e.workspace()
	invoiceID, payID := e.invoice(ws, 1000)
	args := e.receive(refundEvent(payID, e.id("re_1"), 250, payment.RefundSucceeded))

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if err := e.process(args); err != nil {
				t.Errorf("process: %v", err)
			}
		})
	}
	wg.Wait()
	if n, sum := e.refunds(invoiceID); n != 1 || sum != 250 {
		t.Errorf("%d refunds, %d refunded; want 1, 250", n, sum)
	}
}

func TestConcurrentRefundsNeverExceedTotal(t *testing.T) {
	e := newEventsEnv(t)
	ws := e.workspace()
	invoiceID, payID := e.invoice(ws, 1000)
	var jobs []model.ProcessPaymentEventArgs
	for i := range 6 {
		jobs = append(jobs, e.receive(refundEvent(payID, e.id("re_"+string(rune('a'+i))), 300, payment.RefundSucceeded)))
	}
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Go(func() {
			if err := e.process(j); err != nil {
				t.Errorf("process: %v", err)
			}
		})
	}
	wg.Wait()
	if n, sum := e.refunds(invoiceID); n != 3 || sum != 900 {
		t.Errorf("%d refunds, %d refunded; want 3, 900", n, sum)
	}
}

func TestDisputes(t *testing.T) {
	e := newEventsEnv(t)
	ws := e.workspace()
	dispute := func(typ payment.EventType, payID, outcome string) model.ProcessPaymentEventArgs {
		return e.receive(payment.Event{
			ExternalID: "evt_" + uuid.NewString(), Type: typ, ProviderType: string(typ), OccurredAt: time.Now().UTC(),
			Payment: &payment.Payment{ExternalID: payID, Status: outcome},
		})
	}

	won, wonPay := e.invoice(ws, 500)
	e.mustProcess(dispute(payment.EventDisputeOpened, wonPay, ""))
	if s := e.invoiceStatus(won); s != model.InvoiceDisputed {
		t.Fatalf("opened: invoice %s, want disputed", s)
	}
	e.mustProcess(dispute(payment.EventDisputeClosed, wonPay, "won"))
	if s := e.invoiceStatus(won); s != model.InvoicePaid {
		t.Errorf("won: invoice %s, want paid", s)
	}

	lost, lostPay := e.invoice(ws, 500)
	// The lost invoice paid for the workspace's lifetime grant.
	var orderID, planID uuid.UUID
	if err := e.pool.QueryRow(e.ctx, `SELECT o.id, p.plan_id FROM invoices i JOIN orders o ON o.id = i.order_id
		JOIN plan_prices p ON p.id = o.plan_price_id WHERE i.id = $1`, lost).Scan(&orderID, &planID); err != nil {
		t.Fatal(err)
	}
	grantID, err := e.repo.CreateGrant(e.ctx, gen.CreateGrantParams{
		WorkspaceID: ws, PlanID: planID, Source: model.GrantFromOrder, OrderID: &orderID,
		StartsAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	e.mustProcess(dispute(payment.EventDisputeOpened, lostPay, ""))
	e.mustProcess(dispute(payment.EventDisputeClosed, lostPay, "lost"))
	if s := e.invoiceStatus(lost); s != model.InvoiceDisputed {
		t.Errorf("lost: invoice %s, want disputed", s)
	}
	var status string
	if err := e.pool.QueryRow(e.ctx, `SELECT status FROM plan_grants WHERE id = $1`, grantID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != model.GrantSuspended {
		t.Errorf("lost: grant %s, want suspended", status)
	}
}

func TestUnmatchedEventRetriesThenGivesUp(t *testing.T) {
	e := newEventsEnv(t)
	recent := refundEvent("pay_nobody_"+uuid.NewString(), e.id("re_n"), 100, payment.RefundSucceeded)
	args := e.receive(recent)
	if err := e.process(args); err == nil {
		t.Fatal("a recent event that matches nothing was not retried")
	}
	if got := e.stored(recent.ExternalID); got.processed {
		t.Fatal("a retried event was marked processed")
	}

	old := refundEvent("pay_nobody_"+uuid.NewString(), e.id("re_o"), 100, payment.RefundSucceeded)
	args = e.receive(old)
	args.Event.OccurredAt = time.Now().Add(-2 * unmatchedRetryWindow)
	e.mustProcess(args)
	if got := e.stored(old.ExternalID); !got.processed || got.workspaceID != nil {
		t.Errorf("old unmatched event = %+v, want processed with no workspace", got)
	}
}

// A failing handler rolls the whole event back: no workspace, no processed_at,
// so River's retry starts clean. An ltd checkout session with no order fails
// in CompleteCheckout.
func TestHandlerErrorRollsBack(t *testing.T) {
	e := newEventsEnv(t)
	ws := e.workspace()
	session, err := e.repo.CreateCheckoutSession(e.ctx, gen.CreateCheckoutSessionParams{
		WorkspaceID: ws, PlanPriceID: e.price(), Purpose: "ltd", Provider: "stripe", AmountMinor: 30000,
		Currency: "USD", IdempotencyKey: uuid.NewString(), ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	ref := session.ID
	args := e.receive(payment.Event{
		ExternalID: "evt_" + uuid.NewString(), Type: payment.EventCheckoutCompleted, ProviderType: "checkout.session.completed",
		OccurredAt: time.Now().UTC(), Reference: &ref,
		Payment: &payment.Payment{ExternalID: "pay_" + uuid.NewString(), TotalMinor: 30000, Currency: "USD"},
	})
	if err := e.process(args); err == nil {
		t.Fatal("completing an ltd checkout with no order should fail")
	}
	if got := e.stored(args.Event.ExternalID); got.processed || got.workspaceID != nil {
		t.Errorf("after a failed handler: %+v, want unprocessed with no workspace", got)
	}
}

func TestPayloadIsNeverLogged(t *testing.T) {
	e := newEventsEnv(t)
	ws := e.workspace()
	_, payID := e.invoice(ws, 1000)

	// Received, replayed, processed, and a bad signature: every log path.
	header, body := e.webhook(refundEvent(payID, e.id("re_1"), 100, payment.RefundSucceeded))
	for range 2 {
		if err := e.svc.ReceiveWebhook(e.ctx, "stripe", header, body); err != nil {
			t.Fatal(err)
		}
	}
	e.mustProcess(e.queue.all()[0].(model.ProcessPaymentEventArgs))
	_ = e.svc.ReceiveWebhook(e.ctx, "stripe", header, append(body, ' '))
	e.mustProcess(e.receive(refundEvent(payID, e.id("re_2"), 5000, payment.RefundSucceeded))) // over the total: logged as an error

	logs := e.logs.String()
	if !strings.Contains(logs, "webhook received") || !strings.Contains(logs, "payment event applied") {
		t.Fatalf("expected log lines missing:\n%s", logs)
	}
	if strings.Contains(logs, payloadMarker) {
		t.Errorf("the webhook payload reached the logs:\n%s", logs)
	}
}

func TestPurgeEventPayloads(t *testing.T) {
	e := newEventsEnv(t)
	oldEv := refundEvent("pay_x", e.id("re_x"), 100, payment.RefundSucceeded)
	newEv := refundEvent("pay_y", e.id("re_y"), 100, payment.RefundSucceeded)
	e.receive(oldEv)
	e.receive(newEv)
	if _, err := e.pool.Exec(e.ctx, `UPDATE payment_events SET received_at = now() - interval '91 days'
		WHERE provider = 'stripe' AND external_event_id = $1`, oldEv.ExternalID); err != nil {
		t.Fatal(err)
	}
	n, err := e.svc.PurgeEventPayloads(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Errorf("purged %d, want at least 1", n)
	}
	if e.stored(oldEv.ExternalID).hasPayload {
		t.Error("the old payload was kept")
	}
	if !e.stored(newEv.ExternalID).hasPayload {
		t.Error("a recent payload was purged")
	}
	if n, err := e.svc.PurgeEventPayloads(e.ctx); err != nil || n != 0 {
		t.Errorf("second purge = %d, %v; want 0", n, err)
	}
}

func TestCheckPlanCacheDrift(t *testing.T) {
	e := newEventsEnv(t)
	// The workspace caches "growth" but has no grant, so it is on free.
	ws := e.workspace()
	if _, err := e.pool.Exec(e.ctx, `UPDATE workspaces SET plan = 'growth' WHERE id = $1`, ws); err != nil {
		t.Fatal(err)
	}
	n, err := e.svc.CheckPlanCacheDrift(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("found %d drifted workspaces, want at least 1", n)
	}
	logs := e.logs.String()
	if !strings.Contains(logs, ws.String()) || !strings.Contains(logs, `"granted_plan":"free"`) {
		t.Errorf("drift not logged for %s:\n%s", ws, logs)
	}
	if strings.Contains(logs, "Events test") {
		t.Error("the workspace name reached the logs")
	}
}
