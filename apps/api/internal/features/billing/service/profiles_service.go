package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

// Track T3: billing profiles. Personal data: never log a profile.

// Profile returns the workspace's billing details. model.ErrProfileNotFound if
// none are saved yet.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) Profile(ctx context.Context, workspaceID uuid.UUID) (model.Profile, error) {
	return model.Profile{}, model.ErrNotImplemented
}

// UpdateProfile replaces the workspace's billing details. Past invoices keep
// the bill_to they were issued with.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) UpdateProfile(ctx context.Context, workspaceID uuid.UUID, in model.ProfileInput) (model.Profile, error) {
	return model.Profile{}, model.ErrNotImplemented
}

// BillTo copies the workspace's billing details for a new invoice's bill_to,
// in tx. Every track that issues an invoice uses it.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) BillTo(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID) (model.BillTo, error) {
	return model.BillTo{}, model.ErrNotImplemented
}
