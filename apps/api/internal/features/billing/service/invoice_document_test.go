package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/mailer"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/storage"
)

// testDocuments are invoice document deps for tests that don't look at them.
func testDocuments(t *testing.T) Documents {
	t.Helper()
	store, err := storage.New(t.Context(), config.Storage{Driver: "memory"}, false)
	if err != nil {
		t.Fatal(err)
	}
	return Documents{Renderer: &fakeRenderer{}, Storage: store, Mailer: &fakeMailer{}}
}

// fakeRenderer returns a tiny PDF and keeps each HTML page it was given.
type fakeRenderer struct {
	mu    sync.Mutex
	pages []string
	err   error
}

func (r *fakeRenderer) Render(_ context.Context, html string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	r.pages = append(r.pages, html)
	return []byte(fmt.Sprintf("%%PDF-1.4 render %d\n%%%%EOF", len(r.pages))), nil
}

type fakeMailer struct {
	mu   sync.Mutex
	sent []mailer.Message
}

func (m *fakeMailer) Send(_ context.Context, msg mailer.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	return nil
}

// docsEnv is a t3Env whose invoice documents go to fakes it can inspect.
type docsEnv struct {
	*t3Env
	renderer *fakeRenderer
	mail     *fakeMailer
	queue    *recordingQueue
	// year is this test's own calendar year: invoice numbers count per year in
	// a database other tests share.
	year int
}

func newDocsEnv(t *testing.T) *docsEnv {
	t.Helper()
	e := &docsEnv{t3Env: newT3Env(t), renderer: &fakeRenderer{}, mail: &fakeMailer{}, queue: &recordingQueue{}}
	store, err := storage.New(t.Context(), config.Storage{Driver: "memory"}, false)
	if err != nil {
		t.Fatal(err)
	}
	e.svc.docs = Documents{Renderer: e.renderer, Storage: store, Mailer: e.mail}
	e.svc.queue = e.queue
	e.year = 3000 + rand.IntN(5000) //nolint:gosec // picks a test year, not a secret
	e.clock = time.Date(e.year, time.March, 1, 9, 0, 0, 0, time.UTC)
	return e
}

// renew records a renewal payment paid at `at` for the period starting then.
func (e *docsEnv) renew(sub model.Subscription, externalID string, at time.Time, paid bool) {
	e.t.Helper()
	end := at.AddDate(0, 1, 0)
	e.inTx(func(tx pgx.Tx) error {
		return e.svc.ApplySubscriptionPayment(e.ctx, tx, e.ws, sub.ID, payment.Payment{
			ExternalID: externalID, SubtotalMinor: 7900, TotalMinor: 7900, Currency: "USD",
			PeriodStart: &at, PeriodEnd: &end, PaidAt: &at,
		}, paid)
	})
}

// numbers is the workspace's invoice numbers, oldest first ("" for none).
func (e *docsEnv) numbers() []string {
	e.t.Helper()
	page, err := e.svc.Invoices(e.ctx, e.ws, "")
	if err != nil {
		e.t.Fatal(err)
	}
	out := make([]string, len(page.Invoices))
	for i, inv := range page.Invoices {
		if inv.Number != nil {
			out[len(out)-1-i] = *inv.Number
		}
	}
	return out
}

// documentJobs is the invoices whose document job was enqueued, in order.
func (e *docsEnv) documentJobs() []uuid.UUID {
	var ids []uuid.UUID
	for _, j := range e.queue.all() {
		if a, ok := j.(model.InvoiceDocumentArgs); ok {
			if a.WorkspaceID != e.ws {
				e.t.Fatalf("document job for workspace %s, want %s", a.WorkspaceID, e.ws)
			}
			ids = append(ids, a.InvoiceID)
		}
	}
	return ids
}

func TestPaidInvoicesAreNumberedPerYearWithoutGaps(t *testing.T) {
	e := newDocsEnv(t)
	p := e.plan(t3PlanOpts{Monthly: 7900})
	sub := e.subscribe(p.Monthly)
	y := e.year

	renewal := e.clock.AddDate(0, 1, 0)
	e.renew(sub, "pay_"+uuid.NewString(), renewal, true)
	replayed := "pay_" + uuid.NewString()
	e.renew(sub, replayed, renewal.AddDate(0, 1, 0), true)
	e.renew(sub, replayed, renewal.AddDate(0, 1, 0), true) // the webhook again
	e.renew(sub, "pay_"+uuid.NewString(), renewal.AddDate(0, 2, 0), false)
	e.renew(sub, "pay_"+uuid.NewString(), time.Date(y+1, time.January, 1, 0, 0, 1, 0, time.UTC), true)

	want := []string{
		fmt.Sprintf("RLY-%d-000001", y), fmt.Sprintf("RLY-%d-000002", y), fmt.Sprintf("RLY-%d-000003", y),
		"", fmt.Sprintf("RLY-%d-000001", y+1),
	}
	if got := e.numbers(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("numbers = %q, want %q", got, want)
	}
	if jobs := e.documentJobs(); len(jobs) != 4 {
		t.Fatalf("document jobs = %d, want one per paid invoice (4)", len(jobs))
	}
}

func TestARolledBackPaymentGivesItsNumberBack(t *testing.T) {
	e := newDocsEnv(t)
	p := e.plan(t3PlanOpts{Monthly: 7900})
	sub := e.subscribe(p.Monthly)

	at := e.clock.AddDate(0, 1, 0)
	end := at.AddDate(0, 1, 0)
	rollback := errors.New("later step failed")
	err := withTx(e, func(tx pgx.Tx) error {
		if err := e.svc.ApplySubscriptionPayment(e.ctx, tx, e.ws, sub.ID, payment.Payment{
			ExternalID: "pay_" + uuid.NewString(), SubtotalMinor: 7900, TotalMinor: 7900, Currency: "USD",
			PeriodStart: &at, PeriodEnd: &end, PaidAt: &at,
		}, true); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("tx = %v", err)
	}
	e.renew(sub, "pay_"+uuid.NewString(), at, true)

	want := fmt.Sprintf("RLY-%d-000002", e.year)
	if got := e.numbers(); len(got) != 2 || got[1] != want {
		t.Fatalf("numbers = %q, want the second to be %s", got, want)
	}
}

func withTx(e *docsEnv, fn func(pgx.Tx) error) error {
	tx, err := e.pool.Begin(e.ctx)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(e.ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(e.ctx)
}

func TestInvoiceDocumentRendersAndEmailsOnce(t *testing.T) {
	e := newDocsEnv(t)
	p := e.plan(t3PlanOpts{Monthly: 7900})
	e.subscribe(p.Monthly)
	jobs := e.documentJobs()
	if len(jobs) != 1 {
		t.Fatalf("document jobs = %d, want 1", len(jobs))
	}
	inv := jobs[0]

	before, err := e.svc.Invoice(e.ctx, e.ws, inv)
	if err != nil {
		t.Fatal(err)
	}
	if before.PDFURL != nil {
		t.Fatalf("pdf_url before the job = %q, want none", *before.PDFURL)
	}

	for range 2 {
		if err := e.svc.DeliverInvoiceDocument(e.ctx, e.ws, inv); err != nil {
			t.Fatal(err)
		}
	}

	if len(e.renderer.pages) != 1 {
		t.Fatalf("renders = %d, want 1", len(e.renderer.pages))
	}
	if len(e.mail.sent) != 1 {
		t.Fatalf("emails = %d, want 1", len(e.mail.sent))
	}
	after, err := e.svc.Invoice(e.ctx, e.ws, inv)
	if err != nil {
		t.Fatal(err)
	}
	number := fmt.Sprintf("RLY-%d-000001", e.year)
	if after.Number == nil || *after.Number != number {
		t.Fatalf("number = %v, want %s", after.Number, number)
	}
	if wantURL := fmt.Sprintf("memory://invoices/%s/%s.pdf", e.ws, inv); after.PDFURL == nil || *after.PDFURL != wantURL {
		t.Fatalf("pdf_url = %v, want %s", after.PDFURL, wantURL)
	}

	msg := e.mail.sent[0]
	if len(msg.To) != 1 || msg.To[0] != "billing@acme.test" {
		t.Errorf("to = %v, want bill_to's email", msg.To)
	}
	if msg.Subject != "Relay receipt "+number+" · $79.00" {
		t.Errorf("subject = %q", msg.Subject)
	}
	if len(msg.Attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(msg.Attachments))
	}
	a := msg.Attachments[0]
	if a.Filename != number+".pdf" || a.ContentType != "application/pdf" || !strings.HasPrefix(string(a.Data), "%PDF-1.4 render 1") {
		t.Errorf("attachment = %s %s %q", a.Filename, a.ContentType, a.Data)
	}
	if !strings.Contains(msg.Text, "http://app.test/billing?workspace="+e.ws.String()) {
		t.Errorf("text lacks the billing link: %s", msg.Text)
	}

	page := e.renderer.pages[0]
	period := fmt.Sprintf("Mar 1 – Apr 1, %d", e.year)
	for _, want := range []string{number, "Acme Ltd", "billing@acme.test", "Plan " + p.Code + " plan · monthly · " + period, "$79.00", "Paid with card via Stripe"} {
		if !strings.Contains(page, want) {
			t.Errorf("invoice HTML lacks %q", want)
		}
	}
	if strings.Contains(page, ">Discount<") || strings.Contains(page, ">Tax<") {
		t.Error("invoice HTML shows a zero discount or tax line")
	}
}

func TestInvoiceDocumentRenderFailureRetries(t *testing.T) {
	e := newDocsEnv(t)
	p := e.plan(t3PlanOpts{Monthly: 7900})
	e.subscribe(p.Monthly)
	inv := e.documentJobs()[0]

	e.renderer.err = errors.New("cloudflare returned 429")
	if err := e.svc.DeliverInvoiceDocument(e.ctx, e.ws, inv); err == nil {
		t.Fatal("expected the render error, so River retries")
	}
	got, err := e.svc.Invoice(e.ctx, e.ws, inv)
	if err != nil {
		t.Fatal(err)
	}
	if got.PDFURL != nil || len(e.mail.sent) != 0 {
		t.Fatalf("after a failed render: pdf_url = %v, emails = %d; want neither", got.PDFURL, len(e.mail.sent))
	}

	e.renderer.err = nil
	if err := e.svc.DeliverInvoiceDocument(e.ctx, e.ws, inv); err != nil {
		t.Fatal(err)
	}
	if len(e.mail.sent) != 1 {
		t.Fatalf("emails after the retry = %d, want 1", len(e.mail.sent))
	}
}

func TestInvoiceDescriptions(t *testing.T) {
	month, year := "month", "year"
	at := func(y int, m time.Month, d int) *time.Time {
		t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
		return &t
	}
	seq := int32(2)
	order := uuid.New()
	for _, c := range []struct {
		name string
		row  gen.GetInvoiceDocumentRow
		want string
	}{
		{"monthly period", gen.GetInvoiceDocumentRow{
			PlanName: "Growth", PlanInterval: &month, SubtotalMinor: 7900,
			PeriodStart: at(2026, 11, 7), PeriodEnd: at(2026, 12, 7),
		}, "Growth plan · monthly · Nov 7 – Dec 7, 2026"},
		{"month end clamped", gen.GetInvoiceDocumentRow{
			PlanName: "Growth", PlanInterval: &month,
			PeriodStart: at(2027, 1, 31), PeriodEnd: at(2027, 2, 28),
		}, "Growth plan · monthly · Jan 31 – Feb 28, 2027"},
		{"yearly across years", gen.GetInvoiceDocumentRow{
			PlanName: "Growth", PlanInterval: &year,
			PeriodStart: at(2026, 12, 7), PeriodEnd: at(2027, 12, 7),
		}, "Growth plan · yearly · Dec 7, 2026 – Dec 7, 2027"},
		{"proration", gen.GetInvoiceDocumentRow{
			PlanName: "Growth", PlanInterval: &month, Prorated: true,
			PeriodStart: at(2026, 11, 20), PeriodEnd: at(2026, 12, 7),
		}, "Growth plan · change of plan (prorated)"},
		{"short period, not a plan change", gen.GetInvoiceDocumentRow{
			PlanName: "Growth", PlanInterval: &month,
			PeriodStart: at(2026, 11, 20), PeriodEnd: at(2026, 12, 7),
		}, "Growth plan · monthly · Nov 20 – Dec 7, 2026"},
		{"proration over the full period", gen.GetInvoiceDocumentRow{
			PlanName: "Growth", PlanInterval: &month, SubtotalMinor: 5000, Prorated: true,
			PeriodStart: at(2026, 11, 7), PeriodEnd: at(2026, 12, 7),
		}, "Growth plan · change of plan (prorated)"},
		{"lifetime deal", gen.GetInvoiceDocumentRow{PlanName: "Growth", OrderID: &order}, "Growth plan · lifetime deal"},
		{
			"installment",
			gen.GetInvoiceDocumentRow{PlanName: "Growth plan", OrderID: &order, InstallmentSeq: &seq, InstallmentCount: 3},
			"Growth plan · lifetime deal · installment 2 of 3",
		},
	} {
		if got := invoiceDescription(c.row); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestMoney(t *testing.T) {
	for _, c := range []struct {
		minor    int64
		currency string
		want     string
	}{{7900, "USD", "$79.00"}, {-550, "USD", "-$5.50"}, {1234567, "EUR", "€12345.67"}, {10333, "BDT", "BDT 103.33"}} {
		if got := money(c.minor, c.currency); got != c.want {
			t.Errorf("money(%d, %s) = %q, want %q", c.minor, c.currency, got, c.want)
		}
	}
}

func TestSweepRequeuesInvoicesStillWithoutAPDF(t *testing.T) {
	e := newDocsEnv(t)
	p := e.plan(t3PlanOpts{Monthly: 7900})
	e.subscribe(p.Monthly)
	inv := e.documentJobs()[0]
	e.svc.now = func() time.Time { return time.Now().UTC() }
	swept := func() bool {
		t.Helper()
		e.queue.jobs = nil
		if _, err := e.svc.SweepInvoiceDocuments(e.ctx); err != nil {
			t.Fatal(err)
		}
		for _, j := range e.queue.all() {
			if a, ok := j.(model.InvoiceDocumentArgs); ok && a.InvoiceID == inv {
				return true
			}
		}
		return false
	}

	if swept() {
		t.Fatal("a fresh invoice is still its own job's to finish")
	}
	// Issued long ago, its job gave up: the sweep queues it again.
	e.exec(`UPDATE invoices SET created_at = '2000-01-01' WHERE id = $1`, inv)
	if !swept() {
		t.Fatal("an old invoice without a PDF was not queued again")
	}
	if err := e.svc.DeliverInvoiceDocument(e.ctx, e.ws, inv); err != nil {
		t.Fatal(err)
	}
	if swept() {
		t.Fatal("an invoice with a PDF was queued again")
	}
}

// sweptNow runs the sweep at the real time and reports whether it queued inv.
func (e *docsEnv) sweptNow(inv uuid.UUID) bool {
	e.t.Helper()
	e.svc.now = func() time.Time { return time.Now().UTC() }
	e.queue.jobs = nil
	if _, err := e.svc.SweepInvoiceDocuments(e.ctx); err != nil {
		e.t.Fatal(err)
	}
	for _, j := range e.queue.all() {
		if a, ok := j.(model.InvoiceDocumentArgs); ok && a.InvoiceID == inv {
			return true
		}
	}
	return false
}

func TestSweepRequeuesAnInvoiceWhoseEmailWasNeverSent(t *testing.T) {
	e := newDocsEnv(t)
	p := e.plan(t3PlanOpts{Monthly: 7900})
	e.subscribe(p.Monthly)
	inv := e.documentJobs()[0]
	e.exec(`UPDATE invoices SET pdf_key = 'invoices/stored.pdf', created_at = '2000-01-01' WHERE id = $1`, inv)
	if !e.sweptNow(inv) {
		t.Fatal("an invoice with its PDF but no receipt email was not queued again")
	}
	e.exec(`UPDATE invoices SET emailed_at = now(), document_swept_at = NULL WHERE id = $1`, inv)
	if e.sweptNow(inv) {
		t.Fatal("a delivered invoice was queued again")
	}
}

// An invoice whose job keeps failing waits longer each time, so it can't take
// a slot from newer invoices on every sweep.
func TestSweepBacksOffAnInvoiceThatKeepsFailing(t *testing.T) {
	e := newDocsEnv(t)
	p := e.plan(t3PlanOpts{Monthly: 7900})
	e.subscribe(p.Monthly)
	inv := e.documentJobs()[0]
	e.exec(`UPDATE invoices SET created_at = '2000-01-01' WHERE id = $1`, inv)
	if !e.sweptNow(inv) {
		t.Fatal("the first sweep skipped an old invoice without a PDF")
	}
	if e.sweptNow(inv) {
		t.Fatal("the next sweep queued it again at once")
	}
	e.exec(`UPDATE invoices SET document_swept_at = now() - interval '20 minutes' WHERE id = $1`, inv)
	if !e.sweptNow(inv) {
		t.Fatal("not queued again after its first backoff")
	}
	e.exec(`UPDATE invoices SET document_swept_at = now() - interval '20 minutes' WHERE id = $1`, inv)
	if e.sweptNow(inv) {
		t.Fatal("the second backoff is no longer than the first")
	}
}

// An invoice that still can't be delivered after maxDocumentSweeps sweeps is
// given up on, with one error log for a person, instead of being swept daily
// for good.
func TestSweepGivesUpOnAnInvoiceThatNeverDelivers(t *testing.T) {
	e := newDocsEnv(t)
	logs := &syncBuffer{}
	e.svc.log = slog.New(slog.NewJSONHandler(logs, nil))
	p := e.plan(t3PlanOpts{Monthly: 7900})
	e.subscribe(p.Monthly)
	inv := e.documentJobs()[0]
	e.exec(`UPDATE invoices SET created_at = '2000-01-01', document_sweeps = $2 - 1 WHERE id = $1`, inv, maxDocumentSweeps)
	if !e.sweptNow(inv) {
		t.Fatal("the last sweep skipped the invoice")
	}
	e.exec(`UPDATE invoices SET document_swept_at = '2000-01-01' WHERE id = $1`, inv)
	if e.sweptNow(inv) {
		t.Fatalf("swept again after %d sweeps", maxDocumentSweeps)
	}
	if n := strings.Count(logs.String(), inv.String()); n != 1 {
		t.Errorf("invoice logged %d times, want once", n)
	}
}

// The PDF names the price the invoice was paid for, not the price the
// subscription has moved to since.
func TestInvoiceDocumentNamesThePricePaidFor(t *testing.T) {
	e := newDocsEnv(t)
	p := e.plan(t3PlanOpts{Monthly: 7900})
	other := e.plan(t3PlanOpts{Monthly: 9900})
	sub := e.subscribe(p.Monthly)
	e.queue.jobs = nil
	e.renew(sub, "pay_"+uuid.NewString(), *sub.CurrentPeriodEnd, true)
	inv := e.documentJobs()[0]
	e.exec(`UPDATE subscriptions SET plan_price_id = $1 WHERE workspace_id = $2`, other.Yearly, e.ws)
	if err := e.svc.DeliverInvoiceDocument(e.ctx, e.ws, inv); err != nil {
		t.Fatal(err)
	}
	if page := e.renderer.pages[0]; !strings.Contains(page, "Plan "+p.Code+" plan · monthly") {
		t.Fatalf("invoice HTML doesn't name the monthly %s price it was paid for", p.Code)
	}
}

// The renewal that starts a scheduled downgrade is paid for the new price.
func TestInvoiceDocumentNamesTheDowngradeARenewalStarts(t *testing.T) {
	e := newDocsEnv(t)
	p := e.plan(t3PlanOpts{Monthly: 7900})
	smaller := e.plan(t3PlanOpts{Monthly: 2900})
	sub := e.subscribe(p.Monthly)
	e.exec(`UPDATE subscriptions SET pending_plan_price_id = $1, pending_change_at = current_period_end WHERE id = $2`,
		smaller.Monthly, sub.ID)
	e.queue.jobs = nil
	e.renew(sub, "pay_"+uuid.NewString(), *sub.CurrentPeriodEnd, true)
	if err := e.svc.DeliverInvoiceDocument(e.ctx, e.ws, e.documentJobs()[0]); err != nil {
		t.Fatal(err)
	}
	if page := e.renderer.pages[0]; !strings.Contains(page, "Plan "+smaller.Code+" plan · monthly") {
		t.Fatalf("the renewal invoice doesn't name the %s price it started", smaller.Code)
	}
}

// A discount the provider applied (not one of Relay's) lowers the amount, but
// the charge is still a full period, not a proration. A second charge inside
// a paid period (Creem's upgrade) is one.
func TestInvoiceDocumentTellsProrationsFromDiscounts(t *testing.T) {
	e := newDocsEnv(t)
	p := e.plan(t3PlanOpts{Monthly: 7900})
	sub := e.subscribe(p.Monthly)
	start := *sub.CurrentPeriodEnd
	end := start.AddDate(0, 1, 0)
	charge := func(total int64, at time.Time) {
		t.Helper()
		e.queue.jobs = nil
		e.inTx(func(tx pgx.Tx) error {
			return e.svc.ApplySubscriptionPayment(e.ctx, tx, e.ws, sub.ID, payment.Payment{
				ExternalID: "pay_" + uuid.NewString(), SubtotalMinor: 7900, DiscountMinor: 7900 - total, TotalMinor: total,
				Currency: "USD", PeriodStart: &start, PeriodEnd: &end, PaidAt: &at,
			}, true)
		})
		if err := e.svc.DeliverInvoiceDocument(e.ctx, e.ws, e.documentJobs()[0]); err != nil {
			t.Fatal(err)
		}
	}
	charge(6900, start)
	charge(2000, start.Add(24*time.Hour))
	pages := e.renderer.pages
	if want := "monthly · " + dateRange(start, end); !strings.Contains(pages[len(pages)-2], want) {
		t.Errorf("a provider-discounted renewal is not a full period (%q)", want)
	}
	if !strings.Contains(pages[len(pages)-1], "change of plan (prorated)") {
		t.Error("a second charge inside a paid period is not shown as a proration")
	}
}

// A duplicate subscription checkout's payment is refunded when it is
// recorded: its invoice says so, and nobody is thanked for it by email, now
// or by a later sweep.
func TestRefundedDuplicateGetsNoReceipt(t *testing.T) {
	e := newDocsEnv(t)
	p := e.plan(t3PlanOpts{Monthly: 7900})
	first := e.checkout(p.Monthly, "")
	second := e.checkout(p.Monthly, "")
	e.pay(second)
	e.pay(first)
	jobs := e.documentJobs()
	if len(jobs) != 2 {
		t.Fatalf("document jobs = %d, want the plan's and the duplicate's", len(jobs))
	}
	for _, id := range jobs {
		if err := e.svc.DeliverInvoiceDocument(e.ctx, e.ws, id); err != nil {
			t.Fatal(err)
		}
	}
	dup, err := e.svc.Invoice(e.ctx, e.ws, jobs[1])
	if err != nil {
		t.Fatal(err)
	}
	if dup.Status != model.InvoiceRefunded {
		t.Errorf("duplicate invoice %s, want refunded", dup.Status)
	}
	for _, m := range e.mail.sent {
		if strings.Contains(m.Subject, *dup.Number) {
			t.Errorf("receipt %q sent for a refunded payment", m.Subject)
		}
	}
	if len(e.renderer.pages) != 2 || !strings.Contains(e.renderer.pages[1], "Refunded") ||
		strings.Contains(e.renderer.pages[1], ">Paid<") {
		t.Errorf("duplicate invoice PDF doesn't say refunded")
	}

	e.tick(48 * time.Hour)
	if _, err := e.svc.SweepInvoiceDocuments(e.ctx); err != nil {
		t.Fatal(err)
	}
	queued := 0
	for _, j := range e.queue.all() {
		if a, ok := j.(model.InvoiceDocumentArgs); ok && a.InvoiceID == dup.ID {
			queued++
		}
	}
	if queued != 1 {
		t.Errorf("duplicate's document queued %d times, want once: a sweep has nothing to deliver", queued)
	}
}

// A lifetime deal a discount makes free charged nothing: its invoice and
// receipt must not say it was paid with a card (X6).
func TestFreeOrderInvoiceSaysNothingWasCharged(t *testing.T) {
	e := newDocsEnv(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	ltd := e.price(p.ID, model.BillingOneTime, "", 5000)
	d := e.discount(func(d *model.NewDiscount) { d.PercentBps = ptrTo(10000) })
	if _, err := e.svc.CreateCheckout(e.ctx, e.ws, e.user, model.CheckoutInput{
		PlanPriceID: ltd, DiscountCode: d.Code, IdempotencyKey: uuid.NewString(),
	}); err != nil {
		t.Fatal(err)
	}
	e.wantNothingCharged()
}

// A card trial's first invoice is for $0 too.
func TestCardTrialInvoiceSaysNothingWasCharged(t *testing.T) {
	e := newDocsEnv(t)
	p := e.plan(t3PlanOpts{Monthly: 1000, TrialDays: 14, RequiresCard: true})
	e.pay(e.checkout(p.Monthly, ""))
	e.wantNothingCharged()
}

// wantNothingCharged delivers the workspace's one invoice document and checks
// neither its PDF nor its email says money was paid.
func (e *docsEnv) wantNothingCharged() {
	e.t.Helper()
	jobs := e.documentJobs()
	if len(jobs) != 1 {
		e.t.Fatalf("document jobs = %d, want one", len(jobs))
	}
	if err := e.svc.DeliverInvoiceDocument(e.ctx, e.ws, jobs[0]); err != nil {
		e.t.Fatal(err)
	}
	if len(e.renderer.pages) != 1 {
		e.t.Fatalf("rendered %d pages, want 1", len(e.renderer.pages))
	}
	if page := e.renderer.pages[0]; strings.Contains(page, "Paid with card") || !strings.Contains(page, "Nothing was charged") {
		e.t.Errorf("$0 invoice PDF says it was paid with a card:\n%s", page)
	}
	if len(e.mail.sent) != 1 {
		e.t.Fatalf("sent %d emails, want the receipt", len(e.mail.sent))
	}
	if m := e.mail.sent[0]; strings.Contains(m.Text, "Thanks for your payment") || strings.Contains(m.Text, "Amount paid") {
		e.t.Errorf("$0 invoice email thanks for a payment:\n%s", m.Text)
	}
}
