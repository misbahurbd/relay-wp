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

func TestApplyInstallmentPaymentRejectsOverpayment(t *testing.T) {
	h := newLTDHarness(t, model.CollectionProviderSchedule)
	q, err := h.quote(h.basic, &h.basic3, "")
	h.must(err)
	o := h.buy(q, "sub_sched_over")

	// The orders CHECK refuses paying past the total, and nothing is kept.
	if err := h.providerPayment(o.ID, h.payment(30000), true); !errors.Is(err, model.ErrOverpayment) {
		t.Fatalf("pay more than the balance: %v, want ErrOverpayment", err)
	}
	wantOrder(t, h.order(o.ID), model.OrderInInstallments, 10333, "paid", "scheduled", "scheduled")
	if n := h.invoiceCount(o.ID); n != 1 {
		t.Errorf("%d invoices, want 1", n)
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
	if !strings.Contains(in.ReturnURL, "workspace="+h.ws.String()) || strings.Contains(in.ReturnURL, "/w/") {
		t.Errorf("return URL %q should name the workspace in a query parameter", in.ReturnURL)
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
	if len(msg.To) != 1 || msg.To[0] != "billing@acme.test" || !strings.Contains(msg.Text, "USD 103.33") ||
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
