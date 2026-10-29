package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database/dbtest"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// ltdHarness runs T4's rules against a real database. Track T3's rules are
// still stubs on this branch, so it stands in for them with the smallest
// version that writes the same rows (grants, checkout sessions) T4 reads.
// Tests that use it run one at a time: RunInstallmentsDue sees every workspace.
type ltdHarness struct {
	t    *testing.T
	ctx  context.Context
	pool *pgxpool.Pool
	r    repository.Repository
	l    lifetimeDeals
	fake *payment.Fake
	now  time.Time

	ws, user, customer uuid.UUID
	basicPlan, proPlan uuid.UUID
	// basic and pro are one-time (lifetime) prices; monthly is a subscription price.
	basic, pro, monthly model.Price
	// basic3 splits basic into 3 payments (total 31000: a markup, Q4).
	basic3 model.InstallmentOffer
	prices map[uuid.UUID]model.Price
	offers map[uuid.UUID]model.InstallmentOffer

	discounts       map[string]model.AppliedDiscount
	subCredit       model.SubscriptionCredit
	canceledForLTD  int
	paymentInputs   []model.PaymentCheckoutInput
	paymentSequence int
}

func newLTDHarness(t *testing.T, collection string) *ltdHarness {
	t.Helper()
	pool := dbtest.New(t)
	ctx := context.Background()
	fake := payment.NewFake(payment.Stripe, "whsec_test")
	reg, err := payment.NewRegistry(fake)
	if err != nil {
		t.Fatal(err)
	}
	svc := New(pool, reg, nil, Config{AppBaseURL: "https://app.test"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h := &ltdHarness{
		t: t, ctx: ctx, pool: pool, r: repository.NewRepository(pool), fake: fake,
		now:    time.Date(2026, time.January, 31, 12, 0, 0, 0, time.UTC),
		prices: map[uuid.UUID]model.Price{}, offers: map[uuid.UUID]model.InstallmentOffer{},
		discounts: map[string]model.AppliedDiscount{},
	}
	svc.now = func() time.Time { return h.now }
	fake.SetClock(func() time.Time { return h.now })
	h.l = lifetimeDeals{Service: svc, t3: trackT3{
		price: func(_ context.Context, id uuid.UUID) (model.Price, error) {
			if p, ok := h.prices[id]; ok {
				return p, nil
			}
			return model.Price{}, model.ErrPriceNotFound
		},
		offer: func(_ context.Context, id uuid.UUID) (model.InstallmentOffer, error) {
			if o, ok := h.offers[id]; ok {
				return o, nil
			}
			return model.InstallmentOffer{}, model.ErrOfferNotFound
		},
		applyDiscount: func(_ context.Context, _ uuid.UUID, code string, _ model.Price, _ *model.InstallmentOffer) (model.AppliedDiscount, error) {
			if d, ok := h.discounts[code]; ok {
				return d, nil
			}
			return model.AppliedDiscount{}, model.ErrDiscountNotFound
		},
		subscriptionCredit: func(context.Context, uuid.UUID) (model.SubscriptionCredit, error) { return h.subCredit, nil },
		cancelForLTD: func(context.Context, pgx.Tx, uuid.UUID) (model.SubscriptionCredit, error) {
			h.canceledForLTD++
			return h.subCredit, nil
		},
		activateGrant: h.activateGrant,
		suspendGrant: func(ctx context.Context, tx pgx.Tx, ws, id uuid.UUID) error {
			_, err := repository.NewRepository(tx).SuspendGrant(ctx, gen.SuspendGrantParams{Now: &h.now, WorkspaceID: ws, GrantID: id})
			return err
		},
		restoreGrant: func(ctx context.Context, tx pgx.Tx, ws, id uuid.UUID) error {
			_, err := repository.NewRepository(tx).RestoreGrant(ctx, gen.RestoreGrantParams{WorkspaceID: ws, GrantID: id})
			return err
		},
		billTo: func(context.Context, pgx.Tx, uuid.UUID) (model.BillTo, error) {
			return model.BillTo{LegalName: "Acme Ltd", Email: "billing@acme.test"}, nil
		},
		startPaymentCheckout: h.startPaymentCheckout,
	}}
	h.seed(collection)
	t.Cleanup(h.closeOpenInstallments)
	return h
}

func (h *ltdHarness) seed(collection string) {
	h.t.Helper()
	h.ws, h.user = uuid.New(), uuid.New()
	h.exec(`INSERT INTO workspaces (id, name) VALUES ($1, 'LTD test')`, h.ws)
	h.exec(`INSERT INTO users (id, email) VALUES ($1, $2)`, h.user, h.user.String()+"@relay.test")
	cust, err := h.r.EnsureBillingCustomer(h.ctx, gen.EnsureBillingCustomerParams{WorkspaceID: h.ws, Provider: "stripe"})
	h.must(err)
	h.customer = cust.ID

	h.basicPlan, h.basic = h.seedPlan("basic", 30000)
	h.proPlan, h.pro = h.seedPlan("pro", 60000)
	month := model.IntervalMonth
	monthlyID, err := h.r.CreatePlanPrice(h.ctx, gen.CreatePlanPriceParams{
		PlanID: h.basicPlan, BillingType: model.BillingRecurring, Interval: &month, Currency: "USD", AmountMinor: 2900,
	})
	h.must(err)
	h.monthly = model.Price{
		ID: monthlyID, PlanID: h.basicPlan, BillingType: model.BillingRecurring, Interval: month,
		Currency: "USD", AmountMinor: 2900,
	}
	h.prices[monthlyID] = h.monthly

	h.basic3 = h.seedOffer(h.basic, 3, 31000, 7, 2, collection)
}

func (h *ltdHarness) seedPlan(code string, amount int64) (uuid.UUID, model.Price) {
	h.t.Helper()
	planID, err := h.r.CreatePlan(h.ctx, gen.CreatePlanParams{
		Code: code + "_" + uuid.NewString()[:8], Version: 1, Name: code, Limits: []byte(`{}`),
	})
	h.must(err)
	priceID, err := h.r.CreatePlanPrice(h.ctx, gen.CreatePlanPriceParams{
		PlanID: planID, BillingType: model.BillingOneTime, Currency: "USD", AmountMinor: amount,
	})
	h.must(err)
	p := model.Price{ID: priceID, PlanID: planID, BillingType: model.BillingOneTime, Currency: "USD", AmountMinor: amount}
	h.prices[priceID] = p
	return planID, p
}

func (h *ltdHarness) seedOffer(price model.Price, count int, total int64, grace, maxMissed int, collection string) model.InstallmentOffer {
	h.t.Helper()
	id, err := h.r.CreateInstallmentOffer(h.ctx, gen.CreateInstallmentOfferParams{
		PlanPriceID: price.ID, Count: int32(count), TotalMinor: total, //nolint:gosec // small test values
		GraceDays: int32(grace), MaxMissed: int32(maxMissed), //nolint:gosec // small test values
	})
	h.must(err)
	o := model.InstallmentOffer{
		ID: id, PlanPriceID: price.ID, Count: count, Interval: model.IntervalMonth, TotalMinor: total,
		GraceDays: grace, MaxMissed: maxMissed,
		Providers: []model.OfferProvider{{Provider: payment.Stripe, Collection: collection}},
	}
	h.offers[id] = o
	return o
}

// activateGrant is T3's ActivateGrant, reduced to its rows: end the current grant, start the new one.
func (h *ltdHarness) activateGrant(ctx context.Context, tx pgx.Tx, ws uuid.UUID, in model.GrantInput) (model.Grant, error) {
	r := repository.NewRepository(tx)
	cur, err := r.LockCurrentGrant(ctx, ws)
	switch {
	case err == nil:
		if _, err := r.EndGrant(ctx, gen.EndGrantParams{Now: &in.StartsAt, WorkspaceID: ws, GrantID: cur.ID}); err != nil {
			return model.Grant{}, err
		}
	case !repository.IsNoRows(err):
		return model.Grant{}, err
	}
	id, err := r.CreateGrant(ctx, gen.CreateGrantParams{
		WorkspaceID: ws, PlanID: in.PlanID, Source: in.Source, OrderID: in.OrderID, Limits: []byte(`{}`),
		StartsAt: in.StartsAt, EndsAt: in.EndsAt, CreatedBy: in.CreatedBy,
	})
	return model.Grant{ID: id, PlanID: in.PlanID, Source: in.Source, OrderID: in.OrderID, Status: model.GrantActive}, err
}

// startPaymentCheckout is T3's StartPaymentCheckout, reduced to its rows plus the fake gateway.
func (h *ltdHarness) startPaymentCheckout(ctx context.Context, ws, userID uuid.UUID, in model.PaymentCheckoutInput) (model.StartedCheckout, error) {
	h.paymentInputs = append(h.paymentInputs, in)
	o, err := h.r.GetOrder(ctx, gen.GetOrderParams{WorkspaceID: ws, OrderID: in.OrderID})
	if err != nil {
		return model.StartedCheckout{}, err
	}
	var createdBy *uuid.UUID
	if userID != uuid.Nil {
		createdBy = &userID
	}
	row, err := h.r.CreateCheckoutSession(ctx, gen.CreateCheckoutSessionParams{
		WorkspaceID: ws, PlanPriceID: o.PlanPriceID, InstallmentOfferID: o.InstallmentOfferID,
		Purpose: model.PurposeInstallment, OrderID: &o.ID, InstallmentID: in.InstallmentID,
		Provider: string(in.Provider), AmountMinor: in.AmountMinor, Currency: in.Currency,
		IdempotencyKey: in.IdempotencyKey, CreatedBy: createdBy, ExpiresAt: h.now.Add(time.Hour),
	})
	if repository.IsNoRows(err) {
		got, err := h.r.GetCheckoutSessionByIdempotencyKey(ctx, gen.GetCheckoutSessionByIdempotencyKeyParams{
			WorkspaceID: ws, IdempotencyKey: in.IdempotencyKey,
		})
		return model.StartedCheckout{Session: sessionFromRow(gen.CreateCheckoutSessionRow(got))}, err
	}
	if err != nil {
		return model.StartedCheckout{}, err
	}
	c, err := h.fake.CreateCheckout(ctx, payment.CheckoutRequest{
		Reference: row.ID, IdempotencyKey: in.IdempotencyKey, Mode: payment.ModePayment,
		AmountMinor: in.AmountMinor, Currency: in.Currency, Description: in.Description, ReturnURL: in.ReturnURL,
	})
	if err != nil {
		return model.StartedCheckout{}, err
	}
	return model.StartedCheckout{Session: sessionFromRow(row), ClientSecret: c.ClientSecret}, nil
}

func sessionFromRow(r gen.CreateCheckoutSessionRow) model.CheckoutSession {
	return model.CheckoutSession{
		ID: r.ID, WorkspaceID: r.WorkspaceID, PlanPriceID: r.PlanPriceID, InstallmentOfferID: r.InstallmentOfferID,
		Purpose: r.Purpose, SubscriptionID: r.SubscriptionID, OrderID: r.OrderID, InstallmentID: r.InstallmentID,
		Provider: payment.Provider(r.Provider), Status: r.Status, AmountMinor: r.AmountMinor, Currency: r.Currency,
		CreatedBy: r.CreatedBy, ExpiresAt: r.ExpiresAt, CreatedAt: r.CreatedAt,
	}
}

// quote asks for a lifetime deal on price, in installments when offer is set.
func (h *ltdHarness) quote(price model.Price, offer *model.InstallmentOffer, code string) (model.OrderQuote, error) {
	in := model.QuoteInput{PlanPriceID: price.ID, DiscountCode: code}
	if offer != nil {
		in.InstallmentOfferID = &offer.ID
	}
	return h.l.quoteOrder(h.ctx, h.ws, in)
}

// buy does what T3's CreateCheckout and CompleteCheckout do for a lifetime
// deal: the pending order and its ltd session, then the first payment.
func (h *ltdHarness) buy(q model.OrderQuote, scheduleID string) model.Order {
	h.t.Helper()
	var order model.Order
	var session model.CheckoutSession
	h.must(database.WithTx(h.ctx, h.pool, func(tx pgx.Tx) error {
		var err error
		order, err = h.l.CreateOrder(h.ctx, tx, h.ws, model.NewOrder{
			Provider: payment.Stripe, BillingCustomerID: h.customer, Quote: q,
		})
		if err != nil {
			return err
		}
		row, err := repository.NewRepository(tx).CreateCheckoutSession(h.ctx, gen.CreateCheckoutSessionParams{
			WorkspaceID: h.ws, PlanPriceID: q.PlanPriceID, InstallmentOfferID: q.InstallmentOfferID,
			Purpose: model.PurposeLTD, OrderID: &order.ID, Provider: "stripe", AmountMinor: q.DueNowMinor,
			Currency: q.Currency, IdempotencyKey: uuid.NewString(), CreatedBy: &h.user, ExpiresAt: h.now.Add(time.Hour),
		})
		session = sessionFromRow(row)
		return err
	}))
	p := h.payment(q.DueNowMinor)
	p.ExternalScheduleID = scheduleID
	h.complete(session, p)
	return h.order(order.ID)
}

func (h *ltdHarness) complete(session model.CheckoutSession, p payment.Payment) {
	h.t.Helper()
	h.must(h.completeErr(session, p))
}

func (h *ltdHarness) completeErr(session model.CheckoutSession, p payment.Payment) error {
	return database.WithTx(h.ctx, h.pool, func(tx pgx.Tx) error {
		return h.l.completeOrderCheckout(h.ctx, tx, h.ws, session, p)
	})
}

// providerPayment applies a provider-run installment payment, as T5's webhook dispatch does.
func (h *ltdHarness) providerPayment(orderID uuid.UUID, p payment.Payment, paid bool) error {
	return database.WithTx(h.ctx, h.pool, func(tx pgx.Tx) error {
		return h.l.applyInstallmentPayment(h.ctx, tx, h.ws, orderID, p, paid)
	})
}

func (h *ltdHarness) payment(amount int64) payment.Payment {
	h.paymentSequence++
	paidAt := h.now
	return payment.Payment{
		ExternalID: "pi_" + uuid.NewString(), SubtotalMinor: amount, TotalMinor: amount, Currency: "USD",
		PaidAt: &paidAt,
	}
}

// run advances the clock and runs the daily installments pass.
func (h *ltdHarness) run(advance time.Duration) model.InstallmentRun {
	h.t.Helper()
	h.now = h.now.Add(advance)
	run, err := h.l.runInstallmentsDue(h.ctx)
	h.must(err)
	return run
}

func (h *ltdHarness) order(id uuid.UUID) model.Order {
	h.t.Helper()
	o, err := h.l.order(h.ctx, h.ws, id)
	h.must(err)
	return o
}

func (h *ltdHarness) grant(orderID uuid.UUID) gen.GetOrderGrantRow {
	h.t.Helper()
	g, err := h.r.GetOrderGrant(h.ctx, gen.GetOrderGrantParams{WorkspaceID: h.ws, OrderID: &orderID})
	h.must(err)
	return g
}

func (h *ltdHarness) invoiceCount(orderID uuid.UUID) int {
	h.t.Helper()
	var n int
	h.must(h.pool.QueryRow(h.ctx, `SELECT count(*) FROM invoices WHERE workspace_id = $1 AND order_id = $2`,
		h.ws, orderID).Scan(&n))
	return n
}

// closeOpenInstallments takes this test's orders out of later tests' daily passes.
func (h *ltdHarness) closeOpenInstallments() {
	h.exec(`UPDATE installments SET status = 'canceled'
	        WHERE workspace_id = $1 AND status IN ('scheduled', 'due', 'overdue', 'failed')`, h.ws)
}

func (h *ltdHarness) exec(sql string, args ...any) {
	h.t.Helper()
	_, err := h.pool.Exec(h.ctx, sql, args...)
	h.must(err)
}

func (h *ltdHarness) must(err error) {
	h.t.Helper()
	if err != nil {
		h.t.Fatal(err)
	}
}

func statuses(insts []model.Installment) []string {
	out := make([]string, len(insts))
	for i, in := range insts {
		out[i] = in.Status
	}
	return out
}

func amounts(sched []model.ScheduledPayment) []int64 {
	out := make([]int64, len(sched))
	for i, s := range sched {
		out[i] = s.AmountMinor
	}
	return out
}

func wantOrder(t *testing.T, o model.Order, status string, paid int64, insts ...string) {
	t.Helper()
	if o.Status != status || o.PaidMinor != paid {
		t.Errorf("order: status %s paid %d, want %s paid %d", o.Status, o.PaidMinor, status, paid)
	}
	if got := statuses(o.Installments); !slices.Equal(got, insts) {
		t.Errorf("installments %v, want %v", got, insts)
	}
}

func wantGrant(t *testing.T, g gen.GetOrderGrantRow, status string) {
	t.Helper()
	if g.Status != status {
		t.Errorf("grant %s, want %s", g.Status, status)
	}
	if g.EndsAt != nil && status != model.GrantEnded {
		t.Errorf("lifetime grant ends at %v, want never", g.EndsAt)
	}
}

const month = 31 * 24 * time.Hour

func TestLifetimeDealLifecycle(t *testing.T) {
	tests := []struct {
		name       string
		collection string
		run        func(t *testing.T, h *ltdHarness)
	}{
		{"paid in full at once", model.CollectionAppLink, func(t *testing.T, h *ltdHarness) {
			q, err := h.quote(h.basic, nil, "")
			h.must(err)
			if q.DueNowMinor != 30000 || len(q.Schedule) != 1 {
				t.Fatalf("quote due now %d schedule %v, want 30000 in one payment", q.DueNowMinor, q.Schedule)
			}
			o := h.buy(q, "")
			wantOrder(t, o, model.OrderPaid, 30000)
			if o.PaidInFull == nil {
				t.Error("paid_in_full_at not set")
			}
			wantGrant(t, h.grant(o.ID), model.GrantActive)
		}},
		{"3 provider-run installments paid on time", model.CollectionProviderSchedule, func(t *testing.T, h *ltdHarness) {
			q, err := h.quote(h.basic, &h.basic3, "")
			h.must(err)
			o := h.buy(q, "sub_sched_1")
			// Q1: full access from installment 1.
			wantOrder(t, o, model.OrderInInstallments, 10333, "paid", "scheduled", "scheduled")
			wantGrant(t, h.grant(o.ID), model.GrantActive)
			if o.ExternalRef == nil || *o.ExternalRef != "sub_sched_1" {
				t.Errorf("external_ref %v, want the schedule ID", o.ExternalRef)
			}

			for i, amount := range []int64{10333, 10334} {
				run := h.run(month)
				if run.MarkedDue != 1 || run.Reminded != 1 {
					t.Errorf("payment %d: run %+v, want one marked due and reminded", i+2, run)
				}
				h.must(h.providerPayment(o.ID, h.payment(amount), true))
			}
			if len(h.paymentInputs) != 0 {
				t.Errorf("provider-run schedule got %d pay links, want none", len(h.paymentInputs))
			}
			o = h.order(o.ID)
			wantOrder(t, o, model.OrderPaid, 31000, "paid", "paid", "paid")
			wantGrant(t, h.grant(o.ID), model.GrantActive)
			if n := h.invoiceCount(o.ID); n != 3 {
				t.Errorf("%d invoices, want 3", n)
			}
		}},
		{"3 app-link installments paid on time", model.CollectionAppLink, func(t *testing.T, h *ltdHarness) {
			q, err := h.quote(h.basic, &h.basic3, "")
			h.must(err)
			o := h.buy(q, "")
			for i := range 2 {
				h.run(month)
				// The emailed link's checkout, paid.
				in := h.paymentInputs[len(h.paymentInputs)-1]
				if !strings.HasPrefix(in.IdempotencyKey, "installment-due-") || *in.InstallmentID != o.Installments[i+1].ID {
					t.Fatalf("after due date %d: pay link %+v, want installment %d's", i+2, in, i+2)
				}
				started, err := h.l.payInstallment(h.ctx, h.ws, h.user, o.ID, *in.InstallmentID, "pay-"+in.InstallmentID.String())
				h.must(err)
				h.complete(started.Session, h.payment(in.AmountMinor))
			}
			o = h.order(o.ID)
			wantOrder(t, o, model.OrderPaid, 31000, "paid", "paid", "paid")
			wantGrant(t, h.grant(o.ID), model.GrantActive)
		}},
		{"missed, then paid late", model.CollectionAppLink, func(t *testing.T, h *ltdHarness) {
			q, err := h.quote(h.basic, &h.basic3, "")
			h.must(err)
			o := h.buy(q, "")
			h.run(month)
			if run := h.run(8 * 24 * time.Hour); run.Overdue != 1 || run.Suspended != 1 || run.Defaulted != 0 {
				t.Errorf("past grace: run %+v, want one overdue and suspended", run)
			}
			wantGrant(t, h.grant(o.ID), model.GrantSuspended)
			wantOrder(t, h.order(o.ID), model.OrderInInstallments, 10333, "paid", "overdue", "scheduled")

			// Q2: a late payment restores it.
			second := h.order(o.ID).Installments[1]
			started, err := h.l.payInstallment(h.ctx, h.ws, h.user, o.ID, second.ID, "late")
			h.must(err)
			h.complete(started.Session, h.payment(second.AmountMinor))
			wantGrant(t, h.grant(o.ID), model.GrantActive)
			wantOrder(t, h.order(o.ID), model.OrderInInstallments, 20666, "paid", "paid", "scheduled")
		}},
		{"provider payment missed, then paid late", model.CollectionProviderSchedule, func(t *testing.T, h *ltdHarness) {
			q, err := h.quote(h.basic, &h.basic3, "")
			h.must(err)
			o := h.buy(q, "sub_sched_2")
			h.now = h.now.Add(month)
			h.must(h.providerPayment(o.ID, h.payment(10333), false))
			h.run(8 * 24 * time.Hour)
			wantGrant(t, h.grant(o.ID), model.GrantSuspended)
			h.must(h.providerPayment(o.ID, h.payment(10333), true))
			wantGrant(t, h.grant(o.ID), model.GrantActive)
			wantOrder(t, h.order(o.ID), model.OrderInInstallments, 20666, "paid", "paid", "scheduled")
		}},
		{"defaulted, then resumed", model.CollectionAppLink, func(t *testing.T, h *ltdHarness) {
			q, err := h.quote(h.basic, &h.basic3, "")
			h.must(err)
			o := h.buy(q, "")
			h.run(month)
			h.run(8 * 24 * time.Hour) // installment 2 missed
			h.run(month - 8*24*time.Hour)
			if run := h.run(8 * 24 * time.Hour); run.Defaulted != 1 {
				t.Errorf("second miss: run %+v, want the order defaulted (max_missed 2)", run)
			}
			o = h.order(o.ID)
			wantOrder(t, o, model.OrderDefaulted, 10333, "paid", "overdue", "overdue")
			wantGrant(t, h.grant(o.ID), model.GrantSuspended)
			if o.DefaultedAt == nil {
				t.Error("defaulted_at not set")
			}

			if _, err := h.l.payInstallment(h.ctx, h.ws, h.user, o.ID, o.Installments[1].ID, "x"); !errors.Is(err, model.ErrInstallmentNotDue) {
				t.Errorf("pay an installment of a defaulted order: %v, want ErrInstallmentNotDue", err)
			}
			// Q12: the money paid stays; paying the balance resumes the deal.
			started, err := h.l.resumeOrder(h.ctx, h.ws, h.user, o.ID, "resume")
			h.must(err)
			if started.Session.AmountMinor != 20667 {
				t.Errorf("resume checkout for %d, want the balance 20667", started.Session.AmountMinor)
			}
			h.complete(started.Session, h.payment(20667))
			o = h.order(o.ID)
			wantOrder(t, o, model.OrderPaid, 31000, "paid", "paid", "canceled")
			wantGrant(t, h.grant(o.ID), model.GrantActive)
		}},
		{"upgraded at installment 2", model.CollectionAppLink, func(t *testing.T, h *ltdHarness) {
			q, err := h.quote(h.basic, &h.basic3, "")
			h.must(err)
			old := h.buy(q, "")
			h.run(month)
			in := h.paymentInputs[0]
			started, err := h.l.payInstallment(h.ctx, h.ws, h.user, old.ID, *in.InstallmentID, "two")
			h.must(err)
			h.complete(started.Session, h.payment(in.AmountMinor))

			pro3 := h.seedOffer(h.pro, 3, 60000, 7, 2, model.CollectionAppLink)
			up, err := h.quote(h.pro, &pro3, "")
			h.must(err)
			// Q16: what was paid on the old order is credit on the new one.
			if up.UpgradesOrderID == nil || *up.UpgradesOrderID != old.ID || up.CreditMinor != 20666 {
				t.Fatalf("upgrade quote %+v, want credit 20666 from the old order", up)
			}
			if got := amounts(up.Schedule); !slices.Equal(got, []int64{13111, 13111, 13112}) {
				t.Errorf("upgrade schedule %v", got)
			}
			o := h.buy(up, "")
			wantOrder(t, o, model.OrderInInstallments, 13111, "paid", "scheduled", "scheduled")
			wantGrant(t, h.grant(o.ID), model.GrantActive)
			wantOrder(t, h.order(old.ID), model.OrderUpgraded, 20666, "paid", "paid", "canceled")
			if g := h.grant(old.ID); g.Status != model.GrantEnded {
				t.Errorf("old order's grant %s, want ended", g.Status)
			}
		}},
		{"subscriber switches to a lifetime deal", model.CollectionAppLink, func(t *testing.T, h *ltdHarness) {
			subID, err := h.r.CreateSubscription(h.ctx, gen.CreateSubscriptionParams{
				WorkspaceID: h.ws, PlanPriceID: h.monthly.ID, BillingCustomerID: h.customer, Provider: "stripe",
				RenewalMode: model.RenewalProvider, Status: model.SubscriptionActive,
			})
			h.must(err)
			h.subCredit = model.SubscriptionCredit{SubscriptionID: &subID, CreditMinor: 1500, Currency: "USD"}
			q, err := h.quote(h.basic, nil, "")
			h.must(err)
			if q.ReplacesSubscriptionID == nil || q.CreditMinor != 1500 || q.DueNowMinor != 28500 {
				t.Fatalf("quote %+v, want the subscription's 1500 credited", q)
			}
			o := h.buy(q, "")
			if h.canceledForLTD != 1 {
				t.Errorf("CancelForLTD called %d times, want 1", h.canceledForLTD)
			}
			wantOrder(t, o, model.OrderPaid, 28500)
			if o.CreditMinor != 1500 || o.ReplacedSubscriptionID == nil {
				t.Errorf("order credit %d replaced %v", o.CreditMinor, o.ReplacedSubscriptionID)
			}
			wantGrant(t, h.grant(o.ID), model.GrantActive)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t, newLTDHarness(t, tt.collection))
		})
	}
}

func TestQuoteOrderDiscountThenCredit(t *testing.T) {
	h := newLTDHarness(t, model.CollectionAppLink)
	maxR := int32(100)
	discountID, err := h.r.CreateDiscount(h.ctx, gen.CreateDiscountParams{
		Code: "LTD" + uuid.NewString()[:6], Name: "LTD", Kind: model.DiscountAmount, AmountOffMinor: new(int64(1000)),
		Currency: new("USD"), Duration: model.DurationOnce, AppliesTo: model.AppliesToAll, AllowInstallments: true,
		MaxRedemptions: &maxR, PerWorkspaceLimit: 1, StartsAt: h.now.Add(-time.Hour),
	})
	h.must(err)
	h.discounts["TENOFF"] = model.AppliedDiscount{Discount: model.Discount{ID: discountID}, DiscountMinor: 1000}
	subID := uuid.New()
	h.subCredit = model.SubscriptionCredit{SubscriptionID: &subID, CreditMinor: 1, Currency: "USD"}

	q, err := h.quote(h.basic, &h.basic3, "TENOFF")
	h.must(err)
	// 31000 - 1000 discount - 1 credit = 29999, the last payment absorbs the rounding.
	if q.DiscountMinor != 1000 || q.CreditMinor != 1 || !slices.Equal(amounts(q.Schedule), []int64{9999, 9999, 10001}) {
		t.Errorf("quote discount %d credit %d schedule %v", q.DiscountMinor, q.CreditMinor, amounts(q.Schedule))
	}
	if q.DueNowMinor != 9999 {
		t.Errorf("due now %d, want 9999", q.DueNowMinor)
	}
	// Due dates keep the day but never pass the month's end.
	wantDays := []string{"2026-01-31", "2026-02-28", "2026-03-31"}
	for i, s := range q.Schedule {
		if got := s.DueAt.Format(time.DateOnly); got != wantDays[i] {
			t.Errorf("payment %d due %s, want %s", s.Seq, got, wantDays[i])
		}
	}
}

func TestSchedule(t *testing.T) {
	now := time.Date(2026, time.March, 31, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		balance int64
		count   int
		want    []int64
	}{
		{"divides evenly", 30000, 3, []int64{10000, 10000, 10000}},
		{"last absorbs one cent", 10000, 3, []int64{3333, 3333, 3334}},
		{"last absorbs two cents", 10001, 3, []int64{3333, 3333, 3335}},
		{"twelve payments", 99999, 12, []int64{8333, 8333, 8333, 8333, 8333, 8333, 8333, 8333, 8333, 8333, 8333, 8336}},
		{"one payment", 4999, 1, []int64{4999}},
		{"balance smaller than count", 2, 3, []int64{1, 1}},
		{"nothing owed", 0, 3, []int64{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := schedule(tt.balance, tt.count, model.IntervalMonth, now)
			if !slices.Equal(amounts(got), tt.want) {
				t.Errorf("schedule(%d, %d) = %v, want %v", tt.balance, tt.count, amounts(got), tt.want)
			}
			var sum int64
			for i, p := range got {
				sum += p.AmountMinor
				if p.Seq != i+1 {
					t.Errorf("payment %d has seq %d", i, p.Seq)
				}
			}
			if sum != tt.balance {
				t.Errorf("payments add up to %d, want %d", sum, tt.balance)
			}
		})
	}
	if got := addInterval(now, model.IntervalMonth, 1).Format(time.DateOnly); got != "2026-04-30" {
		t.Errorf("Mar 31 + 1 month = %s, want 2026-04-30", got)
	}
	if got := addInterval(time.Date(2028, time.February, 29, 0, 0, 0, 0, time.UTC), model.IntervalYear, 1).Format(time.DateOnly); got != "2029-02-28" {
		t.Errorf("Feb 29 + 1 year = %s, want 2029-02-28", got)
	}
}

func TestQuoteOrderRejectsWhatCannotBeUpgraded(t *testing.T) {
	h := newLTDHarness(t, model.CollectionAppLink)
	q, err := h.quote(h.pro, nil, "")
	h.must(err)
	h.buy(q, "")

	if _, err := h.quote(h.basic, nil, ""); !errors.Is(err, model.ErrOrderNotUpgradable) {
		t.Errorf("downgrade a lifetime deal: %v, want ErrOrderNotUpgradable", err)
	}
	if _, err := h.quote(h.pro, nil, ""); !errors.Is(err, model.ErrOrderNotUpgradable) {
		t.Errorf("buy the same deal again: %v, want ErrOrderNotUpgradable", err)
	}
	if _, err := h.quote(h.monthly, nil, ""); !errors.Is(err, model.ErrInvalidInput) {
		t.Errorf("quote a subscription price: %v, want ErrInvalidInput", err)
	}
}

func TestCompleteOrderCheckoutIgnoresAReplayedPayment(t *testing.T) {
	h := newLTDHarness(t, model.CollectionAppLink)
	q, err := h.quote(h.basic, &h.basic3, "")
	h.must(err)
	o := h.buy(q, "")
	h.run(month)
	in := h.paymentInputs[0]
	started, err := h.l.payInstallment(h.ctx, h.ws, h.user, o.ID, *in.InstallmentID, "two")
	h.must(err)
	p := h.payment(in.AmountMinor)
	h.complete(started.Session, p)
	h.complete(started.Session, p)

	wantOrder(t, h.order(o.ID), model.OrderInInstallments, 20666, "paid", "paid", "scheduled")
	if n := h.invoiceCount(o.ID); n != 2 {
		t.Errorf("%d invoices, want 2", n)
	}
}

func TestCurrentOrder(t *testing.T) {
	h := newLTDHarness(t, model.CollectionAppLink)
	if _, err := h.l.currentOrder(h.ctx, h.ws); !errors.Is(err, model.ErrNoOrder) {
		t.Fatalf("no order yet: %v, want ErrNoOrder", err)
	}
	q, err := h.quote(h.basic, &h.basic3, "")
	h.must(err)
	o := h.buy(q, "")
	got, err := h.l.currentOrder(h.ctx, h.ws)
	h.must(err)
	if got.ID != o.ID || len(got.Installments) != 3 || got.Price == nil || got.Price.ID != h.basic.ID {
		t.Errorf("current order %+v", got)
	}
	if _, err := h.l.order(h.ctx, uuid.New(), o.ID); !errors.Is(err, model.ErrNoOrder) {
		t.Errorf("another workspace's order: %v, want ErrNoOrder", err)
	}
}
