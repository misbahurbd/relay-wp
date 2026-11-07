package service

import (
	"context"
	"fmt"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// PlanLimits reads a cap of the workspace's billing plan. Implemented by billing.Service.
type PlanLimits interface {
	// Limit is the plan's cap on a resource (model.PlanResourceMembers); a negative value is unlimited.
	Limit(ctx context.Context, workspaceID uuid.UUID, resource string) (int, error)
}

// SetPlanLimits makes invites and acceptances respect the plan's member cap.
// Without it (nil) only the workspace's seat limit applies.
func (s *Service) SetPlanLimits(p PlanLimits) { s.plans = p }

// memberLimit is the plan's member cap, or -1 when there is none.
func (s *Service) memberLimit(ctx context.Context, workspaceID uuid.UUID) (int, error) {
	if s.plans == nil {
		return -1, nil
	}
	limit, err := s.plans.Limit(ctx, workspaceID, model.PlanResourceMembers)
	if err != nil {
		return 0, fmt.Errorf("plan member limit: %w", err)
	}
	return limit, nil
}

// takeSeats checks, under the workspace's seat lock in tx, that n more
// seat-using invites fit the plan's member cap and the workspace's seats.
func (s *Service) takeSeats(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, n int64) error {
	planLimit, err := s.memberLimit(ctx, workspaceID)
	if err != nil {
		return err
	}
	seats, err := s.seats(ctx, tx, workspaceID)
	if err != nil {
		return err
	}
	if err := overMemberLimit(planLimit, seats.Used+seats.Reserved, n); err != nil {
		return err
	}
	if seats.Limit >= 0 && seats.Available < n {
		return model.ErrNotEnoughSeats.WithDetails(apperr.Detail{
			Field:   "emails",
			Message: fmt.Sprintf("%d seats needed, %d available", n, seats.Available),
		})
	}
	return nil
}

// overMemberLimit fails when adding more seat-using people to the taken ones
// (members plus pending invites) would pass the plan's cap.
func overMemberLimit(limit int, taken, adding int64) error {
	if limit < 0 || taken+adding <= int64(limit) {
		return nil
	}
	return model.ErrMemberLimit.WithDetails(apperr.Detail{
		Message: fmt.Sprintf("Your plan allows %d members; %d are taken or invited", limit, taken),
	})
}
