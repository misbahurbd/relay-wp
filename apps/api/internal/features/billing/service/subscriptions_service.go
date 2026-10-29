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

// Track T3: subscriptions.

// pastDueGrace is how long a past_due subscription keeps its grant before it is suspended.
const pastDueGrace = 7 * 24 * time.Hour

// Subscription returns the workspace's live subscription, or its latest one if
// none is live. model.ErrNoSubscription if it never had one.
func (s *Service) Subscription(ctx context.Context, workspaceID uuid.UUID) (model.Subscription, error) {
	r := repository.NewRepository(s.pool)
	row, err := r.GetLatestSubscription(ctx, workspaceID)
	if repository.IsNoRows(err) {
		return model.Subscription{}, model.ErrNoSubscription
	}
	if err != nil {
		return model.Subscription{}, fmt.Errorf("get latest subscription: %w", err)
	}
	return s.withPrice(ctx, r, toSubscription(gen.GetSubscriptionRow(row)))
}

// liveSubscription reads the workspace's live subscription with its price through r.
func (s *Service) liveSubscription(ctx context.Context, r repository.Repository, workspaceID uuid.UUID) (model.Subscription, error) {
	row, err := r.GetLiveSubscription(ctx, workspaceID)
	if repository.IsNoRows(err) {
		return model.Subscription{}, model.ErrNoSubscription
	}
	if err != nil {
		return model.Subscription{}, fmt.Errorf("get live subscription: %w", err)
	}
	return s.withPrice(ctx, r, toSubscription(gen.GetSubscriptionRow(row)))
}

// subscriptionByID reads one of the workspace's subscriptions with its price through r.
func (s *Service) subscriptionByID(ctx context.Context, r repository.Repository, workspaceID, subscriptionID uuid.UUID) (model.Subscription, error) {
	row, err := r.GetSubscription(ctx, gen.GetSubscriptionParams{WorkspaceID: workspaceID, SubscriptionID: subscriptionID})
	if repository.IsNoRows(err) {
		return model.Subscription{}, model.ErrNoSubscription
	}
	if err != nil {
		return model.Subscription{}, fmt.Errorf("get subscription: %w", err)
	}
	return s.withPrice(ctx, r, toSubscription(row))
}

func (s *Service) withPrice(ctx context.Context, r repository.Repository, sub model.Subscription) (model.Subscription, error) {
	p, err := s.loadPrice(ctx, r, sub.PlanPriceID)
	if err != nil {
		return model.Subscription{}, err
	}
	sub.Price = &p.Price
	return sub, nil
}

// PreviewChange says what moving the live subscription to planPriceID would
// do: an upgrade now with proration, or a downgrade at the period end (Q6).
func (s *Service) PreviewChange(ctx context.Context, workspaceID, planPriceID uuid.UUID) (model.ChangeQuote, error) {
	r := repository.NewRepository(s.pool)
	sub, err := s.liveSubscription(ctx, r, workspaceID)
	if err != nil {
		return model.ChangeQuote{}, err
	}
	target, err := s.loadPrice(ctx, r, planPriceID)
	if err != nil {
		return model.ChangeQuote{}, err
	}
	return quoteChange(sub, target, s.now())
}

// quoteChange works out a move from sub's price to target. Upgrades, and
// monthly to yearly, apply now; downgrades and yearly to monthly wait for the
// period end. An immediate change on the same interval charges the price
// difference for the rest of the period; one that changes the interval starts
// a new period now, less the unused time on the old price. Trials prorate nothing.
func quoteChange(sub model.Subscription, target loadedPrice, now time.Time) (model.ChangeQuote, error) {
	cur := sub.Price
	switch {
	case target.ID == cur.ID:
		return model.ChangeQuote{}, model.ErrSamePrice
	case !target.IsActive:
		return model.ChangeQuote{}, model.ErrPriceNotFound
	case target.BillingType != model.BillingRecurring, target.Currency != cur.Currency,
		sub.CurrentPeriodStart == nil, sub.CurrentPeriodEnd == nil:
		return model.ChangeQuote{}, model.ErrChangeNotSupported
	}
	q := model.ChangeQuote{FromPriceID: cur.ID, ToPriceID: target.ID, Currency: cur.Currency}
	applyNow := false
	switch {
	case target.PlanID == cur.PlanID && target.Interval != cur.Interval:
		q.Kind = model.ChangeInterval
		applyNow = target.Interval == model.IntervalYear
	case yearly(target.Price) > yearly(*cur):
		q.Kind, applyNow = model.ChangeUpgrade, true
	default:
		q.Kind = model.ChangeDowngrade
	}
	if !applyNow {
		q.EffectiveAt = *sub.CurrentPeriodEnd
		return q, nil
	}
	q.EffectiveAt = now
	if sub.Status == model.SubscriptionTrialing {
		return q, nil
	}
	start, end := *sub.CurrentPeriodStart, *sub.CurrentPeriodEnd
	if target.Interval == cur.Interval {
		q.ProrationMinor = prorate(target.AmountMinor-cur.AmountMinor, start, end, now)
	} else {
		q.ProrationMinor = target.AmountMinor - prorate(cur.AmountMinor, start, end, now)
	}
	return q, nil
}

// yearly is what a recurring price costs over a year, to compare prices on different intervals.
func yearly(p model.Price) int64 {
	if p.Interval == model.IntervalMonth {
		return p.AmountMinor * 12
	}
	return p.AmountMinor
}

// prorate is the share of amount for the part of [start, end) still to come at now.
func prorate(amount int64, start, end, now time.Time) int64 {
	total := int64(end.Sub(start) / time.Second)
	left := int64(end.Sub(now) / time.Second)
	if total <= 0 || left <= 0 {
		return 0
	}
	return amount * min(left, total) / total
}

// ChangePlan moves the live subscription to planPriceID: an upgrade applies now
// (prorated, with a new grant), a downgrade is scheduled through
// pending_plan_price_id. Both are logged in subscription_changes.
func (s *Service) ChangePlan(ctx context.Context, workspaceID, _, planPriceID uuid.UUID) (model.Subscription, error) {
	r := repository.NewRepository(s.pool)
	sub, err := s.liveSubscription(ctx, r, workspaceID)
	if err != nil {
		return model.Subscription{}, err
	}
	target, err := s.loadPrice(ctx, r, planPriceID)
	if err != nil {
		return model.Subscription{}, err
	}
	now := s.now()
	q, err := quoteChange(sub, target, now)
	if err != nil {
		return model.Subscription{}, err
	}
	immediate := q.EffectiveAt.Equal(now)
	g, extSub, err := s.providerSubscription(sub)
	if err != nil {
		return model.Subscription{}, err
	}
	pp, err := externalPriceID(ctx, r, target.ID, nil, sub.Provider)
	if err != nil || pp.ExternalPriceID == nil {
		return model.Subscription{}, model.ErrChangeNotSupported
	}
	changed, err := g.ChangeSubscription(ctx, payment.ChangeRequest{
		ExternalSubscriptionID: extSub, ExternalPriceID: *pp.ExternalPriceID, ProrateNow: immediate,
		IdempotencyKey: fmt.Sprintf("change:%s:%s:%d", sub.ID, target.ID, now.Unix()),
	})
	if err != nil {
		return model.Subscription{}, s.gatewayErr("change subscription", err)
	}
	if changed.ProrationMinor != 0 {
		q.ProrationMinor = changed.ProrationMinor
	}

	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if _, err := r.LockSubscription(ctx, gen.LockSubscriptionParams{WorkspaceID: workspaceID, SubscriptionID: sub.ID}); err != nil {
			return fmt.Errorf("lock subscription: %w", err)
		}
		if err := r.CreateSubscriptionChange(ctx, gen.CreateSubscriptionChangeParams{
			SubscriptionID: sub.ID, WorkspaceID: workspaceID, FromPriceID: q.FromPriceID, ToPriceID: q.ToPriceID,
			Kind: q.Kind, EffectiveAt: q.EffectiveAt, ProrationMinor: q.ProrationMinor,
		}); err != nil {
			return fmt.Errorf("log subscription change: %w", err)
		}
		if !immediate {
			_, err := r.SetSubscriptionPendingChange(ctx, gen.SetSubscriptionPendingChangeParams{
				PendingPlanPriceID: &target.ID, PendingChangeAt: &q.EffectiveAt, WorkspaceID: workspaceID, SubscriptionID: sub.ID,
			})
			if err != nil {
				return fmt.Errorf("schedule plan change: %w", err)
			}
			return nil
		}
		if _, err := r.SetSubscriptionPrice(ctx, gen.SetSubscriptionPriceParams{
			PlanPriceID: target.ID, WorkspaceID: workspaceID, SubscriptionID: sub.ID,
		}); err != nil {
			return fmt.Errorf("set subscription price: %w", err)
		}
		if changed.CurrentPeriodStart != nil && changed.CurrentPeriodEnd != nil {
			sub.CurrentPeriodStart, sub.CurrentPeriodEnd = changed.CurrentPeriodStart, changed.CurrentPeriodEnd
			if err := syncPeriod(ctx, r, sub); err != nil {
				return err
			}
		}
		return s.ensureSubscriptionGrant(ctx, tx, workspaceID, sub.ID, target.PlanID)
	})
	if err != nil {
		return model.Subscription{}, err
	}
	return s.subscriptionByID(ctx, r, workspaceID, sub.ID)
}

// syncPeriod writes sub's periods back, keeping everything else as it is.
func syncPeriod(ctx context.Context, r repository.Repository, sub model.Subscription) error {
	if _, err := r.SyncSubscription(ctx, gen.SyncSubscriptionParams{
		Status: sub.Status, ProviderStatus: sub.ProviderStatus, CurrentPeriodStart: sub.CurrentPeriodStart,
		CurrentPeriodEnd: sub.CurrentPeriodEnd, GraceEndsAt: sub.GraceEndsAt, CancelAtPeriodEnd: sub.CancelAtPeriodEnd,
		CanceledAt: sub.CanceledAt, WorkspaceID: sub.WorkspaceID, SubscriptionID: sub.ID,
	}); err != nil {
		return fmt.Errorf("sync subscription: %w", err)
	}
	return nil
}

// CancelPendingChange drops a scheduled downgrade. model.ErrNoPendingChange if there is none.
func (s *Service) CancelPendingChange(ctx context.Context, workspaceID, _ uuid.UUID) (model.Subscription, error) {
	r := repository.NewRepository(s.pool)
	sub, err := s.liveSubscription(ctx, r, workspaceID)
	if err != nil {
		return model.Subscription{}, err
	}
	if sub.PendingPlanPriceID == nil {
		return model.Subscription{}, model.ErrNoPendingChange
	}
	g, extSub, err := s.providerSubscription(sub)
	if err != nil {
		return model.Subscription{}, err
	}
	pp, err := externalPriceID(ctx, r, sub.PlanPriceID, nil, sub.Provider)
	if err != nil || pp.ExternalPriceID == nil {
		return model.Subscription{}, model.ErrChangeNotSupported
	}
	// The provider was told the new price for the next period; put the current one back.
	if _, err := g.ChangeSubscription(ctx, payment.ChangeRequest{
		ExternalSubscriptionID: extSub, ExternalPriceID: *pp.ExternalPriceID,
		IdempotencyKey: fmt.Sprintf("change:%s:%s:%d", sub.ID, sub.PlanPriceID, s.now().Unix()),
	}); err != nil {
		return model.Subscription{}, s.gatewayErr("undo plan change", err)
	}
	if _, err := r.SetSubscriptionPendingChange(ctx, gen.SetSubscriptionPendingChangeParams{
		WorkspaceID: workspaceID, SubscriptionID: sub.ID,
	}); err != nil {
		return model.Subscription{}, fmt.Errorf("clear plan change: %w", err)
	}
	return s.subscriptionByID(ctx, r, workspaceID, sub.ID)
}

// CancelSubscription cancels the live subscription at the period end, or now
// (its grant ends with it).
func (s *Service) CancelSubscription(ctx context.Context, workspaceID, _ uuid.UUID, atPeriodEnd bool) (model.Subscription, error) {
	r := repository.NewRepository(s.pool)
	sub, err := s.liveSubscription(ctx, r, workspaceID)
	if err != nil {
		return model.Subscription{}, err
	}
	if atPeriodEnd && sub.CancelAtPeriodEnd {
		return sub, nil
	}
	g, extSub, err := s.providerSubscription(sub)
	if err != nil {
		return model.Subscription{}, err
	}
	mode := payment.CancelNow
	if atPeriodEnd {
		mode = payment.CancelAtPeriodEnd
	}
	if _, err := g.CancelSubscription(ctx, payment.CancelRequest{ExternalSubscriptionID: extSub, Mode: mode}); err != nil {
		return model.Subscription{}, s.gatewayErr("cancel subscription", err)
	}
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if atPeriodEnd {
			_, err := repository.NewRepository(tx).SetSubscriptionCancelAtPeriodEnd(ctx, gen.SetSubscriptionCancelAtPeriodEndParams{
				CancelAtPeriodEnd: true, WorkspaceID: workspaceID, SubscriptionID: sub.ID,
			})
			if err != nil {
				return fmt.Errorf("set cancel at period end: %w", err)
			}
			return nil
		}
		return s.cancelNow(ctx, tx, workspaceID, sub.ID)
	})
	if err != nil {
		return model.Subscription{}, err
	}
	return s.subscriptionByID(ctx, r, workspaceID, sub.ID)
}

// cancelNow marks a subscription canceled in tx and ends its grant.
func (s *Service) cancelNow(ctx context.Context, tx pgx.Tx, workspaceID, subscriptionID uuid.UUID) error {
	now := s.now()
	if _, err := repository.NewRepository(tx).CancelSubscriptionNow(ctx, gen.CancelSubscriptionNowParams{
		CanceledAt: &now, WorkspaceID: workspaceID, SubscriptionID: subscriptionID,
	}); err != nil {
		return fmt.Errorf("cancel subscription: %w", err)
	}
	return s.endSubscriptionGrant(ctx, tx, workspaceID, subscriptionID)
}

// ResumeSubscription undoes a cancel at the period end. model.ErrNotCanceling if none is set.
func (s *Service) ResumeSubscription(ctx context.Context, workspaceID, _ uuid.UUID) (model.Subscription, error) {
	r := repository.NewRepository(s.pool)
	sub, err := s.liveSubscription(ctx, r, workspaceID)
	if err != nil {
		return model.Subscription{}, err
	}
	if !sub.CancelAtPeriodEnd {
		return model.Subscription{}, model.ErrNotCanceling
	}
	g, extSub, err := s.providerSubscription(sub)
	if err != nil {
		return model.Subscription{}, err
	}
	if _, err := g.CancelSubscription(ctx, payment.CancelRequest{ExternalSubscriptionID: extSub, Mode: payment.CancelUndo}); err != nil {
		return model.Subscription{}, s.gatewayErr("resume subscription", err)
	}
	if _, err := r.SetSubscriptionCancelAtPeriodEnd(ctx, gen.SetSubscriptionCancelAtPeriodEndParams{
		WorkspaceID: workspaceID, SubscriptionID: sub.ID,
	}); err != nil {
		return model.Subscription{}, fmt.Errorf("clear cancel at period end: %w", err)
	}
	return s.subscriptionByID(ctx, r, workspaceID, sub.ID)
}

// SubscriptionCredit is what CancelForLTD would credit right now: the unused
// time on the live subscription. Zero with no live subscription. Track T4's
// QuoteOrder uses it.
func (s *Service) SubscriptionCredit(ctx context.Context, workspaceID uuid.UUID) (model.SubscriptionCredit, error) {
	r := repository.NewRepository(s.pool)
	sub, err := s.liveSubscription(ctx, r, workspaceID)
	if errors.Is(err, model.ErrNoSubscription) {
		return model.SubscriptionCredit{}, nil
	}
	if err != nil {
		return model.SubscriptionCredit{}, err
	}
	return s.credit(ctx, r, sub)
}

// credit is the unused time on sub at its discounted price, if a discount still applies.
// Trials and subscriptions without a period have none.
func (s *Service) credit(ctx context.Context, r repository.Repository, sub model.Subscription) (model.SubscriptionCredit, error) {
	out := model.SubscriptionCredit{SubscriptionID: &sub.ID, Currency: sub.Price.Currency}
	if sub.Status == model.SubscriptionTrialing || sub.CurrentPeriodStart == nil || sub.CurrentPeriodEnd == nil {
		return out, nil
	}
	now := s.now()
	amount := sub.Price.AmountMinor
	if sub.DiscountID != nil && (sub.DiscountEndsAt == nil || sub.DiscountEndsAt.After(now)) {
		d, err := r.GetDiscount(ctx, *sub.DiscountID)
		if err != nil {
			return model.SubscriptionCredit{}, fmt.Errorf("get discount: %w", err)
		}
		amount -= amountOff(toDiscount(d), amount)
	}
	out.CreditMinor = prorate(amount, *sub.CurrentPeriodStart, *sub.CurrentPeriodEnd, now)
	return out, nil
}

// CancelForLTD cancels the live subscription now, in tx, because the workspace
// is buying a lifetime deal (Q13), and returns the unused time as credit for
// orders.credit_minor. Zero with no live subscription. Track T4 calls it.
func (s *Service) CancelForLTD(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID) (model.SubscriptionCredit, error) {
	r := repository.NewRepository(tx)
	live, err := r.GetLiveSubscription(ctx, workspaceID)
	if repository.IsNoRows(err) {
		return model.SubscriptionCredit{}, nil
	}
	if err != nil {
		return model.SubscriptionCredit{}, fmt.Errorf("get live subscription: %w", err)
	}
	locked, err := r.LockSubscription(ctx, gen.LockSubscriptionParams{WorkspaceID: workspaceID, SubscriptionID: live.ID})
	if err != nil {
		return model.SubscriptionCredit{}, fmt.Errorf("lock subscription: %w", err)
	}
	sub, err := s.withPrice(ctx, r, toSubscription(gen.GetSubscriptionRow(locked)))
	if err != nil {
		return model.SubscriptionCredit{}, err
	}
	credit, err := s.credit(ctx, r, sub)
	if err != nil {
		return model.SubscriptionCredit{}, err
	}
	if g, extSub, err := s.providerSubscription(sub); err == nil {
		// Already gone at the provider is fine: we are canceling it anyway.
		if _, err := g.CancelSubscription(ctx, payment.CancelRequest{ExternalSubscriptionID: extSub, Mode: payment.CancelNow}); err != nil &&
			!errors.Is(err, payment.ErrNotFound) {
			return model.SubscriptionCredit{}, s.gatewayErr("cancel subscription for lifetime deal", err)
		}
	}
	if err := s.cancelNow(ctx, tx, workspaceID, sub.ID); err != nil {
		return model.SubscriptionCredit{}, err
	}
	return credit, nil
}

// SyncSubscription copies the provider's view of a subscription in tx
// (subscription.updated and subscription.canceled events) and changes the
// grant to match: a new period keeps it, past_due keeps it until
// grace_ends_at, canceled ends it.
func (s *Service) SyncSubscription(ctx context.Context, tx pgx.Tx, workspaceID, subscriptionID uuid.UUID, ps payment.Subscription) error {
	r := repository.NewRepository(tx)
	locked, err := r.LockSubscription(ctx, gen.LockSubscriptionParams{WorkspaceID: workspaceID, SubscriptionID: subscriptionID})
	if repository.IsNoRows(err) {
		return model.ErrNoSubscription
	}
	if err != nil {
		return fmt.Errorf("lock subscription: %w", err)
	}
	sub := toSubscription(gen.GetSubscriptionRow(locked))
	if sub.Status == model.SubscriptionCanceled {
		return nil // canceled is final; a late event changes nothing
	}
	switch ps.Status {
	case payment.SubscriptionIncomplete, payment.SubscriptionTrialing, payment.SubscriptionActive,
		payment.SubscriptionPastDue, payment.SubscriptionCanceled:
	default:
		return model.ErrInvalidInput
	}
	now := s.now()
	next := sub
	next.Status = string(ps.Status)
	next.ProviderStatus = nilIfEmpty(ps.ProviderStatus)
	next.CancelAtPeriodEnd = ps.CancelAtPeriodEnd
	if ps.CurrentPeriodStart != nil && ps.CurrentPeriodEnd != nil {
		next.CurrentPeriodStart, next.CurrentPeriodEnd = ps.CurrentPeriodStart, ps.CurrentPeriodEnd
	}
	next.GraceEndsAt = nil
	if next.Status == model.SubscriptionPastDue {
		next.GraceEndsAt = sub.GraceEndsAt
		if next.GraceEndsAt == nil {
			next.GraceEndsAt = ptrTo(now.Add(pastDueGrace))
		}
	}
	if next.Status == model.SubscriptionCanceled {
		next.CanceledAt = ps.CanceledAt
		if next.CanceledAt == nil {
			next.CanceledAt = &now
		}
		if err := syncPeriod(ctx, r, next); err != nil {
			return err
		}
		return s.cancelNow(ctx, tx, workspaceID, sub.ID)
	}
	if err := syncPeriod(ctx, r, next); err != nil {
		return err
	}

	switch next.Status {
	case model.SubscriptionTrialing, model.SubscriptionActive:
		at := now
		if next.CurrentPeriodStart != nil {
			at = *next.CurrentPeriodStart
		}
		planPriceID, err := applyPendingChange(ctx, r, next, at)
		if err != nil {
			return err
		}
		return s.ensurePriceGrant(ctx, tx, workspaceID, sub.ID, planPriceID)
	case model.SubscriptionPastDue:
		if now.Before(*next.GraceEndsAt) {
			return s.ensurePriceGrant(ctx, tx, workspaceID, sub.ID, sub.PlanPriceID)
		}
		g, err := currentGrant(ctx, r, workspaceID)
		if err != nil || g == nil || !sameID(g.SubscriptionID, sub.ID) {
			return err
		}
		return s.SuspendGrant(ctx, tx, workspaceID, g.ID)
	default:
		return nil
	}
}

// ApplySubscriptionPayment records a renewal payment in tx (payment.succeeded
// or payment.failed for a subscription): an invoice with bill_to, and a
// pending downgrade applied when its period starts.
func (s *Service) ApplySubscriptionPayment(ctx context.Context, tx pgx.Tx, workspaceID, subscriptionID uuid.UUID, p payment.Payment, paid bool) error {
	r := repository.NewRepository(tx)
	locked, err := r.LockSubscription(ctx, gen.LockSubscriptionParams{WorkspaceID: workspaceID, SubscriptionID: subscriptionID})
	if repository.IsNoRows(err) {
		return model.ErrNoSubscription
	}
	if err != nil {
		return fmt.Errorf("lock subscription: %w", err)
	}
	sub := toSubscription(gen.GetSubscriptionRow(locked))
	billTo, err := s.BillTo(ctx, tx, workspaceID)
	if err != nil {
		return err
	}
	inv := invoiceInput{
		Provider: sub.Provider, SubscriptionID: &sub.ID, Payment: p, Paid: paid, BillTo: billTo, DiscountID: sub.DiscountID,
		Now: s.now(),
	}
	if created, err := createInvoice(ctx, r, workspaceID, inv); err != nil || !created {
		return err // a replayed payment adds nothing
	}
	if !paid || sub.Status == model.SubscriptionCanceled {
		return nil
	}

	now := s.now()
	at := now
	if p.PeriodStart != nil {
		at = *p.PeriodStart
	}
	planPriceID, err := applyPendingChange(ctx, r, sub, at)
	if err != nil {
		return err
	}
	if sub.DiscountEndsAt != nil && !sub.DiscountEndsAt.After(at) {
		if _, err := r.SetSubscriptionDiscount(ctx, gen.SetSubscriptionDiscountParams{
			WorkspaceID: workspaceID, SubscriptionID: sub.ID,
		}); err != nil {
			return fmt.Errorf("end subscription discount: %w", err)
		}
	}
	if sub.Status == model.SubscriptionIncomplete {
		return nil
	}
	return s.ensurePriceGrant(ctx, tx, workspaceID, sub.ID, planPriceID)
}

// applyPendingChange moves sub to its scheduled price once at reaches the
// change date, and returns the price sub is on afterwards.
func applyPendingChange(ctx context.Context, r repository.Repository, sub model.Subscription, at time.Time) (uuid.UUID, error) {
	if sub.PendingPlanPriceID == nil || sub.PendingChangeAt == nil || at.Before(*sub.PendingChangeAt) {
		return sub.PlanPriceID, nil
	}
	if _, err := r.SetSubscriptionPrice(ctx, gen.SetSubscriptionPriceParams{
		PlanPriceID: *sub.PendingPlanPriceID, WorkspaceID: sub.WorkspaceID, SubscriptionID: sub.ID,
	}); err != nil {
		return uuid.Nil, fmt.Errorf("apply plan change: %w", err)
	}
	return *sub.PendingPlanPriceID, nil
}

// ensurePriceGrant makes sure the workspace's grant is the plan of planPriceID, from this subscription.
func (s *Service) ensurePriceGrant(ctx context.Context, tx pgx.Tx, workspaceID, subscriptionID, planPriceID uuid.UUID) error {
	price, err := s.loadPrice(ctx, repository.NewRepository(tx), planPriceID)
	if err != nil {
		return err
	}
	return s.ensureSubscriptionGrant(ctx, tx, workspaceID, subscriptionID, price.PlanID)
}

// ensureSubscriptionGrant makes the subscription's grant for planID current:
// a no-op if it already is, restored if it was suspended, a new grant otherwise.
func (s *Service) ensureSubscriptionGrant(ctx context.Context, tx pgx.Tx, workspaceID, subscriptionID, planID uuid.UUID) error {
	g, err := currentGrant(ctx, repository.NewRepository(tx), workspaceID)
	if err != nil {
		return err
	}
	if g != nil && sameID(g.SubscriptionID, subscriptionID) && g.PlanID == planID {
		if g.Status == model.GrantSuspended {
			return s.RestoreGrant(ctx, tx, workspaceID, g.ID)
		}
		return nil
	}
	_, err = s.ActivateGrant(ctx, tx, workspaceID, model.GrantInput{
		PlanID: planID, Source: model.GrantFromSubscription, SubscriptionID: &subscriptionID, StartsAt: s.now(),
	})
	return err
}

// endSubscriptionGrant ends the current grant if it comes from this subscription.
func (s *Service) endSubscriptionGrant(ctx context.Context, tx pgx.Tx, workspaceID, subscriptionID uuid.UUID) error {
	g, err := currentGrant(ctx, repository.NewRepository(tx), workspaceID)
	if err != nil || g == nil || !sameID(g.SubscriptionID, subscriptionID) {
		return err
	}
	return s.EndGrant(ctx, tx, workspaceID, g.ID)
}

// providerSubscription is the gateway and provider ID for a subscription the provider renews.
func (s *Service) providerSubscription(sub model.Subscription) (payment.Gateway, string, error) {
	if sub.RenewalMode != model.RenewalProvider || sub.ExternalSubscriptionID == nil {
		return nil, "", model.ErrChangeNotSupported
	}
	g, err := s.gateway(sub.Provider)
	if err != nil {
		return nil, "", err
	}
	return g, *sub.ExternalSubscriptionID, nil
}

func sameID(p *uuid.UUID, id uuid.UUID) bool { return p != nil && *p == id }

func toSubscription(r gen.GetSubscriptionRow) model.Subscription {
	return model.Subscription{
		ID: r.ID, WorkspaceID: r.WorkspaceID, PlanPriceID: r.PlanPriceID, BillingCustomerID: r.BillingCustomerID,
		Provider: payment.Provider(r.Provider), ExternalSubscriptionID: r.ExternalSubscriptionID, RenewalMode: r.RenewalMode,
		Status: r.Status, ProviderStatus: r.ProviderStatus, CurrentPeriodStart: r.CurrentPeriodStart,
		CurrentPeriodEnd: r.CurrentPeriodEnd, GraceEndsAt: r.GraceEndsAt, CancelAtPeriodEnd: r.CancelAtPeriodEnd,
		CanceledAt: r.CanceledAt, PendingPlanPriceID: r.PendingPlanPriceID, PendingChangeAt: r.PendingChangeAt,
		DiscountID: r.DiscountID, DiscountEndsAt: r.DiscountEndsAt, CreatedAt: r.CreatedAt,
	}
}
