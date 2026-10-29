package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Track T4: installments.

// installmentBatch is how many installments the daily pass reads at a time.
const installmentBatch = 200

// PayInstallment starts a checkout for one due or overdue app_link
// installment (Pay now), through StartPaymentCheckout.
func (s *Service) PayInstallment(ctx context.Context, workspaceID, userID, orderID, installmentID uuid.UUID, idempotencyKey string) (model.StartedCheckout, error) {
	return s.ltd().payInstallment(ctx, workspaceID, userID, orderID, installmentID, idempotencyKey)
}

func (l lifetimeDeals) payInstallment(ctx context.Context, workspaceID, userID, orderID, installmentID uuid.UUID, idempotencyKey string) (model.StartedCheckout, error) {
	r := repository.NewRepository(l.pool)
	o, err := getOrder(ctx, r, workspaceID, orderID)
	if err != nil {
		return model.StartedCheckout{}, err
	}
	insts, err := listInstallments(ctx, r, workspaceID, orderID)
	if err != nil {
		return model.StartedCheckout{}, err
	}
	var inst *model.Installment
	for i := range insts {
		if insts[i].ID == installmentID {
			inst = &insts[i]
		}
	}
	if inst == nil {
		return model.StartedCheckout{}, model.ErrInstallmentNotFound
	}
	// A defaulted order resumes through ResumeOrder; installments are paid in order.
	if o.Status != model.OrderInInstallments || inst.Status == model.InstallmentScheduled ||
		firstOpen(insts) != inst {
		return model.StartedCheckout{}, model.ErrInstallmentNotDue
	}
	coll, err := l.collection(ctx, o)
	if err != nil {
		return model.StartedCheckout{}, err
	}
	if coll != model.CollectionAppLink {
		// The provider collects it; paying here as well would charge twice.
		return model.StartedCheckout{}, model.ErrInstallmentNotDue
	}
	return l.t3.startPaymentCheckout(ctx, workspaceID, userID, installmentCheckout(o, *inst, len(insts), idempotencyKey,
		l.billingURL(workspaceID)))
}

func installmentCheckout(o model.Order, inst model.Installment, count int, idempotencyKey, returnURL string) model.PaymentCheckoutInput {
	return model.PaymentCheckoutInput{
		OrderID: o.ID, InstallmentID: &inst.ID, AmountMinor: inst.AmountMinor, Currency: o.Currency,
		Provider: o.Provider, IdempotencyKey: idempotencyKey,
		Description: fmt.Sprintf("Lifetime deal, installment %d of %d", inst.Seq, count), ReturnURL: returnURL,
	}
}

// collection is how the order's provider collects its installments. When the
// offer is gone, an order with a provider schedule ID is provider_schedule,
// and any other is app_link.
func (l lifetimeDeals) collection(ctx context.Context, o model.Order) (string, error) {
	if o.InstallmentOfferID == nil {
		return "", nil
	}
	offer, err := l.t3.offer(ctx, *o.InstallmentOfferID)
	if err != nil && !errors.Is(err, model.ErrOfferNotFound) {
		return "", err
	}
	for _, p := range offer.Providers {
		if p.Provider == o.Provider {
			return p.Collection, nil
		}
	}
	if o.ExternalRef != nil {
		return model.CollectionProviderSchedule, nil
	}
	return model.CollectionAppLink, nil
}

// ApplyInstallmentPayment records a provider-schedule installment payment in tx
// (payment.succeeded or payment.failed naming the order's external_ref): the
// invoice, paid_minor, the installment's status, the grant restored after a
// late payment, and the order paid after the last one.
func (s *Service) ApplyInstallmentPayment(ctx context.Context, tx pgx.Tx, workspaceID, orderID uuid.UUID, p payment.Payment, paid bool) error {
	return s.ltd().applyInstallmentPayment(ctx, tx, workspaceID, orderID, p, paid)
}

func (l lifetimeDeals) applyInstallmentPayment(ctx context.Context, tx pgx.Tx, workspaceID, orderID uuid.UUID, p payment.Payment, paid bool) error {
	r := repository.NewRepository(tx)
	o, err := lockOrder(ctx, r, workspaceID, orderID)
	if err != nil {
		return err
	}
	switch o.Status {
	case model.OrderInInstallments, model.OrderDefaulted, model.OrderPaid:
	default:
		return fmt.Errorf("installment payment for order %s in status %s", o.ID, o.Status)
	}
	insts, err := listInstallments(ctx, r, workspaceID, orderID)
	if err != nil {
		return err
	}
	inst := firstOpen(insts)

	if !paid {
		// The provider retries. Mark it due so the daily pass applies the
		// offer's grace days before it counts as missed (Q2).
		if inst != nil && inst.Status == model.InstallmentScheduled {
			if _, err := r.SetInstallmentStatus(ctx, gen.SetInstallmentStatusParams{
				Status: model.InstallmentDue, WorkspaceID: workspaceID, InstallmentID: inst.ID,
			}); err != nil {
				return fmt.Errorf("mark installment due: %w", err)
			}
		}
		return nil
	}

	recorded, o, err := l.recordPayment(ctx, tx, r, o, inst, nil, p)
	if err != nil || !recorded {
		return err
	}
	return l.settle(ctx, tx, r, o)
}

// RunInstallmentsDue is the daily pass (Q2): mark scheduled installments due
// and remind (creating app_link pay links), mark unpaid ones overdue after
// grace_days and suspend the grant, and default orders after max_missed. Run
// by the InstallmentsDue worker.
func (s *Service) RunInstallmentsDue(ctx context.Context) (model.InstallmentRun, error) {
	return s.ltd().runInstallmentsDue(ctx)
}

func (l lifetimeDeals) runInstallmentsDue(ctx context.Context) (model.InstallmentRun, error) {
	var run model.InstallmentRun
	var errs []error
	now := l.now()
	r := repository.NewRepository(l.pool)

	// Each pass changes the status of what it handles, so the next batch is
	// new rows. It stops when a batch moves nothing (young pending orders).
	for {
		rows, err := r.ListInstallmentsComingDue(ctx, gen.ListInstallmentsComingDueParams{
			Now: now, MaxRows: installmentBatch,
		})
		if err != nil {
			return run, fmt.Errorf("list installments coming due: %w", err)
		}
		moved := 0
		for _, row := range rows {
			changed, err := l.comeDue(ctx, row, now, &run)
			if err != nil {
				l.log.ErrorContext(ctx, "installment coming due", "installment_id", row.ID, "error", err)
				errs = append(errs, err)
			}
			if changed {
				moved++
			}
		}
		if len(rows) < installmentBatch || moved == 0 {
			break
		}
	}

	for {
		rows, err := r.ListInstallmentsPastGrace(ctx, gen.ListInstallmentsPastGraceParams{
			Now: now, MaxRows: installmentBatch,
		})
		if err != nil {
			return run, fmt.Errorf("list installments past grace: %w", err)
		}
		moved := 0
		for _, row := range rows {
			if err := l.pastGrace(ctx, row, now, &run); err != nil {
				l.log.ErrorContext(ctx, "installment past grace", "installment_id", row.ID, "error", err)
				errs = append(errs, err)
				continue
			}
			moved++
		}
		if len(rows) < installmentBatch || moved == 0 {
			break
		}
	}
	return run, errors.Join(errs...)
}

// comeDue marks one scheduled installment due and reminds the customer. An
// order that is no longer live gets its open installments canceled instead,
// and one whose checkout was abandoned long ago is canceled with them.
func (l lifetimeDeals) comeDue(ctx context.Context, row gen.ListInstallmentsComingDueRow, now time.Time, run *model.InstallmentRun) (bool, error) {
	var o model.Order
	var changed, markedDue, remind bool
	err := database.WithTx(ctx, l.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		var err error
		if o, err = lockOrder(ctx, r, row.WorkspaceID, row.OrderID); err != nil {
			return err
		}
		switch o.Status {
		case model.OrderInInstallments, model.OrderDefaulted:
			n, err := r.SetInstallmentStatus(ctx, gen.SetInstallmentStatusParams{
				Status: model.InstallmentDue, WorkspaceID: row.WorkspaceID, InstallmentID: row.ID,
			})
			if err != nil {
				return fmt.Errorf("mark installment due: %w", err)
			}
			changed, markedDue = n > 0, n > 0
			remind = markedDue && o.Status == model.OrderInInstallments
			return nil
		case model.OrderPending:
			if now.Sub(o.CreatedAt) < abandonedOrderAge {
				return nil // its checkout may still complete
			}
			if err := setOrderStatus(ctx, r, o, model.OrderCanceled, now); err != nil {
				return err
			}
		}
		n, err := r.CancelOpenInstallments(ctx, gen.CancelOpenInstallmentsParams{
			WorkspaceID: row.WorkspaceID, OrderID: row.OrderID,
		})
		if err != nil {
			return fmt.Errorf("cancel open installments: %w", err)
		}
		changed = n > 0
		return nil
	})
	if err != nil {
		return false, err
	}
	if markedDue {
		run.MarkedDue++
	}
	if !remind {
		return changed, nil
	}
	if err := l.remind(ctx, o, row, now); err != nil {
		return true, err
	}
	run.Reminded++
	return true, nil
}

// remind gives an app_link installment its pay link and stamps the reminder.
func (l lifetimeDeals) remind(ctx context.Context, o model.Order, row gen.ListInstallmentsComingDueRow, now time.Time) error {
	coll, err := l.collection(ctx, o)
	if err != nil {
		return err
	}
	if coll == model.CollectionAppLink {
		r := repository.NewRepository(l.pool)
		insts, err := listInstallments(ctx, r, o.WorkspaceID, o.ID)
		if err != nil {
			return err
		}
		inst := model.Installment{ID: row.ID, Seq: int(row.Seq), AmountMinor: row.AmountMinor}
		// No user starts it: the daily pass does (created_by stays empty).
		if _, err := l.t3.startPaymentCheckout(ctx, o.WorkspaceID, uuid.Nil, installmentCheckout(o, inst, len(insts),
			"installment-due-"+row.ID.String(), l.billingURL(o.WorkspaceID))); err != nil {
			return err
		}
	}
	if _, err := repository.NewRepository(l.pool).SetInstallmentReminderSent(ctx, gen.SetInstallmentReminderSentParams{
		Now: &now, WorkspaceID: o.WorkspaceID, InstallmentID: row.ID,
	}); err != nil {
		return fmt.Errorf("stamp installment reminder: %w", err)
	}
	l.log.InfoContext(ctx, "installment reminder", "workspace_id", o.WorkspaceID, "order_id", o.ID,
		"installment_id", row.ID, "collection", coll)
	return nil
}

// pastGrace marks one due installment overdue, suspends the lifetime grant
// and defaults the order once max_missed installments are missed (Q2).
func (l lifetimeDeals) pastGrace(ctx context.Context, row gen.ListInstallmentsPastGraceRow, now time.Time, run *model.InstallmentRun) error {
	var overdue, suspended, defaulted bool
	err := database.WithTx(ctx, l.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		o, err := lockOrder(ctx, r, row.WorkspaceID, row.OrderID)
		if err != nil {
			return err
		}
		if o.Status != model.OrderInInstallments && o.Status != model.OrderDefaulted {
			if _, err := r.CancelOpenInstallments(ctx, gen.CancelOpenInstallmentsParams{
				WorkspaceID: row.WorkspaceID, OrderID: row.OrderID,
			}); err != nil {
				return fmt.Errorf("cancel open installments: %w", err)
			}
			return nil
		}
		n, err := r.SetInstallmentStatus(ctx, gen.SetInstallmentStatusParams{
			Status: model.InstallmentOverdue, WorkspaceID: row.WorkspaceID, InstallmentID: row.ID,
		})
		if err != nil {
			return fmt.Errorf("mark installment overdue: %w", err)
		}
		overdue = n > 0
		if o.Status == model.OrderDefaulted {
			return nil // already suspended
		}

		g, err := r.GetOrderGrant(ctx, gen.GetOrderGrantParams{WorkspaceID: row.WorkspaceID, OrderID: &o.ID})
		switch {
		case err == nil && g.Status == model.GrantActive:
			if err := l.t3.suspendGrant(ctx, tx, row.WorkspaceID, g.ID); err != nil {
				return err
			}
			suspended = true
		case err != nil && !repository.IsNoRows(err):
			return fmt.Errorf("get order grant: %w", err)
		}

		missed, err := r.CountMissedInstallments(ctx, gen.CountMissedInstallmentsParams{
			WorkspaceID: row.WorkspaceID, OrderID: o.ID,
		})
		if err != nil {
			return fmt.Errorf("count missed installments: %w", err)
		}
		if missed >= row.MaxMissed {
			if err := setOrderStatus(ctx, r, o, model.OrderDefaulted, now); err != nil {
				return err
			}
			defaulted = true
		}
		return nil
	})
	if err != nil {
		return err
	}
	if overdue {
		run.Overdue++
	}
	if suspended {
		run.Suspended++
	}
	if defaulted {
		run.Defaulted++
	}
	return nil
}
