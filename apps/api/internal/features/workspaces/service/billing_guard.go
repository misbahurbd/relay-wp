package service

import (
	"context"
	"fmt"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/workspaces/model"

	"github.com/google/uuid"
)

// Why billing blocks deleting a workspace (Q17).
const (
	deleteBlockedSubscription = "subscription" // cancel the subscription first
	deleteNeedsLTDConfirm     = "lifetime"     // the owner must confirm giving up the deal
)

// DeletionCheck is billing's answer on deleting a workspace.
type DeletionCheck struct {
	Allowed bool
	// Reason is "subscription" or "lifetime" when not allowed.
	Reason string
}

// Billing says whether a paid workspace can be deleted. Implemented by billing.Service, adapted by internal/app.
type Billing interface {
	DeletionCheck(ctx context.Context, workspaceID uuid.UUID) (DeletionCheck, error)
}

// SetBilling makes CheckDeletion ask billing. Without it (nil) every workspace can be deleted.
func (s *Service) SetBilling(b Billing) { s.billing = b }

// CheckDeletion fails while billing stops the workspace from being deleted: a
// live subscription always blocks it, and a lifetime deal blocks it until the
// owner confirms giving the deal up (confirmLifetime). Deleting a workspace
// calls it first.
func (s *Service) CheckDeletion(ctx context.Context, workspaceID uuid.UUID, confirmLifetime bool) error {
	if s.billing == nil {
		return nil
	}
	c, err := s.billing.DeletionCheck(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("billing deletion check: %w", err)
	}
	if c.Allowed {
		return nil
	}
	switch c.Reason {
	case deleteBlockedSubscription:
		return model.ErrDeleteSubscription
	case deleteNeedsLTDConfirm:
		if confirmLifetime {
			return nil
		}
		return model.ErrDeleteLifetime
	}
	return fmt.Errorf("billing blocks deletion for an unknown reason %q", c.Reason)
}
