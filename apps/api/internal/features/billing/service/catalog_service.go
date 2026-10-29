package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

// Track T3: catalog.

// Catalog lists every active plan with its active prices, installment offers
// and the enabled providers that sell each one. It needs no workspace: it
// backs the public pricing page.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) Catalog(ctx context.Context) (model.Catalog, error) {
	return model.Catalog{}, model.ErrNotImplemented
}

// Price returns one price, active or not, with its plan code and the enabled
// providers that sell it. model.ErrPriceNotFound if there is none.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) Price(ctx context.Context, planPriceID uuid.UUID) (model.Price, error) {
	return model.Price{}, model.ErrNotImplemented
}

// InstallmentOffer returns one active installment offer and the providers
// that collect it. model.ErrOfferNotFound if there is none.
//
//nolint:revive // stub: track T3 builds it
func (s *Service) InstallmentOffer(ctx context.Context, offerID uuid.UUID) (model.InstallmentOffer, error) {
	return model.InstallmentOffer{}, model.ErrNotImplemented
}
