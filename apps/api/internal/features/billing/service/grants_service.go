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
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// Track T3: grants, what a workspace can use. Every grant change writes
// workspaces.plan and trial_ends_at in the same transaction (Q8).

// freePlanName is what the billing page calls the plan with no grant.
const freePlanName = "Free"

// jobBatch is how many rows one pass of a cleanup job takes at a time.
const jobBatch = 200

// Overview is the billing page: the current plan, the live subscription and
// the live lifetime deal with its installments.
func (s *Service) Overview(ctx context.Context, workspaceID uuid.UUID) (model.Overview, error) {
	plan, err := s.CurrentPlan(ctx, workspaceID)
	if err != nil {
		return model.Overview{}, err
	}
	out := model.Overview{Plan: plan}
	sub, err := s.liveSubscription(ctx, repository.NewRepository(s.pool), workspaceID)
	switch {
	case err == nil:
		out.Subscription = &sub
	case !errors.Is(err, model.ErrNoSubscription):
		return model.Overview{}, err
	}
	order, err := s.CurrentOrder(ctx, workspaceID)
	switch {
	case err == nil:
		out.Order = &order
	case !errors.Is(err, model.ErrNoOrder):
		return model.Overview{}, err
	}
	return out, nil
}

// CurrentPlan is the plan the workspace is on now: free with no grant, and
// free limits while the grant is suspended.
func (s *Service) CurrentPlan(ctx context.Context, workspaceID uuid.UUID) (model.CurrentPlan, error) {
	r := repository.NewRepository(s.pool)
	g, err := currentGrant(ctx, r, workspaceID)
	if err != nil {
		return model.CurrentPlan{}, err
	}
	if planCodeOf(g) != model.FreePlan {
		return model.CurrentPlan{PlanCode: g.PlanCode, PlanName: g.PlanName, Grant: g, Limits: g.PlanLimits.With(g.Overrides).Complete()}, nil
	}
	free, err := freeLimits(ctx, r)
	if err != nil {
		return model.CurrentPlan{}, err
	}
	return model.CurrentPlan{PlanCode: model.FreePlan, PlanName: freePlanName, Grant: g, Limits: free.Complete()}, nil
}

// planCodeOf is the plan a workspace with current grant g (nil: none) is on.
func planCodeOf(g *model.Grant) string {
	if g == nil || g.Status == model.GrantSuspended {
		return model.FreePlan
	}
	return g.PlanCode
}

// freeLimits are the free plan row's limits (ADR 0008).
func freeLimits(ctx context.Context, r repository.Repository) (model.Limits, error) {
	raw, err := r.GetFreePlanLimits(ctx)
	if err != nil {
		return nil, fmt.Errorf("get free plan limits: %w", err)
	}
	return model.ParseLimits(raw)
}

// Limit is the current plan's cap on resource key, or model.Unlimited. Team
// and WhatsApp call it through interfaces they declare (ADR 0008).
func (s *Service) Limit(ctx context.Context, workspaceID uuid.UUID, key string) (int, error) {
	if _, ok := model.ResourceByKey(key); !ok {
		return 0, fmt.Errorf("plan limit: %q is not in the resource catalog", key)
	}
	p, err := s.CurrentPlan(ctx, workspaceID)
	if err != nil {
		return 0, err
	}
	return p.Limits.Of(key), nil
}

// DeletionCheck says whether the workspace can be deleted (Q17): not while a
// subscription is live, and only with confirmation while it has a lifetime
// deal. For features/workspaces, through an interface it declares (T8).
func (s *Service) DeletionCheck(ctx context.Context, workspaceID uuid.UUID) (model.DeletionCheck, error) {
	r := repository.NewRepository(s.pool)
	_, err := r.GetLiveSubscription(ctx, workspaceID)
	if err == nil {
		return model.DeletionCheck{Reason: model.DeleteBlockedSubscription}, nil
	}
	if !repository.IsNoRows(err) {
		return model.DeletionCheck{}, fmt.Errorf("get live subscription: %w", err)
	}
	_, err = r.GetLiveOrder(ctx, workspaceID)
	if err == nil {
		return model.DeletionCheck{Reason: model.DeleteNeedsLTDConfirm}, nil
	}
	if !repository.IsNoRows(err) {
		return model.DeletionCheck{}, fmt.Errorf("get live order: %w", err)
	}
	return model.DeletionCheck{Allowed: true}, nil
}

// StartWorkspaceTrial starts a new workspace's trial of cfg.TrialPlan in tx and
// returns when it ends. Nil when that plan has no card-less trial or isn't in
// the catalog. For features/workspaces, through an interface it declares.
func (s *Service) StartWorkspaceTrial(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID) (*time.Time, error) {
	if s.cfg.TrialPlan == "" {
		return nil, nil
	}
	g, err := s.StartTrial(ctx, tx, workspaceID, s.cfg.TrialPlan)
	if errors.Is(err, model.ErrPlanNotFound) {
		s.logger().WarnContext(ctx, "billing: TRIAL_PLAN is not an active plan; no trial", "plan", s.cfg.TrialPlan)
		return nil, nil
	}
	if err != nil || g == nil {
		return nil, err
	}
	return g.EndsAt, nil
}

// StartTrial gives a new workspace planCode's card-less trial in tx (Q5): a
// grant with source trial ending after plans.trial_days. Nil when the plan has
// no trial or needs a card.
func (s *Service) StartTrial(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, planCode string) (*model.Grant, error) {
	plans, err := repository.NewRepository(tx).ListActivePlans(ctx)
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}
	// The newest active version of the plan.
	idx := -1
	for i, p := range plans {
		if p.Code == planCode && (idx < 0 || p.Version > plans[idx].Version) {
			idx = i
		}
	}
	if idx < 0 {
		return nil, model.ErrPlanNotFound
	}
	plan := plans[idx]
	if plan.TrialDays == 0 || plan.TrialRequiresCard {
		return nil, nil //nolint:nilnil // no trial is not an error
	}
	now := s.now()
	ends := now.AddDate(0, 0, int(plan.TrialDays))
	g, err := s.ActivateGrant(ctx, tx, workspaceID, model.GrantInput{
		PlanID: plan.ID, Source: model.GrantFromTrial, StartsAt: now, EndsAt: &ends,
	})
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// ActivateGrant ends the current grant and starts a new one in tx. Track T4
// calls it when installment 1 is paid.
func (s *Service) ActivateGrant(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, in model.GrantInput) (model.Grant, error) {
	if err := checkGrantInput(in); err != nil {
		return model.Grant{}, err
	}
	r := repository.NewRepository(tx)
	plan, err := r.GetPlan(ctx, in.PlanID)
	if repository.IsNoRows(err) {
		return model.Grant{}, model.ErrPlanNotFound
	}
	if err != nil {
		return model.Grant{}, fmt.Errorf("get plan: %w", err)
	}
	if plan.Code == model.FreePlan {
		return model.Grant{}, model.ErrInvalidInput.WithDetails(apperr.Detail{
			Field: "plan_id", Message: "The free plan is what a workspace has with no grant; end the grant instead",
		})
	}
	now := s.now()
	cur, err := r.LockCurrentGrant(ctx, workspaceID)
	switch {
	case err == nil:
		if _, err := r.EndGrant(ctx, gen.EndGrantParams{Now: &now, WorkspaceID: workspaceID, GrantID: cur.ID}); err != nil {
			return model.Grant{}, fmt.Errorf("end grant: %w", err)
		}
	case !repository.IsNoRows(err):
		return model.Grant{}, fmt.Errorf("lock current grant: %w", err)
	}
	startsAt := in.StartsAt
	if startsAt.IsZero() {
		startsAt = now
	}
	if _, err := r.CreateGrant(ctx, gen.CreateGrantParams{
		WorkspaceID: workspaceID, PlanID: in.PlanID, Source: in.Source, SubscriptionID: in.SubscriptionID,
		OrderID: in.OrderID, StartsAt: startsAt, EndsAt: in.EndsAt, CreatedBy: in.CreatedBy,
		Reason: in.Reason,
	}); err != nil {
		if repository.IsForeignKey(err) {
			return model.Grant{}, model.ErrInvalidInput.Wrap(err)
		}
		return model.Grant{}, fmt.Errorf("create grant: %w", err)
	}
	g, err := s.syncPlanCache(ctx, r, workspaceID)
	if err != nil {
		return model.Grant{}, err
	}
	return *g, nil
}

// checkGrantInput mirrors the plan_grants CHECKs: each source names what it comes from.
func checkGrantInput(in model.GrantInput) error {
	var details []apperr.Detail
	bad := func(field, msg string) { details = append(details, apperr.Detail{Field: field, Message: msg}) }
	switch in.Source {
	case model.GrantFromSubscription, model.GrantFromOrder, model.GrantFromTrial, model.GrantManual:
	default:
		bad("source", "Unknown grant source")
	}
	if (in.Source == model.GrantFromSubscription) != (in.SubscriptionID != nil) {
		bad("subscription_id", "Only a subscription grant names a subscription, and it must")
	}
	if (in.Source == model.GrantFromOrder) != (in.OrderID != nil) {
		bad("order_id", "Only an order grant names an order, and it must")
	}
	if in.Source == model.GrantFromTrial && in.EndsAt == nil {
		bad("ends_at", "A trial needs an end")
	}
	if in.Source == model.GrantManual && (in.Reason == nil || *in.Reason == "") {
		bad("reason", "A manual grant needs a reason")
	}
	if len(details) > 0 {
		return model.ErrInvalidInput.WithDetails(details...)
	}
	return nil
}

// SuspendGrant suspends a grant in tx: the workspace falls back to free limits
// and keeps its data (Q2). Track T4 calls it for an overdue installment.
func (s *Service) SuspendGrant(ctx context.Context, tx pgx.Tx, workspaceID, grantID uuid.UUID, reason string) error {
	r := repository.NewRepository(tx)
	now := s.now()
	if _, err := r.SuspendGrant(ctx, gen.SuspendGrantParams{
		Now: &now, Reason: &reason, WorkspaceID: workspaceID, GrantID: grantID,
	}); err != nil {
		return fmt.Errorf("suspend grant: %w", err)
	}
	_, err := s.syncPlanCache(ctx, r, workspaceID)
	return err
}

// RestoreGrant reactivates a suspended grant in tx. Track T4 calls it when a
// late installment is paid or a defaulted order resumes.
func (s *Service) RestoreGrant(ctx context.Context, tx pgx.Tx, workspaceID, grantID uuid.UUID) error {
	r := repository.NewRepository(tx)
	if _, err := r.RestoreGrant(ctx, gen.RestoreGrantParams{WorkspaceID: workspaceID, GrantID: grantID}); err != nil {
		return fmt.Errorf("restore grant: %w", err)
	}
	_, err := s.syncPlanCache(ctx, r, workspaceID)
	return err
}

// EndGrant ends a grant in tx; the workspace is on the free plan until another starts.
func (s *Service) EndGrant(ctx context.Context, tx pgx.Tx, workspaceID, grantID uuid.UUID) error {
	r := repository.NewRepository(tx)
	now := s.now()
	if _, err := r.EndGrant(ctx, gen.EndGrantParams{Now: &now, WorkspaceID: workspaceID, GrantID: grantID}); err != nil {
		return fmt.Errorf("end grant: %w", err)
	}
	_, err := s.syncPlanCache(ctx, r, workspaceID)
	return err
}

// ExpireGrants ends card-less trials and staff grants that reached ends_at, and
// returns how many. Run by the ExpireTrials worker.
func (s *Service) ExpireGrants(ctx context.Context) (int, error) {
	ended := 0
	for {
		now := s.now()
		rows, err := repository.NewRepository(s.pool).ListExpiredGrants(ctx, gen.ListExpiredGrantsParams{
			Now: &now, MaxRows: jobBatch,
		})
		if err != nil {
			return ended, fmt.Errorf("list expired grants: %w", err)
		}
		for _, g := range rows {
			if err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
				return s.EndGrant(ctx, tx, g.WorkspaceID, g.ID)
			}); err != nil {
				return ended, err
			}
			ended++
		}
		if len(rows) < jobBatch {
			return ended, nil
		}
	}
}

// syncPlanCache writes workspaces.plan and trial_ends_at from the current
// grant, in the caller's transaction (Q8), and returns that grant (nil: free).
func (s *Service) syncPlanCache(ctx context.Context, r repository.Repository, workspaceID uuid.UUID) (*model.Grant, error) {
	g, err := currentGrant(ctx, r, workspaceID)
	if err != nil {
		return nil, err
	}
	params := gen.SetWorkspacePlanCacheParams{PlanCode: planCodeOf(g), WorkspaceID: workspaceID}
	if g != nil && g.Status == model.GrantActive && g.Source == model.GrantFromTrial {
		params.TrialEndsAt = g.EndsAt
	}
	n, err := r.SetWorkspacePlanCache(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("set workspace plan: %w", err)
	}
	if n == 0 {
		return nil, fmt.Errorf("set workspace plan: workspace %s not found", workspaceID)
	}
	return g, nil
}

// currentGrant is the workspace's active or suspended grant, or nil.
func currentGrant(ctx context.Context, r repository.Repository, workspaceID uuid.UUID) (*model.Grant, error) {
	row, err := r.GetCurrentGrant(ctx, workspaceID)
	if repository.IsNoRows(err) {
		return nil, nil //nolint:nilnil // no grant means the free plan
	}
	if err != nil {
		return nil, fmt.Errorf("get current grant: %w", err)
	}
	planLimits, err := model.ParseLimits(row.PlanLimits)
	if err != nil {
		return nil, fmt.Errorf("grant %s plan: %w", row.ID, err)
	}
	overrides, err := model.ParseLimits(row.Overrides)
	if err != nil {
		return nil, fmt.Errorf("grant %s overrides: %w", row.ID, err)
	}
	return &model.Grant{
		ID: row.ID, PlanID: row.PlanID, PlanCode: row.PlanCode, PlanName: row.PlanName, Source: row.Source,
		SubscriptionID: row.SubscriptionID, OrderID: row.OrderID, Status: row.Status, PlanLimits: planLimits, Overrides: overrides,
		StartsAt: row.StartsAt, EndsAt: row.EndsAt, SuspendedAt: row.SuspendedAt, SuspendReason: deref(row.SuspendReason),
	}, nil
}
