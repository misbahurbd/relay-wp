package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// A provider-run installment charge for more than the balance (the schedule
// charged after the order was paid off another way) is refunded in full, as
// an app_link duplicate is: the event applies instead of failing on every
// retry, and the order is left as it was.
func TestOverpaidProviderInstallmentIsRefunded(t *testing.T) {
	h := newLTDHarness(t, model.CollectionProviderSchedule)
	q, err := h.quote(h.basic, &h.basic3, "")
	h.must(err)
	o := h.buy(q, "sub_sched_over")

	over := h.payment(30000)
	if err := h.providerPayment(o.ID, over, true); err != nil {
		t.Fatalf("pay more than the balance: %v, want applied", err)
	}
	h.sendRefunds()
	wantOrder(t, h.order(o.ID), model.OrderInInstallments, 10333, "paid", "scheduled", "scheduled")
	if n := h.invoiceCount(o.ID); n != 2 {
		t.Errorf("%d invoices, want 2: the overpayment is recorded", n)
	}
	refunds := h.fake.Refunds()
	if len(refunds) != 1 || refunds[0].ExternalPaymentID != over.ExternalID || refunds[0].AmountMinor != 30000 {
		t.Fatalf("refunds = %+v, want the overpayment refunded in full", refunds)
	}
	// The provider's retry of the same webhook refunds nothing more.
	h.must(h.providerPayment(o.ID, over, true))
	h.sendRefunds()
	if n := len(h.fake.RefundRequests()); n != 1 {
		t.Fatalf("%d refund requests after a replay, want 1", n)
	}
}

func TestApplyInstallmentPaymentKeepsTaxOffTheBalance(t *testing.T) {
	h := newLTDHarness(t, model.CollectionProviderSchedule)
	q, err := h.quote(h.basic, &h.basic3, "")
	h.must(err)
	o := h.buy(q, "sub_sched_tax")

	p := h.payment(10333)
	p.TaxMinor, p.TotalMinor = 2000, 12333 // Creem as merchant of record adds tax
	h.must(h.providerPayment(o.ID, p, true))
	wantOrder(t, h.order(o.ID), model.OrderInInstallments, 20666, "paid", "paid", "scheduled")
}

func TestPayInstallmentOnlyTheNextDueAppLinkOne(t *testing.T) {
	h := newLTDHarness(t, model.CollectionAppLink)
	q, err := h.quote(h.basic, &h.basic3, "")
	h.must(err)
	o := h.buy(q, "")
	insts := o.Installments

	if _, err := h.l.payInstallment(h.ctx, h.ws, h.user, o.ID, insts[1].ID, "early"); !errors.Is(err, model.ErrInstallmentNotDue) {
		t.Errorf("pay before the due date: %v, want ErrInstallmentNotDue", err)
	}
	if _, err := h.l.payInstallment(h.ctx, h.ws, h.user, o.ID, uuid.New(), "nope"); !errors.Is(err, model.ErrInstallmentNotFound) {
		t.Errorf("unknown installment: %v, want ErrInstallmentNotFound", err)
	}
	if _, err := h.l.payInstallment(h.ctx, uuid.New(), h.user, o.ID, insts[1].ID, "nope"); !errors.Is(err, model.ErrNoOrder) {
		t.Errorf("another workspace's order: %v, want ErrNoOrder", err)
	}
	if _, err := h.l.resumeOrder(h.ctx, h.ws, h.user, o.ID, "nope"); !errors.Is(err, model.ErrOrderNotDefaulted) {
		t.Errorf("resume a healthy order: %v, want ErrOrderNotDefaulted", err)
	}

	h.run(2 * month) // installments 2 and 3 both due
	if _, err := h.l.payInstallment(h.ctx, h.ws, h.user, o.ID, insts[2].ID, "skip"); !errors.Is(err, model.ErrInstallmentNotDue) {
		t.Errorf("pay installment 3 before 2: %v, want ErrInstallmentNotDue", err)
	}
	started, err := h.l.payInstallment(h.ctx, h.ws, h.user, o.ID, insts[1].ID, "two")
	h.must(err)
	s := started.Session
	if s.Purpose != model.PurposeInstallment || s.AmountMinor != 10333 || s.InstallmentID == nil || *s.InstallmentID != insts[1].ID {
		t.Errorf("pay-now checkout %+v", s)
	}
	in := h.paymentInputs[len(h.paymentInputs)-1]
	// The same page a plan checkout returns to, which shows the remembered session.
	if want := "https://app.test/billing/checkout?session=return"; in.ReturnURL != want {
		t.Errorf("return URL = %q, want %q", in.ReturnURL, want)
	}
}

func TestPayingAnInstallmentTwiceRefundsTheSecond(t *testing.T) {
	h := newLTDHarness(t, model.CollectionAppLink)
	q, err := h.quote(h.basic, &h.basic3, "")
	h.must(err)
	o := h.buy(q, "")
	h.run(month)

	// Two Pay windows for installment 2, both paid.
	first, err := h.l.payInstallment(h.ctx, h.ws, h.user, o.ID, o.Installments[1].ID, "window-1")
	h.must(err)
	second, err := h.l.payInstallment(h.ctx, h.ws, h.user, o.ID, o.Installments[1].ID, "window-2")
	h.must(err)
	h.complete(first.Session, h.payment(10333))
	dup := h.payment(10333)
	h.complete(second.Session, dup)
	h.sendRefunds()

	wantOrder(t, h.order(o.ID), model.OrderInInstallments, 20666, "paid", "paid", "scheduled")
	if n := h.invoiceCount(o.ID); n != 3 {
		t.Errorf("%d invoices, want 3: the duplicate is recorded", n)
	}
	refunds := h.fake.Refunds()
	if len(refunds) != 1 || refunds[0].ExternalPaymentID != dup.ExternalID || refunds[0].AmountMinor != 10333 {
		t.Fatalf("refunds = %+v, want the duplicate refunded in full", refunds)
	}
	// A replay of the duplicate's webhook refunds nothing more.
	h.complete(second.Session, dup)
	h.sendRefunds()
	if n := len(h.fake.RefundRequests()); n != 1 {
		t.Fatalf("%d refunds after a replay, want 1", n)
	}
}

// A duplicate installment payment keeps its numbered invoice, so Relay's
// numbers stay gapless, but the invoice says Refunded and nobody is thanked
// for the payment by email (N3).
func TestRefundedDuplicateInstallmentGetsNoReceipt(t *testing.T) {
	h := newLTDHarness(t, model.CollectionAppLink)
	mail := &fakeMailer{}
	renderer := &fakeRenderer{}
	h.l.docs.Mailer, h.l.docs.Renderer = mail, renderer
	q, err := h.quote(h.basic, &h.basic3, "")
	h.must(err)
	o := h.buy(q, "")
	h.run(month)
	first, err := h.l.payInstallment(h.ctx, h.ws, h.user, o.ID, o.Installments[1].ID, "window-1")
	h.must(err)
	second, err := h.l.payInstallment(h.ctx, h.ws, h.user, o.ID, o.Installments[1].ID, "window-2")
	h.must(err)
	h.complete(first.Session, h.payment(10333))
	dup := h.payment(10333)
	h.complete(second.Session, dup)
	for _, j := range h.queue.all() {
		if a, ok := j.(model.InvoiceDocumentArgs); ok {
			h.must(h.l.DeliverInvoiceDocument(h.ctx, a.WorkspaceID, a.InvoiceID))
		}
	}

	var number *string
	var status string
	h.must(h.pool.QueryRow(h.ctx, `SELECT number, status FROM invoices WHERE external_payment_id = $1`,
		dup.ExternalID).Scan(&number, &status))
	if number == nil || status != model.InvoiceRefunded {
		t.Fatalf("duplicate invoice number %v, status %s; want numbered and refunded", number, status)
	}
	receipts := 0
	for _, m := range mail.sent {
		if strings.Contains(m.Subject, *number) {
			t.Errorf("receipt %q sent for a refunded duplicate", m.Subject)
		}
		receipts++
	}
	if receipts != 2 {
		t.Errorf("%d receipts, want the first payment's and installment 2's", receipts)
	}
	refunded := 0
	for _, page := range renderer.pages {
		if strings.Contains(page, *number) && strings.Contains(page, "Refunded") {
			refunded++
		}
	}
	if refunded != 1 {
		t.Errorf("duplicate's PDF doesn't say refunded")
	}
}

func TestPayInstallmentRefusesAProviderRunSchedule(t *testing.T) {
	h := newLTDHarness(t, model.CollectionProviderSchedule)
	q, err := h.quote(h.basic, &h.basic3, "")
	h.must(err)
	o := h.buy(q, "sub_sched_3")
	h.run(month)
	if _, err := h.l.payInstallment(h.ctx, h.ws, h.user, o.ID, o.Installments[1].ID, "x"); !errors.Is(err, model.ErrInstallmentNotDue) {
		t.Errorf("pay a provider-collected installment: %v, want ErrInstallmentNotDue", err)
	}
}

func TestRunInstallmentsDue(t *testing.T) {
	h := newLTDHarness(t, model.CollectionAppLink)
	q, err := h.quote(h.basic, &h.basic3, "")
	h.must(err)
	o := h.buy(q, "")

	due := o.Installments[1].DueAt
	if run := h.run(due.Sub(h.now) - time.Minute); run != (model.InstallmentRun{}) {
		t.Errorf("nothing due yet: run %+v", run)
	}
	run := h.run(time.Minute)
	if run != (model.InstallmentRun{MarkedDue: 1, Reminded: 1}) {
		t.Errorf("due date: run %+v", run)
	}
	o = h.order(o.ID)
	if r := o.Installments[1]; r.Status != model.InstallmentDue || r.ReminderSentAt == nil {
		t.Errorf("installment 2: %s reminded %v", r.Status, r.ReminderSentAt)
	}
	// The reminder is an email linking to the billing page, not a checkout.
	if len(h.paymentInputs) != 0 {
		t.Errorf("reminder started %d checkouts, want none", len(h.paymentInputs))
	}
	msg := h.lastEmail()
	if len(msg.To) != 1 || msg.To[0] != "billing@acme.test" || !strings.Contains(msg.Text, "$103.33") ||
		!strings.Contains(msg.Text, "https://app.test/billing?workspace="+h.ws.String()) {
		t.Errorf("reminder email %+v", msg)
	}
	if run := h.run(6 * 24 * time.Hour); run != (model.InstallmentRun{}) {
		t.Errorf("within the 7 grace days: run %+v", run)
	}
	if run := h.run(2 * 24 * time.Hour); run != (model.InstallmentRun{Overdue: 1, Suspended: 1}) {
		t.Errorf("past grace: run %+v", run)
	}
	if run := h.run(24 * time.Hour); run != (model.InstallmentRun{}) {
		t.Errorf("a second pass the same day: run %+v", run)
	}
}

func TestRunInstallmentsDueCancelsAbandonedOrders(t *testing.T) {
	h := newLTDHarness(t, model.CollectionAppLink)
	q, err := h.quote(h.basic, &h.basic3, "")
	h.must(err)
	var o model.Order
	h.must(database.WithTx(h.ctx, h.pool, func(tx pgx.Tx) error {
		o, err = h.l.CreateOrder(h.ctx, tx, h.ws, model.NewOrder{
			Provider: payment.Stripe, BillingCustomerID: h.customer, Quote: q,
		})
		return err
	}))
	h.exec(`UPDATE orders SET created_at = $1 WHERE id = $2`, h.now, o.ID)
	// Its checkout may still complete: left alone.
	h.run(24 * time.Hour)
	wantOrder(t, h.order(o.ID), model.OrderPending, 0, "scheduled", "scheduled", "scheduled")

	// Its checkout never completed: canceled with its installments.
	h.run(abandonedOrderAge)
	wantOrder(t, h.order(o.ID), model.OrderCanceled, 0, "canceled", "canceled", "canceled")
	if len(h.paymentInputs) != 0 {
		t.Errorf("abandoned order got %d pay links", len(h.paymentInputs))
	}
}

// A batch of young pending orders, whose checkouts may still complete, never
// holds up installments due after them: the hourly pass reads past them.
func TestRunInstallmentsDueReadsPastYoungPendingOrders(t *testing.T) {
	h := newLTDHarness(t, model.CollectionAppLink)
	q, err := h.quote(h.basic, &h.basic3, "")
	h.must(err)
	o := h.buy(q, "")
	h.now = o.Installments[1].DueAt.Add(-time.Hour)
	h.must(database.WithTx(h.ctx, h.pool, func(tx pgx.Tx) error {
		for range installmentBatch {
			if _, err := h.l.CreateOrder(h.ctx, tx, h.ws, model.NewOrder{
				Provider: payment.Stripe, BillingCustomerID: h.customer, Quote: q,
			}); err != nil {
				return err
			}
		}
		return nil
	}))
	h.exec(`UPDATE installments SET due_at = $1 WHERE workspace_id = $2 AND order_id <> $3`, h.now, h.ws, o.ID)

	h.run(time.Hour)
	if got := h.order(o.ID).Installments[1].Status; got != model.InstallmentDue {
		t.Errorf("installment 2 behind %d young pending orders: %s, want due", installmentBatch, got)
	}
}

func TestDefaultStopsTheProviderSchedule(t *testing.T) {
	h := newLTDHarness(t, model.CollectionProviderSchedule)
	q, err := h.quote(h.basic, &h.basic3, "")
	h.must(err)
	o := h.buy(q, "sub_sched_default")
	// The provider fails installments 2 and 3; the second miss defaults the order.
	h.run(month)
	h.must(h.providerPayment(o.ID, h.payment(10333), false))
	h.run(8 * 24 * time.Hour)
	h.run(month - 8*24*time.Hour)
	h.must(h.providerPayment(o.ID, h.payment(10334), false))
	if run := h.run(8 * 24 * time.Hour); run.Defaulted != 1 {
		t.Fatalf("second miss: run %+v, want the order defaulted", run)
	}
	if got := h.fake.CanceledSchedules(); len(got) != 1 || got[0] != "sub_sched_default" {
		t.Errorf("canceled schedules %v, want the order's", got)
	}
}
