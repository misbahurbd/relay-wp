package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

// Track T3: discount codes (Q18).

// PreviewDiscount checks a code against a price before checkout and returns
// the reduced price (and reduced installments for an offer), or the model
// error that says why the code doesn't apply.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) PreviewDiscount(ctx context.Context, workspaceID uuid.UUID, in model.DiscountPreviewInput) (model.DiscountPreview, error) {
	return model.DiscountPreview{}, model.ErrNotImplemented
}

// ApplyDiscount validates code for this workspace and price (active, in its
// dates, plan and price type, installments allowed, per-workspace limit) and
// works out the amount off: of the price, or of offer's total when offer is
// set. It reserves nothing. Track T4's QuoteOrder uses it.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) ApplyDiscount(ctx context.Context, workspaceID uuid.UUID, code string, price model.Price, offer *model.InstallmentOffer) (model.AppliedDiscount, error) {
	return model.AppliedDiscount{}, model.ErrNotImplemented
}

// ReserveDiscount holds one use of d for a checkout session in tx, through
// the repository's single-statement ReserveDiscount so max_redemptions holds
// under concurrency. model.ErrDiscountUsedUp when nothing is left.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) ReserveDiscount(ctx context.Context, tx pgx.Tx, workspaceID, sessionID uuid.UUID, d model.AppliedDiscount) error {
	return model.ErrNotImplemented
}

// RedeemDiscount turns a session's reservation into a redemption in tx, once it is paid.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) RedeemDiscount(ctx context.Context, tx pgx.Tx, workspaceID, sessionID uuid.UUID, subscriptionID, orderID *uuid.UUID) error {
	return model.ErrNotImplemented
}

// ReleaseDiscount frees a failed or expired session's reservation in tx and
// gives the use back. A session with none is a no-op.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) ReleaseDiscount(ctx context.Context, tx pgx.Tx, workspaceID, sessionID uuid.UUID) error {
	return model.ErrNotImplemented
}

// CreateDiscount adds a discount code. For the billing discounts CLI command.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) CreateDiscount(ctx context.Context, in model.NewDiscount) (model.Discount, error) {
	return model.Discount{}, model.ErrNotImplemented
}

// ListDiscounts lists discount codes, newest first. For the CLI.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) ListDiscounts(ctx context.Context, activeOnly bool) ([]model.Discount, error) {
	return nil, model.ErrNotImplemented
}

// DeactivateDiscount stops a code from being used again, and its provider
// coupons with it. Redemptions already made stand. For the CLI.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) DeactivateDiscount(ctx context.Context, code string) error {
	return model.ErrNotImplemented
}
