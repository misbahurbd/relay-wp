package service

import (
	"context"
	"fmt"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// Permissions returns the catalog in editor order.
func (s *Service) Permissions(ctx context.Context) ([]model.Permission, error) {
	rows, err := repository.NewRepository(s.pool).ListPermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	out := make([]model.Permission, len(rows))
	for i, p := range rows {
		out[i] = model.Permission{Key: p.Key, Group: p.GroupKey, Label: p.Label, Sensitive: p.Sensitive}
	}
	return out, nil
}

// Seats returns the plan's seats and how many are used or held by pending invites.
func (s *Service) Seats(ctx context.Context, workspaceID uuid.UUID) (model.Seats, error) {
	var out model.Seats
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		out, err = s.seats(ctx, tx, workspaceID)
		return err
	})
	return out, err
}

// seats locks the workspace row; call it in the transaction that adds seats.
func (s *Service) seats(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID) (model.Seats, error) {
	limit, err := s.workspaces.LockSeatLimit(ctx, tx, workspaceID)
	if err != nil {
		return model.Seats{}, err
	}
	used, err := repository.NewRepository(tx).CountSeatsUsed(ctx, workspaceID)
	if err != nil {
		return model.Seats{}, fmt.Errorf("count seats: %w", err)
	}
	return model.Seats{
		Limit: limit, Used: used.Members, Reserved: used.Reserved,
		Available: max(0, int64(limit)-used.Members-used.Reserved),
	}, nil
}
