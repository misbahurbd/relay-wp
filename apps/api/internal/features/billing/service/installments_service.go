package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Track T4: installments.

// PayInstallment starts a checkout for one due or overdue app_link
// installment (Pay now), through StartPaymentCheckout.
//
//nolint:revive // stub: track T4 builds it
func (s *Service) PayInstallment(ctx context.Context, workspaceID, userID, orderID, installmentID uuid.UUID, idempotencyKey string) (model.StartedCheckout, error) {
	return model.StartedCheckout{}, model.ErrNotImplemented
}

// ApplyInstallmentPayment records a provider-schedule installment payment in tx
// (payment.succeeded or payment.failed naming the order's external_ref): the
// invoice, paid_minor, the installment's status, the grant restored after a
// late payment, and the order paid after the last one.
//
//nolint:revive // stub: track T4 builds it
func (s *Service) ApplyInstallmentPayment(ctx context.Context, tx pgx.Tx, workspaceID, orderID uuid.UUID, p payment.Payment, paid bool) error {
	return model.ErrNotImplemented
}

// RunInstallmentsDue is the daily pass (Q2): mark scheduled installments due
// and remind (creating app_link pay links), mark unpaid ones overdue after
// grace_days and suspend the grant, and default orders after max_missed. Run
// by the InstallmentsDue worker.
//
//nolint:revive // stub: track T4 builds it
func (s *Service) RunInstallmentsDue(ctx context.Context) (model.InstallmentRun, error) {
	return model.InstallmentRun{}, model.ErrNotImplemented
}
