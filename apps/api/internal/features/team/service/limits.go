package service

import (
	"context"
	"fmt"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/model"

	"github.com/google/uuid"

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
