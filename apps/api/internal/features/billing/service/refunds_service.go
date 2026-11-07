package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Refunds Relay asks for: recorded first, sent after the commit.

// requestRefund records a refund of the whole payment an invoice holds, in
// tx, and queues IssueRefund for after the commit. The provider is only ever
// asked for a refund already on record, so a retried or replayed event never
// asks twice. The invoice counts the refund (shows refunded) from now on.
func (s *Service) requestRefund(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, provider payment.Provider, externalPaymentID, reason string) error {
	repo := repository.NewRepository(tx)
	inv, ok, err := lockedInvoice(ctx, repo, provider, workspaceID, externalPaymentID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("refund: no invoice of workspace %s records payment %s", workspaceID, externalPaymentID)
	}
	if inv.TotalMinor <= 0 {
		return nil
	}
	id, err := repo.RequestRefund(ctx, gen.RequestRefundParams{
		InvoiceID: inv.ID, WorkspaceID: workspaceID, Provider: string(provider), AmountMinor: inv.TotalMinor, Reason: &reason,
	})
	if err != nil {
		return fmt.Errorf("request refund: %w", err)
	}
	if _, err := s.queue.InsertTx(ctx, tx, model.IssueRefundArgs{WorkspaceID: workspaceID, RefundID: id}, nil); err != nil {
		return fmt.Errorf("enqueue refund: %w", err)
	}
	_, err = s.syncRefundedStatus(ctx, repo, workspaceID, inv)
	return err
}

// IssueRefund asks the provider for a refund requestRefund recorded. One
// already issued, named by the provider's webhook, or failed is left alone.
// A crash after the call can't refund twice either: Stripe reuses the
// refund's idempotency key, and Creem's gateway takes a transaction it
// already refunded as done. A refusal for good marks the refund failed, puts
// the invoice back to what its other refunds make it, and is logged once;
// anything else is retried. Run by the IssueRefund worker.
func (s *Service) IssueRefund(ctx context.Context, workspaceID, refundID uuid.UUID) error {
	repo := repository.NewRepository(s.pool)
	r, err := repo.GetRefundToIssue(ctx, gen.GetRefundToIssueParams{WorkspaceID: workspaceID, RefundID: refundID})
	if repository.IsNoRows(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get refund: %w", err)
	}
	if r.IssuedAt != nil || r.ExternalRefundID != nil || r.Status == string(payment.RefundFailed) || r.ExternalPaymentID == nil {
		return nil
	}
	provider := payment.Provider(r.Provider)
	g, err := s.gateway(provider)
	if err != nil {
		return err
	}
	out, err := g.Refund(ctx, payment.RefundRequest{
		ExternalPaymentID: *r.ExternalPaymentID, AmountMinor: r.AmountMinor, Currency: r.Currency,
		Reason: deref(r.Reason), IdempotencyKey: "refund:" + r.ID.String(),
	})
	switch {
	case err == nil:
		if _, err := repo.MarkRefundIssued(ctx, gen.MarkRefundIssuedParams{
			IssuedAt: new(s.now()), ExternalRefundID: nilIfEmpty(out.ExternalID), WorkspaceID: workspaceID, RefundID: r.ID,
		}); err != nil {
			return fmt.Errorf("mark refund issued: %w", err)
		}
		return nil
	case payment.Permanent(err), errors.Is(err, payment.ErrNotFound):
		s.logger().ErrorContext(ctx, "billing: payment provider refused a refund; needs a person",
			"workspace_id", workspaceID, "refund_id", r.ID, "invoice_id", r.InvoiceID, "err", err)
		return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
			repo := repository.NewRepository(tx)
			inv, ok, err := lockedInvoice(ctx, repo, provider, workspaceID, *r.ExternalPaymentID)
			if err != nil {
				return err
			}
			n, err := repo.FailRequestedRefund(ctx, gen.FailRequestedRefundParams{WorkspaceID: workspaceID, RefundID: r.ID})
			if err != nil {
				return fmt.Errorf("fail refund: %w", err)
			}
			if n == 0 || !ok {
				return nil // the provider's webhook named it meanwhile
			}
			_, err = s.syncRefundedStatus(ctx, repo, workspaceID, inv)
			return err
		})
	default:
		return fmt.Errorf("issue refund %s: %w", r.ID, err)
	}
}
