package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Track T5: webhooks and the jobs that keep billing tidy.

const (
	// unmatchedRetryWindow is how long an event that names nothing we stored
	// is retried. Providers send events out of order (a subscription update
	// before the checkout that stores its ID); after this it is someone else's
	// object, so it is logged and marked processed.
	unmatchedRetryWindow = 24 * time.Hour
	// purgeBatch is how many payloads one PurgeEventPayloads statement nulls.
	purgeBatch = 1000
	// driftBatch caps how many drifted workspaces one CheckPlanCacheDrift pass logs.
	driftBatch = 1000
)

// eventHandlers maps each normalised event type to the rules that apply it,
// calling into the T3 and T4 methods that take a pgx.Tx. New calls it once.
func (s *Service) eventHandlers() map[payment.EventType]eventHandler {
	return map[payment.EventType]eventHandler{
		payment.EventCheckoutCompleted:    s.onCheckoutCompleted,
		payment.EventCheckoutFailed:       s.onCheckoutEnded(model.CheckoutFailed),
		payment.EventCheckoutExpired:      s.onCheckoutEnded(model.CheckoutExpired),
		payment.EventPaymentSucceeded:     s.onPayment(true),
		payment.EventPaymentFailed:        s.onPayment(false),
		payment.EventSubscriptionUpdated:  s.onSubscription,
		payment.EventSubscriptionCanceled: s.onSubscription,
		payment.EventRefundUpdated:        s.onRefund,
		payment.EventDisputeOpened:        s.onDisputeOpened,
		payment.EventDisputeClosed:        s.onDisputeClosed,
	}
}

// ReceiveWebhook verifies a webhook with the provider's gateway, stores it in
// payment_events and enqueues ProcessPaymentEventArgs in one transaction, then
// returns so the handler can answer 200 quickly. A replay (same provider event
// ID) is a no-op. A bad signature returns model.ErrWebhookSignature and stores
// nothing; a provider that isn't enabled returns model.ErrProviderNotAvailable.
// header is the request's headers (http.Header).
func (s *Service) ReceiveWebhook(ctx context.Context, provider string, header map[string][]string, body []byte) error {
	p, ok := payment.ParseProvider(provider)
	if !ok {
		return model.ErrProviderNotAvailable
	}
	g, err := s.gateway(p)
	if err != nil {
		return err
	}
	ev, err := g.ParseWebhook(ctx, http.Header(header), body)
	switch {
	case errors.Is(err, payment.ErrInvalidSignature):
		s.log.WarnContext(ctx, "billing: webhook signature rejected", "provider", string(p))
		return model.ErrWebhookSignature
	case errors.Is(err, payment.ErrInvalidRequest):
		return model.ErrInvalidInput
	case err != nil:
		return fmt.Errorf("parse %s webhook: %w", p, err)
	}
	ev.Provider = p
	_, handled := s.events[ev.Type]

	var stored bool
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		repo := repository.NewRepository(tx)
		id, err := repo.InsertPaymentEvent(ctx, gen.InsertPaymentEventParams{
			Provider: string(p), ExternalEventID: ev.ExternalID, EventType: string(ev.Type), Payload: ev.Payload,
		})
		if repository.IsNoRows(err) {
			return nil // a replay: stored the first time
		}
		if err != nil {
			return fmt.Errorf("insert payment event: %w", err)
		}
		stored = true
		if !handled {
			// Verified but nothing Relay acts on: no job, processed now.
			return markProcessed(ctx, repo, id, s.now())
		}
		args := model.ProcessPaymentEventArgs{PaymentEventID: id, Event: ev}
		args.Event.Payload = nil // never in a job; it stays in payment_events
		if _, err := s.queue.InsertTx(ctx, tx, args, nil); err != nil {
			return fmt.Errorf("enqueue payment event: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if stored {
		s.log.InfoContext(ctx, "billing: webhook received", "event", ev)
	} else {
		s.log.InfoContext(ctx, "billing: webhook replay ignored", "event", ev)
	}
	return nil
}

// ProcessPaymentEvent applies one stored event: it finds the workspace through
// the event's reference (our checkout session ID) or a provider ID we stored,
// dispatches to the event's handler, and sets processed_at, all in one
// transaction. Already processed events are skipped. The ProcessPaymentEvent
// worker calls it; River retries on error.
func (s *Service) ProcessPaymentEvent(ctx context.Context, paymentEventID uuid.UUID, ev payment.Event) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		repo := repository.NewRepository(tx)
		// Setting processed_at first locks the row, so a second worker on the
		// same event waits here and then finds it done. An error below rolls it back.
		n, err := repo.MarkPaymentEventProcessed(ctx, gen.MarkPaymentEventProcessedParams{
			Now: new(s.now()), PaymentEventID: paymentEventID,
		})
		if err != nil {
			return fmt.Errorf("mark payment event processed: %w", err)
		}
		if n == 0 {
			return nil // already processed, or gone
		}

		handle, ok := s.events[ev.Type]
		if !ok {
			s.log.InfoContext(ctx, "billing: payment event ignored", "event", ev)
			return nil
		}
		workspaceID, sessionID, err := s.findEventWorkspace(ctx, repo, ev)
		if err != nil {
			return err
		}
		if workspaceID == uuid.Nil {
			if age := s.now().Sub(ev.OccurredAt); age < unmatchedRetryWindow {
				return fmt.Errorf("payment event %s matches nothing yet; retrying", paymentEventID)
			}
			s.log.WarnContext(ctx, "billing: payment event matches no workspace", "event", ev)
			return nil
		}
		if _, err := repo.SetPaymentEventWorkspace(ctx, gen.SetPaymentEventWorkspaceParams{
			WorkspaceID: &workspaceID, CheckoutSessionID: sessionID, PaymentEventID: paymentEventID,
		}); err != nil {
			return fmt.Errorf("set payment event workspace: %w", err)
		}
		if err := handle(ctx, tx, workspaceID, ev); err != nil {
			return fmt.Errorf("apply %s: %w", ev.Type, err)
		}
		s.log.InfoContext(ctx, "billing: payment event applied", "event", ev, "workspace_id", workspaceID)
		return nil
	})
}

func markProcessed(ctx context.Context, repo repository.Repository, id uuid.UUID, now time.Time) error {
	if _, err := repo.MarkPaymentEventProcessed(ctx, gen.MarkPaymentEventProcessedParams{
		Now: &now, PaymentEventID: id,
	}); err != nil {
		return fmt.Errorf("mark payment event processed: %w", err)
	}
	return nil
}

// findEventWorkspace finds the workspace an event belongs to, only through IDs
// we created or stored: our checkout session ID first, then the provider's
// subscription, schedule and payment IDs. uuid.Nil means nothing matched.
// sessionID is set when the event's reference is one of our checkout sessions.
func (s *Service) findEventWorkspace(ctx context.Context, repo repository.Repository, ev payment.Event) (workspaceID uuid.UUID, sessionID *uuid.UUID, err error) {
	provider := string(ev.Provider)
	if ev.Reference != nil {
		ws, err := repo.FindCheckoutSessionWorkspace(ctx, gen.FindCheckoutSessionWorkspaceParams{
			CheckoutSessionID: *ev.Reference, Provider: provider,
		})
		switch {
		case err == nil:
			return ws, ev.Reference, nil
		case !repository.IsNoRows(err):
			return uuid.Nil, nil, fmt.Errorf("find checkout session: %w", err)
		}
	}
	if id := eventSubscriptionID(ev); id != "" {
		row, err := repo.FindSubscriptionByExternalID(ctx, gen.FindSubscriptionByExternalIDParams{
			Provider: provider, ExternalSubscriptionID: &id,
		})
		switch {
		case err == nil:
			return row.WorkspaceID, nil, nil
		case !repository.IsNoRows(err):
			return uuid.Nil, nil, fmt.Errorf("find subscription: %w", err)
		}
	}
	if ev.Payment != nil && ev.Payment.ExternalScheduleID != "" {
		row, err := repo.FindOrderByExternalRef(ctx, gen.FindOrderByExternalRefParams{
			Provider: provider, ExternalRef: &ev.Payment.ExternalScheduleID,
		})
		switch {
		case err == nil:
			return row.WorkspaceID, nil, nil
		case !repository.IsNoRows(err):
			return uuid.Nil, nil, fmt.Errorf("find order: %w", err)
		}
	}
	if id := eventPaymentID(ev); id != "" {
		row, err := repo.FindInvoiceByExternalPaymentID(ctx, gen.FindInvoiceByExternalPaymentIDParams{
			Provider: provider, ExternalPaymentID: &id,
		})
		switch {
		case err == nil:
			return row.WorkspaceID, nil, nil
		case !repository.IsNoRows(err):
			return uuid.Nil, nil, fmt.Errorf("find invoice: %w", err)
		}
	}
	return uuid.Nil, nil, nil
}

// eventSubscriptionID is the provider subscription an event names, if any.
func eventSubscriptionID(ev payment.Event) string {
	if ev.Subscription != nil && ev.Subscription.ExternalID != "" {
		return ev.Subscription.ExternalID
	}
	if ev.Payment != nil {
		return ev.Payment.ExternalSubscriptionID
	}
	return ""
}

// eventPaymentID is the provider payment an event names, if any.
func eventPaymentID(ev payment.Event) string {
	if ev.Refund != nil && ev.Refund.ExternalPaymentID != "" {
		return ev.Refund.ExternalPaymentID
	}
	if ev.Payment != nil {
		return ev.Payment.ExternalID
	}
	return ""
}

// ---------- Dispatch to T3 and T4 ----------

func (s *Service) onCheckoutCompleted(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, ev payment.Event) error {
	if ev.Reference == nil {
		s.log.WarnContext(ctx, "billing: checkout event without a reference", "event", ev)
		return nil
	}
	return s.CompleteCheckout(ctx, tx, workspaceID, *ev.Reference, ev)
}

func (s *Service) onCheckoutEnded(status string) eventHandler {
	return func(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, ev payment.Event) error {
		if ev.Reference == nil {
			s.log.WarnContext(ctx, "billing: checkout event without a reference", "event", ev)
			return nil
		}
		return s.FailCheckout(ctx, tx, workspaceID, *ev.Reference, status)
	}
}

// onPayment applies a later payment: a renewal of a subscription, or an
// installment of a provider-run schedule.
func (s *Service) onPayment(paid bool) eventHandler {
	return func(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, ev payment.Event) error {
		if ev.Payment == nil {
			s.log.WarnContext(ctx, "billing: payment event without a payment", "event", ev)
			return nil
		}
		repo := repository.NewRepository(tx)
		provider := string(ev.Provider)
		if id := ev.Payment.ExternalSubscriptionID; id != "" {
			row, err := repo.FindSubscriptionByExternalID(ctx, gen.FindSubscriptionByExternalIDParams{
				Provider: provider, ExternalSubscriptionID: &id,
			})
			if err == nil && row.WorkspaceID == workspaceID {
				return s.ApplySubscriptionPayment(ctx, tx, workspaceID, row.ID, *ev.Payment, paid)
			}
			if err != nil && !repository.IsNoRows(err) {
				return fmt.Errorf("find subscription: %w", err)
			}
		}
		if id := ev.Payment.ExternalScheduleID; id != "" {
			row, err := repo.FindOrderByExternalRef(ctx, gen.FindOrderByExternalRefParams{
				Provider: provider, ExternalRef: &id,
			})
			if err == nil && row.WorkspaceID == workspaceID {
				return s.ApplyInstallmentPayment(ctx, tx, workspaceID, row.ID, *ev.Payment, paid)
			}
			if err != nil && !repository.IsNoRows(err) {
				return fmt.Errorf("find order: %w", err)
			}
		}
		s.log.WarnContext(ctx, "billing: payment names no subscription or order of the workspace",
			"event", ev, "workspace_id", workspaceID)
		return nil
	}
}

func (s *Service) onSubscription(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, ev payment.Event) error {
	if ev.Subscription == nil || ev.Subscription.ExternalID == "" {
		s.log.WarnContext(ctx, "billing: subscription event without a subscription", "event", ev)
		return nil
	}
	row, err := repository.NewRepository(tx).FindSubscriptionByExternalID(ctx, gen.FindSubscriptionByExternalIDParams{
		Provider: string(ev.Provider), ExternalSubscriptionID: &ev.Subscription.ExternalID,
	})
	if repository.IsNoRows(err) || (err == nil && row.WorkspaceID != workspaceID) {
		s.log.WarnContext(ctx, "billing: subscription event names no subscription of the workspace",
			"event", ev, "workspace_id", workspaceID)
		return nil
	}
	if err != nil {
		return fmt.Errorf("find subscription: %w", err)
	}
	return s.SyncSubscription(ctx, tx, workspaceID, row.ID, *ev.Subscription)
}

// ---------- Refunds and disputes (T5's own) ----------

// lockedInvoice finds the invoice a provider payment ID names and locks it, so
// refunds and disputes on one invoice apply one at a time. ok is false when
// the workspace has no such invoice.
func lockedInvoice(ctx context.Context, repo repository.Repository, provider payment.Provider, workspaceID uuid.UUID, externalPaymentID string) (gen.FindInvoiceByExternalPaymentIDRow, bool, error) {
	find := func() (gen.FindInvoiceByExternalPaymentIDRow, error) {
		return repo.FindInvoiceByExternalPaymentID(ctx, gen.FindInvoiceByExternalPaymentIDParams{
			Provider: string(provider), ExternalPaymentID: &externalPaymentID,
		})
	}
	inv, err := find()
	if repository.IsNoRows(err) || (err == nil && inv.WorkspaceID != workspaceID) {
		return inv, false, nil
	}
	if err != nil {
		return inv, false, fmt.Errorf("find invoice: %w", err)
	}
	// Writing the status it already has takes the row lock; then read it again,
	// as another refund may have changed it while we waited.
	if _, err := repo.SetInvoiceStatus(ctx, gen.SetInvoiceStatusParams{
		Status: inv.Status, WorkspaceID: workspaceID, InvoiceID: inv.ID,
	}); err != nil {
		return inv, false, fmt.Errorf("lock invoice: %w", err)
	}
	if inv, err = find(); err != nil {
		return inv, false, fmt.Errorf("find invoice: %w", err)
	}
	return inv, true, nil
}

// onRefund records a refund the provider reports, created here or in its
// dashboard, and moves the invoice to partially_refunded or refunded. Refunds
// never add up to more than the invoice total.
func (s *Service) onRefund(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, ev payment.Event) error {
	r := ev.Refund
	if r == nil || r.ExternalID == "" || r.ExternalPaymentID == "" {
		s.log.WarnContext(ctx, "billing: refund event without a refund", "event", ev)
		return nil
	}
	repo := repository.NewRepository(tx)
	inv, ok, err := lockedInvoice(ctx, repo, ev.Provider, workspaceID, r.ExternalPaymentID)
	if err != nil {
		return err
	}
	if !ok {
		s.log.WarnContext(ctx, "billing: refund names no invoice of the workspace", "event", ev, "workspace_id", workspaceID)
		return nil
	}

	n, err := repo.SetRefundStatus(ctx, gen.SetRefundStatusParams{
		Status: string(r.Status), WorkspaceID: workspaceID, Provider: string(ev.Provider), ExternalRefundID: &r.ExternalID,
	})
	if err != nil {
		return fmt.Errorf("set refund status: %w", err)
	}
	if n == 0 {
		if err := s.createRefund(ctx, repo, workspaceID, inv, ev); err != nil {
			return err
		}
	}
	return s.syncRefundedStatus(ctx, repo, workspaceID, inv)
}

// createRefund records a refund we haven't seen, unless it would take the
// invoice's refunds past its total.
func (s *Service) createRefund(ctx context.Context, repo repository.Repository, workspaceID uuid.UUID, inv gen.FindInvoiceByExternalPaymentIDRow, ev payment.Event) error {
	r := ev.Refund
	if r.AmountMinor <= 0 {
		s.log.WarnContext(ctx, "billing: refund without an amount", "event", ev, "workspace_id", workspaceID)
		return nil
	}
	if r.Status != payment.RefundFailed {
		refunded, err := repo.SumInvoiceRefunds(ctx, gen.SumInvoiceRefundsParams{WorkspaceID: workspaceID, InvoiceID: inv.ID})
		if err != nil {
			return fmt.Errorf("sum invoice refunds: %w", err)
		}
		if refunded+r.AmountMinor > inv.TotalMinor {
			s.log.ErrorContext(ctx, "billing: refund is more than the invoice's remaining total; not recorded",
				"event", ev, "workspace_id", workspaceID, "invoice_id", inv.ID,
				"amount_minor", r.AmountMinor, "refunded_minor", refunded, "total_minor", inv.TotalMinor)
			return nil
		}
	}
	if _, err := repo.CreateRefund(ctx, gen.CreateRefundParams{
		InvoiceID: inv.ID, WorkspaceID: workspaceID, Provider: string(ev.Provider), ExternalRefundID: &r.ExternalID,
		AmountMinor: r.AmountMinor, Status: string(r.Status),
	}); err != nil && !repository.IsNoRows(err) {
		return fmt.Errorf("create refund: %w", err)
	}
	return nil
}

// syncRefundedStatus sets a paid invoice's status from what has been refunded.
// A disputed, failed, open or void invoice keeps its status.
func (s *Service) syncRefundedStatus(ctx context.Context, repo repository.Repository, workspaceID uuid.UUID, inv gen.FindInvoiceByExternalPaymentIDRow) error {
	switch inv.Status {
	case model.InvoicePaid, model.InvoicePartiallyRefunded, model.InvoiceRefunded:
	default:
		return nil
	}
	status, err := refundedStatus(ctx, repo, workspaceID, inv)
	if err != nil || status == inv.Status {
		return err
	}
	return setInvoiceStatus(ctx, repo, workspaceID, inv.ID, status)
}

// refundedStatus is paid, partially_refunded or refunded, by the invoice's
// pending and succeeded refunds.
func refundedStatus(ctx context.Context, repo repository.Repository, workspaceID uuid.UUID, inv gen.FindInvoiceByExternalPaymentIDRow) (string, error) {
	refunded, err := repo.SumInvoiceRefunds(ctx, gen.SumInvoiceRefundsParams{WorkspaceID: workspaceID, InvoiceID: inv.ID})
	if err != nil {
		return "", fmt.Errorf("sum invoice refunds: %w", err)
	}
	switch {
	case refunded <= 0:
		return model.InvoicePaid, nil
	case refunded >= inv.TotalMinor:
		return model.InvoiceRefunded, nil
	default:
		return model.InvoicePartiallyRefunded, nil
	}
}

func setInvoiceStatus(ctx context.Context, repo repository.Repository, workspaceID, invoiceID uuid.UUID, status string) error {
	if _, err := repo.SetInvoiceStatus(ctx, gen.SetInvoiceStatusParams{
		Status: status, WorkspaceID: workspaceID, InvoiceID: invoiceID,
	}); err != nil {
		return fmt.Errorf("set invoice status: %w", err)
	}
	return nil
}

// onDisputeOpened marks the disputed payment's invoice disputed.
func (s *Service) onDisputeOpened(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, ev payment.Event) error {
	if ev.Payment == nil || ev.Payment.ExternalID == "" {
		s.log.WarnContext(ctx, "billing: dispute event without a payment", "event", ev)
		return nil
	}
	repo := repository.NewRepository(tx)
	inv, ok, err := lockedInvoice(ctx, repo, ev.Provider, workspaceID, ev.Payment.ExternalID)
	if err != nil || !ok {
		if !ok && err == nil {
			s.log.WarnContext(ctx, "billing: dispute names no invoice of the workspace", "event", ev, "workspace_id", workspaceID)
		}
		return err
	}
	if inv.Status == model.InvoiceDisputed {
		return nil
	}
	s.log.WarnContext(ctx, "billing: payment disputed", "event", ev, "workspace_id", workspaceID, "invoice_id", inv.ID)
	return setInvoiceStatus(ctx, repo, workspaceID, inv.ID, model.InvoiceDisputed)
}

// onDisputeClosed ends a dispute. Won: the invoice goes back to paid (or
// partially_refunded / refunded, by its refunds). Lost: the money went back
// to the customer, so the invoice stays disputed for support to follow up.
func (s *Service) onDisputeClosed(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, ev payment.Event) error {
	if ev.Payment == nil || ev.Payment.ExternalID == "" {
		s.log.WarnContext(ctx, "billing: dispute event without a payment", "event", ev)
		return nil
	}
	repo := repository.NewRepository(tx)
	inv, ok, err := lockedInvoice(ctx, repo, ev.Provider, workspaceID, ev.Payment.ExternalID)
	if err != nil || !ok {
		if !ok && err == nil {
			s.log.WarnContext(ctx, "billing: dispute names no invoice of the workspace", "event", ev, "workspace_id", workspaceID)
		}
		return err
	}
	if inv.Status != model.InvoiceDisputed {
		return nil
	}
	if ev.Payment.Status != "won" {
		// The money went back to the customer: suspend the plan it paid for.
		// Support can restore the grant once it is settled.
		s.log.WarnContext(ctx, "billing: dispute lost", "event", ev, "workspace_id", workspaceID, "invoice_id", inv.ID,
			"outcome", ev.Payment.Status)
		g, err := currentGrant(ctx, repo, workspaceID)
		if err != nil || g == nil || g.Status != model.GrantActive {
			return err
		}
		if (inv.SubscriptionID != nil && sameID(g.SubscriptionID, *inv.SubscriptionID)) ||
			(inv.OrderID != nil && sameID(g.OrderID, *inv.OrderID)) {
			return s.SuspendGrant(ctx, tx, workspaceID, g.ID)
		}
		return nil
	}
	status, err := refundedStatus(ctx, repo, workspaceID, inv)
	if err != nil {
		return err
	}
	return setInvoiceStatus(ctx, repo, workspaceID, inv.ID, status)
}

// ---------- Periodic jobs ----------

// PurgeEventPayloads nulls payment_events.payload older than cfg.EventPayloadTTL
// and returns how many it cleared. Run by the PurgeEventPayloads worker.
func (s *Service) PurgeEventPayloads(ctx context.Context) (int64, error) {
	repo := repository.NewRepository(s.pool)
	before := s.now().Add(-s.cfg.EventPayloadTTL)
	var total int64
	for {
		n, err := repo.PurgeEventPayloads(ctx, gen.PurgeEventPayloadsParams{Before: before, MaxRows: purgeBatch})
		if err != nil {
			return total, fmt.Errorf("purge event payloads: %w", err)
		}
		total += n
		if n < purgeBatch {
			break
		}
		if err := ctx.Err(); err != nil {
			return total, fmt.Errorf("purge event payloads: %w", err)
		}
	}
	if total > 0 {
		s.log.InfoContext(ctx, "billing: purged webhook payloads", "count", total, "before", before)
	}
	return total, nil
}

// CheckPlanCacheDrift logs every workspace whose workspaces.plan disagrees with
// its current grant (Q8) and returns how many it found. Run nightly by the
// PlanCacheDrift worker.
func (s *Service) CheckPlanCacheDrift(ctx context.Context) (int, error) {
	rows, err := repository.NewRepository(s.pool).ListPlanCacheDrift(ctx, gen.ListPlanCacheDriftParams{
		FreePlan: model.FreePlan, MaxRows: driftBatch,
	})
	if err != nil {
		return 0, fmt.Errorf("list plan cache drift: %w", err)
	}
	for _, r := range rows {
		s.log.WarnContext(ctx, "billing: plan cache drift", slog.String("workspace_id", r.WorkspaceID.String()),
			slog.String("cached_plan", r.CachedPlan), slog.String("granted_plan", r.GrantedPlan))
	}
	if len(rows) == driftBatch {
		s.log.WarnContext(ctx, "billing: plan cache drift capped; more workspaces may drift", "limit", driftBatch)
	}
	return len(rows), nil
}
