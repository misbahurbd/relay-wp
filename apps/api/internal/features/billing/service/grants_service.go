package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

// Track T3: grants, what a workspace can use. Every grant change writes
// workspaces.plan and trial_ends_at in the same transaction (Q8).

// Overview is the billing page: the current plan, the live subscription and
// the live lifetime deal with its installments.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) Overview(ctx context.Context, workspaceID uuid.UUID) (model.Overview, error) {
	return model.Overview{}, model.ErrNotImplemented
}

// CurrentPlan is the plan the workspace is on now: free with no grant, and
// free limits while the grant is suspended.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) CurrentPlan(ctx context.Context, workspaceID uuid.UUID) (model.CurrentPlan, error) {
	return model.CurrentPlan{}, model.ErrNotImplemented
}

// MemberLimit is how many seat-using members the current plan allows, or
// model.Unlimited. For features/team, through an interface it declares (T8).
//
//nolint:revive // stub: track T3 builds it
func (s *Service) MemberLimit(ctx context.Context, workspaceID uuid.UUID) (int, error) {
	return 0, model.ErrNotImplemented
}

// NumberLimit is how many WhatsApp numbers the current plan allows, or
// model.Unlimited. For features/whatsapp, through an interface it declares (T8).
//
//nolint:revive // stub: track T3 builds it
func (s *Service) NumberLimit(ctx context.Context, workspaceID uuid.UUID) (int, error) {
	return 0, model.ErrNotImplemented
}

// DeletionCheck says whether the workspace can be deleted (Q17): not while a
// subscription is live, and only with confirmation while it has a lifetime
// deal. For features/workspaces, through an interface it declares (T8).
//
//nolint:revive // stub: track T3 builds it
func (s *Service) DeletionCheck(ctx context.Context, workspaceID uuid.UUID) (model.DeletionCheck, error) {
	return model.DeletionCheck{}, model.ErrNotImplemented
}

// StartTrial gives a new workspace planCode's card-less trial in tx (Q5): a
// grant with source trial ending after plans.trial_days. Nil when the plan has
// no trial or needs a card. For workspace creation, wired in Wave 2.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) StartTrial(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, planCode string) (*model.Grant, error) {
	return nil, model.ErrNotImplemented
}

// ActivateGrant ends the current grant and starts a new one in tx. Track T4
// calls it when installment 1 is paid.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) ActivateGrant(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, in model.GrantInput) (model.Grant, error) {
	return model.Grant{}, model.ErrNotImplemented
}

// SuspendGrant suspends a grant in tx: the workspace falls back to free limits
// and keeps its data (Q2). Track T4 calls it for an overdue installment.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) SuspendGrant(ctx context.Context, tx pgx.Tx, workspaceID, grantID uuid.UUID) error {
	return model.ErrNotImplemented
}

// RestoreGrant reactivates a suspended grant in tx. Track T4 calls it when a
// late installment is paid or a defaulted order resumes.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) RestoreGrant(ctx context.Context, tx pgx.Tx, workspaceID, grantID uuid.UUID) error {
	return model.ErrNotImplemented
}

// EndGrant ends a grant in tx; the workspace is on the free plan until another starts.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) EndGrant(ctx context.Context, tx pgx.Tx, workspaceID, grantID uuid.UUID) error {
	return model.ErrNotImplemented
}

// ExpireTrials ends trial grants that reached ends_at and returns how many.
// Run by the ExpireTrials worker.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) ExpireTrials(ctx context.Context) (int, error) {
	return 0, model.ErrNotImplemented
}
