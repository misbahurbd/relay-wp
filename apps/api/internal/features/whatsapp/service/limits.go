package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// PlanLimits reads a cap of the workspace's billing plan. Implemented by billing.Service.
type PlanLimits interface {
	// Limit is the plan's cap on a resource (model.PlanResourceNumbers); a negative value is unlimited.
	Limit(ctx context.Context, workspaceID uuid.UUID, resource string) (int, error)
}

// SetPlanLimits makes Connect respect the plan's number cap. Without it (nil) there is no cap.
func (s *Service) SetPlanLimits(p PlanLimits) { s.plans = p }

// numberLimit is the plan's number cap, or -1 when there is none.
func (s *Service) numberLimit(ctx context.Context, workspaceID uuid.UUID) (int, error) {
	if s.plans == nil {
		return -1, nil
	}
	limit, err := s.plans.Limit(ctx, workspaceID, model.PlanResourceNumbers)
	if err != nil {
		return 0, fmt.Errorf("plan number limit: %w", err)
	}
	return limit, nil
}

// checkNumberLimit fails when connecting phoneNumberID would give the workspace
// more numbers than limit. Reconnecting a number the workspace already has is
// always allowed. Every stored number counts, whatever its status.
func checkNumberLimit(ctx context.Context, db database.DBTX, workspaceID uuid.UUID, phoneNumberID string, limit int) error {
	if limit < 0 {
		return nil
	}
	r := repository.NewRepository(db)
	if ws, err := r.GetNumberWorkspace(ctx, phoneNumberID); err == nil && ws == workspaceID {
		return nil
	} else if err != nil && !repository.IsNoRows(err) {
		return fmt.Errorf("look up number: %w", err)
	}
	numbers, err := r.ListWhatsAppNumbers(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("count numbers: %w", err)
	}
	return overNumberLimit(limit, len(numbers))
}

// overNumberLimit fails when one more number on top of connected would pass limit.
func overNumberLimit(limit, connected int) error {
	if limit < 0 || connected < limit {
		return nil
	}
	return model.ErrNumberLimit.WithDetails(apperr.Detail{
		Message: fmt.Sprintf("Your plan allows %d WhatsApp numbers; %d are connected", limit, connected),
	})
}
