package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// PlanLimits reads a cap of the workspace's billing plan. Implemented by billing.Service.
type PlanLimits interface {
	// Limit is the plan's cap on a resource (model.PlanResourceContacts); a negative value is unlimited.
	Limit(ctx context.Context, workspaceID uuid.UUID, resource string) (int, error)
}

// SetPlanLimits makes adding contacts respect the plan's cap. Without it (nil) there is no cap.
func (s *Service) SetPlanLimits(p PlanLimits) { s.plans = p }

// contactLimit is the plan's contact cap, or -1 when there is none.
func (s *Service) contactLimit(ctx context.Context, workspaceID uuid.UUID) (int, error) {
	if s.plans == nil {
		return -1, nil
	}
	limit, err := s.plans.Limit(ctx, workspaceID, model.PlanResourceContacts)
	if err != nil {
		return 0, fmt.Errorf("plan contact limit: %w", err)
	}
	return limit, nil
}

// contactRoom is how many more contacts fit under limit, holding the
// workspace's contact lock until the transaction ends. -1 is no cap.
func contactRoom(ctx context.Context, r repository.Repository, workspaceID uuid.UUID, limit int) (int64, error) {
	if limit < 0 {
		return -1, nil
	}
	if err := r.LockContactCap(ctx, workspaceID); err != nil {
		return 0, fmt.Errorf("lock contact cap: %w", err)
	}
	have, err := r.CountWorkspaceContacts(ctx, workspaceID)
	if err != nil {
		return 0, fmt.Errorf("count contacts: %w", err)
	}
	return max(int64(limit)-have, 0), nil
}

// overContactLimit fails when room (from contactRoom) can't take one more contact.
func overContactLimit(limit int, room int64) error {
	if room != 0 {
		return nil
	}
	return model.ErrContactLimit.WithDetails(apperr.Detail{
		Message: fmt.Sprintf("Your plan allows %d contacts", limit),
	})
}
