package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Track T4: lifetime-deal orders.

// trackT3 is every track T3 rule lifetime deals call. ltd binds them to the
// service; tests swap them while T3's methods are still stubs.
type trackT3 struct {
	price                func(ctx context.Context, planPriceID uuid.UUID) (model.Price, error)
	offer                func(ctx context.Context, offerID uuid.UUID) (model.InstallmentOffer, error)
	applyDiscount        func(ctx context.Context, workspaceID uuid.UUID, code string, price model.Price, offer *model.InstallmentOffer) (model.AppliedDiscount, error)
	subscriptionCredit   func(ctx context.Context, workspaceID uuid.UUID) (model.SubscriptionCredit, error)
	cancelForLTD         func(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID) (model.SubscriptionCredit, error)
	activateGrant        func(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, in model.GrantInput) (model.Grant, error)
	suspendGrant         func(ctx context.Context, tx pgx.Tx, workspaceID, grantID uuid.UUID) error
	restoreGrant         func(ctx context.Context, tx pgx.Tx, workspaceID, grantID uuid.UUID) error
	billTo               func(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID) (model.BillTo, error)
	startPaymentCheckout func(ctx context.Context, workspaceID, userID uuid.UUID, in model.PaymentCheckoutInput) (model.StartedCheckout, error)
}

// lifetimeDeals runs the order and installment rules.
type lifetimeDeals struct {
	*Service
	t3 trackT3
}

func (s *Service) ltd() lifetimeDeals {
	return lifetimeDeals{Service: s, t3: trackT3{
		price: s.Price, offer: s.InstallmentOffer, applyDiscount: s.ApplyDiscount,
		subscriptionCredit: s.SubscriptionCredit, cancelForLTD: s.CancelForLTD, activateGrant: s.ActivateGrant,
		suspendGrant: s.SuspendGrant, restoreGrant: s.RestoreGrant, billTo: s.BillTo,
		startPaymentCheckout: s.StartPaymentCheckout,
	}}
}

// abandonedOrderAge is how long a pending order waits for its checkout before
// the daily pass cancels it: well past CheckoutTTL and providers' webhook retries.
const abandonedOrderAge = 7 * 24 * time.Hour

// QuoteOrder says what a lifetime deal would cost the workspace now: the
// price or offer total, less a discount code, less credit from a live
// subscription (Q13) or the live order it would upgrade (Q16), split into the
// installment schedule.
func (s *Service) QuoteOrder(ctx context.Context, workspaceID uuid.UUID, in model.QuoteInput) (model.OrderQuote, error) {
	return s.ltd().quoteOrder(ctx, workspaceID, in)
}

func (l lifetimeDeals) quoteOrder(ctx context.Context, workspaceID uuid.UUID, in model.QuoteInput) (model.OrderQuote, error) {
	price, err := l.t3.price(ctx, in.PlanPriceID)
	if err != nil {
		return model.OrderQuote{}, err
	}
	if price.BillingType != model.BillingOneTime {
		return model.OrderQuote{}, model.ErrInvalidInput
	}
	q := model.OrderQuote{
		PlanPriceID: price.ID, InstallmentOfferID: in.InstallmentOfferID, Currency: price.Currency,
		TotalMinor: price.AmountMinor,
	}
	count, interval := 1, model.IntervalMonth
	var offer *model.InstallmentOffer
	if in.InstallmentOfferID != nil {
		o, err := l.t3.offer(ctx, *in.InstallmentOfferID)
		if err != nil {
			return model.OrderQuote{}, err
		}
		if o.PlanPriceID != price.ID {
			return model.OrderQuote{}, model.ErrOfferNotFound
		}
		offer, q.TotalMinor, count, interval = &o, o.TotalMinor, o.Count, o.Interval
	}

	if in.DiscountCode != "" {
		d, err := l.t3.applyDiscount(ctx, workspaceID, in.DiscountCode, price, offer)
		if err != nil {
			return model.OrderQuote{}, err
		}
		q.Discount, q.DiscountMinor = &d, min(d.DiscountMinor, q.TotalMinor)
	}

	credit, err := l.quoteCredit(ctx, workspaceID, price, &q)
	if err != nil {
		return model.OrderQuote{}, err
	}
	// The discount comes off the total first (Q18), then credit.
	q.CreditMinor = min(credit, q.TotalMinor-q.DiscountMinor)

	balance := q.TotalMinor - q.DiscountMinor - q.CreditMinor
	q.Schedule = schedule(balance, count, interval, l.now())
	if len(q.Schedule) > 0 {
		q.DueNowMinor = q.Schedule[0].AmountMinor
	}
	return q, nil
}

// quoteCredit is the money a new order carries in: what was paid on the live
// order it upgrades (Q16), or else the unused time on a live subscription (Q13).
func (l lifetimeDeals) quoteCredit(ctx context.Context, workspaceID uuid.UUID, price model.Price, q *model.OrderQuote) (int64, error) {
	live, err := repository.NewRepository(l.pool).GetLiveOrder(ctx, workspaceID)
	switch {
	case err == nil:
		old := orderFromRow(gen.GetOrderRow(live))
		if old.Status == model.OrderDefaulted {
			// Q12: a defaulted deal resumes by paying its balance, not by upgrading.
			return 0, model.ErrOrderNotUpgradable
		}
		oldPrice, err := l.t3.price(ctx, old.PlanPriceID)
		if err != nil {
			return 0, err
		}
		if oldPrice.PlanID == price.PlanID || price.AmountMinor <= oldPrice.AmountMinor ||
			old.Currency != price.Currency {
			return 0, model.ErrOrderNotUpgradable
		}
		q.UpgradesOrderID = &old.ID
		return old.PaidMinor + old.CreditMinor, nil
	case !repository.IsNoRows(err):
		return 0, fmt.Errorf("get live order: %w", err)
	}

	sc, err := l.t3.subscriptionCredit(ctx, workspaceID)
	if err != nil {
		return 0, err
	}
	if sc.SubscriptionID == nil {
		return 0, nil
	}
	if sc.CreditMinor > 0 && sc.Currency != price.Currency {
		return 0, model.ErrInvalidInput
	}
	q.ReplacesSubscriptionID = sc.SubscriptionID
	return sc.CreditMinor, nil
}

// schedule splits balance into count payments, the first due now and each
// later one an interval apart. A balance smaller than count gets fewer
// payments, since none may be zero; no balance gets none.
func schedule(balance int64, count int, interval string, now time.Time) []model.ScheduledPayment {
	if balance <= 0 {
		return nil
	}
	count = int(min(int64(count), balance))
	out := make([]model.ScheduledPayment, count)
	for i, amount := range model.SplitInstallments(balance, count) {
		out[i] = model.ScheduledPayment{Seq: i + 1, DueAt: addInterval(now, interval, i), AmountMinor: amount}
	}
	return out
}

// addInterval moves t n months (or years) on, keeping the day of the month
// but never running past the month's end: Jan 31 plus one month is Feb 28.
func addInterval(t time.Time, interval string, n int) time.Time {
	months := n
	if interval == model.IntervalYear {
		months = 12 * n
	}
	first := time.Date(t.Year(), t.Month(), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	target := first.AddDate(0, months, 0)
	lastDay := target.AddDate(0, 1, -1).Day()
	return target.AddDate(0, 0, min(t.Day(), lastDay)-1)
}

// CurrentOrder returns the workspace's live order with its installments.
// model.ErrNoOrder if there is none.
func (s *Service) CurrentOrder(ctx context.Context, workspaceID uuid.UUID) (model.Order, error) {
	return s.ltd().currentOrder(ctx, workspaceID)
}

func (l lifetimeDeals) currentOrder(ctx context.Context, workspaceID uuid.UUID) (model.Order, error) {
	r := repository.NewRepository(l.pool)
	row, err := r.GetLiveOrder(ctx, workspaceID)
	if repository.IsNoRows(err) {
		return model.Order{}, model.ErrNoOrder
	}
	if err != nil {
		return model.Order{}, fmt.Errorf("get live order: %w", err)
	}
	return l.withDetails(ctx, r, orderFromRow(gen.GetOrderRow(row)))
}

// Order returns one of the workspace's orders with its installments.
func (s *Service) Order(ctx context.Context, workspaceID, orderID uuid.UUID) (model.Order, error) {
	return s.ltd().order(ctx, workspaceID, orderID)
}

func (l lifetimeDeals) order(ctx context.Context, workspaceID, orderID uuid.UUID) (model.Order, error) {
	r := repository.NewRepository(l.pool)
	o, err := getOrder(ctx, r, workspaceID, orderID)
	if err != nil {
		return model.Order{}, err
	}
	return l.withDetails(ctx, r, o)
}

// withDetails fills in an order's installments and price for a response.
func (l lifetimeDeals) withDetails(ctx context.Context, r repository.Repository, o model.Order) (model.Order, error) {
	var err error
	if o.Installments, err = listInstallments(ctx, r, o.WorkspaceID, o.ID); err != nil {
		return model.Order{}, err
	}
	price, err := l.t3.price(ctx, o.PlanPriceID)
	if err != nil {
		return model.Order{}, err
	}
	o.Price = &price
	return o, nil
}

// CreateOrder creates a pending order and every installment row from a quote
// in tx, when a lifetime-deal checkout starts. Track T3's CreateCheckout calls it.
func (s *Service) CreateOrder(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, in model.NewOrder) (model.Order, error) {
	q := in.Quote
	balance := q.TotalMinor - q.DiscountMinor - q.CreditMinor
	if q.TotalMinor <= 0 || balance < 0 || q.UpgradesOrderID != nil && q.ReplacesSubscriptionID != nil {
		return model.Order{}, model.ErrInvalidInput
	}
	var discountID *uuid.UUID
	if q.Discount != nil {
		discountID = &q.Discount.Discount.ID
	}
	r := repository.NewRepository(tx)
	id, err := r.CreateOrder(ctx, gen.CreateOrderParams{
		WorkspaceID: workspaceID, PlanPriceID: q.PlanPriceID, InstallmentOfferID: q.InstallmentOfferID,
		BillingCustomerID: in.BillingCustomerID, Provider: string(in.Provider), Status: model.OrderPending,
		TotalMinor: q.TotalMinor, DiscountID: discountID, DiscountMinor: q.DiscountMinor, CreditMinor: q.CreditMinor,
		Currency: q.Currency, UpgradedFromOrderID: q.UpgradesOrderID, ReplacedSubscriptionID: q.ReplacesSubscriptionID,
	})
	if repository.IsForeignKey(err) || repository.IsCheck(err) {
		return model.Order{}, model.ErrInvalidInput
	}
	if err != nil {
		return model.Order{}, fmt.Errorf("create order: %w", err)
	}

	// A purchase in full has no installment rows; its one payment is the order's.
	if q.InstallmentOfferID != nil && len(q.Schedule) > 0 {
		p := gen.CreateInstallmentsParams{OrderID: id, WorkspaceID: workspaceID}
		for _, sp := range q.Schedule {
			p.Seqs = append(p.Seqs, int32(sp.Seq)) //nolint:gosec // at most 24 installments
			p.DueAts = append(p.DueAts, sp.DueAt)
			p.AmountsMinor = append(p.AmountsMinor, sp.AmountMinor)
		}
		if err := r.CreateInstallments(ctx, p); err != nil {
			return model.Order{}, fmt.Errorf("create installments: %w", err)
		}
	}

	o, err := getOrder(ctx, r, workspaceID, id)
	if err != nil {
		return model.Order{}, err
	}
	o.Installments, err = listInstallments(ctx, r, workspaceID, id)
	return o, err
}

// CompleteOrderCheckout applies a paid ltd or installment checkout in tx: the
// first payment makes the order in_installments (or paid) with a lifetime
// grant (Q1), cancels a replaced subscription through CancelForLTD (Q13) or
// upgrades the old order (Q16). Track T3's CompleteCheckout calls it.
func (s *Service) CompleteOrderCheckout(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, session model.CheckoutSession, p payment.Payment) error {
	return s.ltd().completeOrderCheckout(ctx, tx, workspaceID, session, p)
}

func (l lifetimeDeals) completeOrderCheckout(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, session model.CheckoutSession, p payment.Payment) error {
	if session.OrderID == nil {
		return fmt.Errorf("checkout %s has no order: %w", session.ID, model.ErrInvalidInput)
	}
	r := repository.NewRepository(tx)
	o, err := lockOrder(ctx, r, workspaceID, *session.OrderID)
	if err != nil {
		return err
	}

	switch session.Purpose {
	case model.PurposeLTD:
		return l.startOrder(ctx, tx, r, o, session, p)
	case model.PurposeInstallment:
		var inst *model.Installment
		if session.InstallmentID != nil {
			i, err := r.LockInstallment(ctx, gen.LockInstallmentParams{
				WorkspaceID: workspaceID, InstallmentID: *session.InstallmentID,
			})
			if err != nil {
				return fmt.Errorf("lock installment: %w", err)
			}
			if i.OrderID != o.ID {
				return fmt.Errorf("installment %s is not on order %s: %w", i.ID, o.ID, model.ErrInvalidInput)
			}
			v := installmentFromRow(gen.ListOrderInstallmentsRow(i))
			inst = &v
		}
		recorded, o, err := l.recordPayment(ctx, tx, r, o, inst, &session.ID, p)
		if err != nil || !recorded {
			return err
		}
		return l.settle(ctx, tx, r, o)
	default:
		return fmt.Errorf("checkout %s purpose %q is not an order's: %w", session.ID, session.Purpose, model.ErrInvalidInput)
	}
}

// startOrder applies a lifetime deal's first payment: the old subscription or
// order makes way, the payment is recorded, and the lifetime grant starts with
// full limits from installment 1 (Q1).
func (l lifetimeDeals) startOrder(ctx context.Context, tx pgx.Tx, r repository.Repository, o model.Order, session model.CheckoutSession, p payment.Payment) error {
	if o.Status != model.OrderPending {
		if o.Status == model.OrderCanceled {
			// Abandoned and canceled by the daily pass, then paid after all: needs a person.
			return fmt.Errorf("order %s was canceled before its first payment arrived", o.ID)
		}
		return nil // already applied
	}
	now := l.now()

	if o.UpgradedFromOrderID != nil {
		// Q16. Its grant ends when the new one starts.
		old, err := lockOrder(ctx, r, o.WorkspaceID, *o.UpgradedFromOrderID)
		if err != nil {
			return err
		}
		if old.Status != model.OrderInInstallments && old.Status != model.OrderPaid {
			return fmt.Errorf("upgraded order %s is %s: %w", old.ID, old.Status, model.ErrOrderNotUpgradable)
		}
		if err := setOrderStatus(ctx, r, old, model.OrderUpgraded, now); err != nil {
			return err
		}
		if _, err := r.CancelOpenInstallments(ctx, gen.CancelOpenInstallmentsParams{
			WorkspaceID: o.WorkspaceID, OrderID: old.ID,
		}); err != nil {
			return fmt.Errorf("cancel upgraded order's installments: %w", err)
		}
	}
	if o.ReplacedSubscriptionID != nil {
		// Q13. The credit was fixed on the order when it was quoted.
		if _, err := l.t3.cancelForLTD(ctx, tx, o.WorkspaceID); err != nil {
			return err
		}
	}

	var first *model.Installment
	insts, err := listInstallments(ctx, r, o.WorkspaceID, o.ID)
	if err != nil {
		return err
	}
	if len(insts) > 0 {
		first = &insts[0]
	}
	recorded, o, err := l.recordPayment(ctx, tx, r, o, first, &session.ID, p)
	if err != nil || !recorded {
		return err
	}
	if p.ExternalScheduleID != "" {
		if _, err := r.SetOrderExternalRef(ctx, gen.SetOrderExternalRefParams{
			ExternalRef: &p.ExternalScheduleID, WorkspaceID: o.WorkspaceID, OrderID: o.ID,
		}); err != nil {
			return fmt.Errorf("set order external ref: %w", err)
		}
	}

	status := model.OrderInInstallments
	if o.BalanceMinor() == 0 {
		status = model.OrderPaid
	}
	if err := setOrderStatus(ctx, r, o, status, now); err != nil {
		return err
	}

	price, err := l.t3.price(ctx, o.PlanPriceID)
	if err != nil {
		return err
	}
	_, err = l.t3.activateGrant(ctx, tx, o.WorkspaceID, model.GrantInput{
		PlanID: price.PlanID, Source: model.GrantFromOrder, OrderID: &o.ID, StartsAt: now,
		CreatedBy: session.CreatedBy,
	})
	return err
}

// recordPayment issues the paid invoice and moves paid_minor with it, after
// the caller locked the order (LockOrder), and marks inst paid. A payment
// already recorded (a replayed webhook) records nothing and returns false.
// Paying more than the balance is model.ErrOverpayment.
func (l lifetimeDeals) recordPayment(ctx context.Context, tx pgx.Tx, r repository.Repository, o model.Order, inst *model.Installment, sessionID *uuid.UUID, p payment.Payment) (bool, model.Order, error) {
	billTo, err := l.t3.billTo(ctx, tx, o.WorkspaceID)
	if err != nil {
		return false, o, err
	}
	rawBillTo, err := json.Marshal(billTo)
	if err != nil {
		return false, o, fmt.Errorf("encode bill_to: %w", err)
	}
	paidAt := l.now()
	if p.PaidAt != nil {
		paidAt = p.PaidAt.UTC()
	}
	var discountID *uuid.UUID
	if p.DiscountMinor > 0 {
		discountID = o.DiscountID
	}
	var installmentID *uuid.UUID
	if inst != nil {
		installmentID = &inst.ID
	}
	_, err = r.CreateInvoice(ctx, gen.CreateInvoiceParams{
		WorkspaceID: o.WorkspaceID, Number: nonEmpty(p.InvoiceNumber), OrderID: &o.ID, InstallmentID: installmentID,
		CheckoutSessionID: sessionID, Provider: string(o.Provider), ExternalPaymentID: nonEmpty(p.ExternalID),
		Status: model.InvoicePaid, SubtotalMinor: p.SubtotalMinor, TaxMinor: p.TaxMinor, TotalMinor: p.TotalMinor,
		Currency: p.Currency, PeriodStart: p.PeriodStart, PeriodEnd: p.PeriodEnd, BillTo: rawBillTo,
		DiscountID: discountID, DiscountMinor: p.DiscountMinor, ReceiptUrl: nonEmpty(p.ReceiptURL), PaidAt: &paidAt,
	})
	if repository.IsNoRows(err) {
		return false, o, nil
	}
	if err != nil {
		return false, o, fmt.Errorf("create invoice: %w", err)
	}

	// Tax belongs to the invoice, not the order's balance.
	if amount := p.TotalMinor - p.TaxMinor; amount > 0 {
		row, err := r.AddOrderPayment(ctx, gen.AddOrderPaymentParams{
			AmountMinor: amount, WorkspaceID: o.WorkspaceID, OrderID: o.ID,
		})
		if repository.IsCheck(err) {
			return false, o, model.ErrOverpayment
		}
		if err != nil {
			return false, o, fmt.Errorf("add order payment: %w", err)
		}
		o.PaidMinor = row.PaidMinor
	}

	if inst != nil {
		if _, err := r.MarkInstallmentPaid(ctx, gen.MarkInstallmentPaidParams{
			PaidAt: &paidAt, WorkspaceID: o.WorkspaceID, InstallmentID: inst.ID,
		}); err != nil {
			return false, o, fmt.Errorf("mark installment paid: %w", err)
		}
	}
	return true, o, nil
}

// settle brings a live order up to date after a later payment: paid once
// nothing is owed (a defaulted order resumes, Q12), and its grant restored
// once no installment is overdue (Q2).
func (l lifetimeDeals) settle(ctx context.Context, tx pgx.Tx, r repository.Repository, o model.Order) error {
	now := l.now()
	switch {
	case o.BalanceMinor() == 0:
		if err := setOrderStatus(ctx, r, o, model.OrderPaid, now); err != nil {
			return err
		}
		// Paying the whole balance at once covers what was still scheduled.
		if _, err := r.CancelOpenInstallments(ctx, gen.CancelOpenInstallmentsParams{
			WorkspaceID: o.WorkspaceID, OrderID: o.ID,
		}); err != nil {
			return fmt.Errorf("close paid order's installments: %w", err)
		}
	case o.Status == model.OrderInInstallments:
		missed, err := r.CountMissedInstallments(ctx, gen.CountMissedInstallmentsParams{
			WorkspaceID: o.WorkspaceID, OrderID: o.ID,
		})
		if err != nil {
			return fmt.Errorf("count missed installments: %w", err)
		}
		if missed > 0 {
			return nil
		}
	default:
		// A defaulted order stays suspended until its whole balance is paid.
		return nil
	}

	g, err := r.GetOrderGrant(ctx, gen.GetOrderGrantParams{WorkspaceID: o.WorkspaceID, OrderID: &o.ID})
	if repository.IsNoRows(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get order grant: %w", err)
	}
	if g.Status != model.GrantSuspended {
		return nil
	}
	return l.t3.restoreGrant(ctx, tx, o.WorkspaceID, g.ID)
}

// ResumeOrder starts a checkout for a defaulted order's remaining balance;
// paying it restores the lifetime grant (Q12).
func (s *Service) ResumeOrder(ctx context.Context, workspaceID, userID, orderID uuid.UUID, idempotencyKey string) (model.StartedCheckout, error) {
	return s.ltd().resumeOrder(ctx, workspaceID, userID, orderID, idempotencyKey)
}

func (l lifetimeDeals) resumeOrder(ctx context.Context, workspaceID, userID, orderID uuid.UUID, idempotencyKey string) (model.StartedCheckout, error) {
	r := repository.NewRepository(l.pool)
	o, err := getOrder(ctx, r, workspaceID, orderID)
	if err != nil {
		return model.StartedCheckout{}, err
	}
	if o.Status != model.OrderDefaulted {
		return model.StartedCheckout{}, model.ErrOrderNotDefaulted
	}
	insts, err := listInstallments(ctx, r, workspaceID, orderID)
	if err != nil {
		return model.StartedCheckout{}, err
	}
	// A checkout with purpose installment names an installment (a CHECK on
	// checkout_sessions), so the balance is paid against the earliest open
	// one; settle closes the rest.
	open := firstOpen(insts)
	if open == nil {
		return model.StartedCheckout{}, fmt.Errorf("defaulted order %s has no open installment", o.ID)
	}
	return l.t3.startPaymentCheckout(ctx, workspaceID, userID, model.PaymentCheckoutInput{
		OrderID: o.ID, InstallmentID: &open.ID, AmountMinor: o.BalanceMinor(), Currency: o.Currency,
		Provider: o.Provider, IdempotencyKey: idempotencyKey, Description: "Lifetime deal, remaining balance",
		ReturnURL: l.billingURL(workspaceID),
	})
}

// billingURL is the billing settings page of one workspace.
func (l lifetimeDeals) billingURL(workspaceID uuid.UUID) string {
	return l.cfg.AppBaseURL + "/settings/billing?workspace=" + workspaceID.String()
}

// firstOpen is the earliest installment still to be paid, or nil.
func firstOpen(insts []model.Installment) *model.Installment {
	for i := range insts {
		switch insts[i].Status {
		case model.InstallmentScheduled, model.InstallmentDue, model.InstallmentOverdue, model.InstallmentFailed:
			return &insts[i]
		}
	}
	return nil
}

func getOrder(ctx context.Context, r repository.Repository, workspaceID, orderID uuid.UUID) (model.Order, error) {
	row, err := r.GetOrder(ctx, gen.GetOrderParams{WorkspaceID: workspaceID, OrderID: orderID})
	if repository.IsNoRows(err) {
		return model.Order{}, model.ErrNoOrder
	}
	if err != nil {
		return model.Order{}, fmt.Errorf("get order: %w", err)
	}
	return orderFromRow(row), nil
}

func lockOrder(ctx context.Context, r repository.Repository, workspaceID, orderID uuid.UUID) (model.Order, error) {
	row, err := r.LockOrder(ctx, gen.LockOrderParams{WorkspaceID: workspaceID, OrderID: orderID})
	if repository.IsNoRows(err) {
		return model.Order{}, model.ErrNoOrder
	}
	if err != nil {
		return model.Order{}, fmt.Errorf("lock order: %w", err)
	}
	return orderFromRow(gen.GetOrderRow(row)), nil
}

func setOrderStatus(ctx context.Context, r repository.Repository, o model.Order, status string, now time.Time) error {
	if o.Status == status {
		return nil
	}
	if _, err := r.SetOrderStatus(ctx, gen.SetOrderStatusParams{
		Status: status, Now: now, WorkspaceID: o.WorkspaceID, OrderID: o.ID,
	}); err != nil {
		return fmt.Errorf("set order %s: %w", status, err)
	}
	return nil
}

func listInstallments(ctx context.Context, r repository.Repository, workspaceID, orderID uuid.UUID) ([]model.Installment, error) {
	rows, err := r.ListOrderInstallments(ctx, gen.ListOrderInstallmentsParams{
		WorkspaceID: workspaceID, OrderID: orderID,
	})
	if err != nil {
		return nil, fmt.Errorf("list installments: %w", err)
	}
	out := make([]model.Installment, len(rows))
	for i, row := range rows {
		out[i] = installmentFromRow(row)
	}
	return out, nil
}

func orderFromRow(r gen.GetOrderRow) model.Order {
	return model.Order{
		ID: r.ID, WorkspaceID: r.WorkspaceID, PlanPriceID: r.PlanPriceID, InstallmentOfferID: r.InstallmentOfferID,
		BillingCustomerID: r.BillingCustomerID, Provider: payment.Provider(r.Provider), ExternalRef: r.ExternalRef,
		Status: r.Status, TotalMinor: r.TotalMinor, DiscountID: r.DiscountID, DiscountMinor: r.DiscountMinor,
		CreditMinor: r.CreditMinor, PaidMinor: r.PaidMinor, Currency: r.Currency, PaidInFull: r.PaidInFullAt,
		DefaultedAt: r.DefaultedAt, UpgradedFromOrderID: r.UpgradedFromOrderID,
		ReplacedSubscriptionID: r.ReplacedSubscriptionID, CreatedAt: r.CreatedAt,
	}
}

func installmentFromRow(r gen.ListOrderInstallmentsRow) model.Installment {
	return model.Installment{
		ID: r.ID, OrderID: r.OrderID, Seq: int(r.Seq), DueAt: r.DueAt, AmountMinor: r.AmountMinor,
		Status: r.Status, PaidAt: r.PaidAt, ReminderSentAt: r.ReminderSentAt,
	}
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
