package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Track T3: catalog.

// Catalog lists every active plan with its active prices, installment offers
// and the enabled providers that sell each one. It needs no workspace: it
// backs the public pricing page.
func (s *Service) Catalog(ctx context.Context) (model.Catalog, error) {
	r := repository.NewRepository(s.pool)
	plans, err := r.ListActivePlans(ctx)
	if err != nil {
		return model.Catalog{}, fmt.Errorf("list plans: %w", err)
	}
	prices, err := r.ListActivePlanPrices(ctx)
	if err != nil {
		return model.Catalog{}, fmt.Errorf("list prices: %w", err)
	}
	offers, err := r.ListActiveInstallmentOffers(ctx)
	if err != nil {
		return model.Catalog{}, fmt.Errorf("list installment offers: %w", err)
	}
	providerPrices, err := r.ListActiveProviderPrices(ctx)
	if err != nil {
		return model.Catalog{}, fmt.Errorf("list provider prices: %w", err)
	}

	// Who sells what, keeping only enabled providers.
	priceProviders := map[uuid.UUID][]payment.Provider{}
	offerProviders := map[uuid.UUID][]model.OfferProvider{}
	for _, pp := range providerPrices {
		p := payment.Provider(pp.Provider)
		if _, ok := s.payments.Get(p); !ok {
			continue
		}
		if pp.InstallmentOfferID == nil {
			priceProviders[pp.PlanPriceID] = append(priceProviders[pp.PlanPriceID], p)
			continue
		}
		offerProviders[*pp.InstallmentOfferID] = append(offerProviders[*pp.InstallmentOfferID],
			model.OfferProvider{Provider: p, Collection: deref(pp.Collection)})
	}
	priceOffers := map[uuid.UUID][]model.InstallmentOffer{}
	for _, o := range offers {
		priceOffers[o.PlanPriceID] = append(priceOffers[o.PlanPriceID], model.InstallmentOffer{
			ID: o.ID, PlanPriceID: o.PlanPriceID, Count: int(o.Count), Interval: o.Interval, TotalMinor: o.TotalMinor,
			GraceDays: int(o.GraceDays), MaxMissed: int(o.MaxMissed), Providers: offerProviders[o.ID],
		})
	}

	out := model.Catalog{Plans: make([]model.Plan, 0, len(plans))}
	byPlan := make(map[uuid.UUID]int, len(plans))
	for _, p := range plans {
		limits, err := model.ParseLimits(p.Limits)
		if err != nil {
			return model.Catalog{}, fmt.Errorf("plan %s: %w", p.ID, err)
		}
		byPlan[p.ID] = len(out.Plans)
		out.Plans = append(out.Plans, model.Plan{
			ID: p.ID, Code: p.Code, Version: int(p.Version), Name: p.Name, Description: p.Description,
			Features: p.Features, Recommended: p.Recommended, TrialDays: int(p.TrialDays),
			TrialRequiresCard: p.TrialRequiresCard, Limits: limits, SortOrder: int(p.SortOrder),
		})
	}
	for _, pr := range prices {
		i, ok := byPlan[pr.PlanID]
		if !ok {
			continue
		}
		plan := &out.Plans[i]
		plan.Prices = append(plan.Prices, model.Price{
			ID: pr.ID, PlanID: pr.PlanID, PlanCode: plan.Code, BillingType: pr.BillingType,
			Interval: deref(pr.Interval), Currency: pr.Currency, AmountMinor: pr.AmountMinor,
			Providers: priceProviders[pr.ID], InstallmentOffers: priceOffers[pr.ID],
		})
	}
	for _, p := range s.payments.Providers() {
		g, _ := s.payments.Get(p)
		out.Providers = append(out.Providers, model.ProviderInfo{Provider: p, Capabilities: g.Capabilities()})
	}
	return out, nil
}

// Price returns one price, active or not, with its plan code and the enabled
// providers that sell it. model.ErrPriceNotFound if there is none.
func (s *Service) Price(ctx context.Context, planPriceID uuid.UUID) (model.Price, error) {
	p, err := s.loadPrice(ctx, repository.NewRepository(s.pool), planPriceID)
	return p.Price, err
}

// InstallmentOffer returns one active installment offer and the providers
// that collect it. model.ErrOfferNotFound if there is none.
func (s *Service) InstallmentOffer(ctx context.Context, offerID uuid.UUID) (model.InstallmentOffer, error) {
	r := repository.NewRepository(s.pool)
	o, err := r.GetInstallmentOffer(ctx, offerID)
	if repository.IsNoRows(err) || (err == nil && !o.IsActive) {
		return model.InstallmentOffer{}, model.ErrOfferNotFound
	}
	if err != nil {
		return model.InstallmentOffer{}, fmt.Errorf("get installment offer: %w", err)
	}
	offer := model.InstallmentOffer{
		ID: o.ID, PlanPriceID: o.PlanPriceID, Count: int(o.Count), Interval: o.Interval, TotalMinor: o.TotalMinor,
		GraceDays: int(o.GraceDays), MaxMissed: int(o.MaxMissed),
	}
	for _, p := range s.payments.Providers() {
		pp, err := r.GetProviderPrice(ctx, gen.GetProviderPriceParams{
			PlanPriceID: o.PlanPriceID, InstallmentOfferID: &o.ID, Provider: string(p),
		})
		if repository.IsNoRows(err) {
			continue
		}
		if err != nil {
			return model.InstallmentOffer{}, fmt.Errorf("get provider price: %w", err)
		}
		offer.Providers = append(offer.Providers, model.OfferProvider{Provider: p, Collection: deref(pp.Collection)})
	}
	return offer, nil
}

// loadedPrice is a price with what checkout and plan changes also need.
type loadedPrice struct {
	model.Price
	PlanName string
	IsActive bool
}

// loadPrice reads a price, active or not, with the enabled providers that sell it, through r (a pool or a tx).
func (s *Service) loadPrice(ctx context.Context, r repository.Repository, planPriceID uuid.UUID) (loadedPrice, error) {
	row, err := r.GetPlanPrice(ctx, planPriceID)
	if repository.IsNoRows(err) {
		return loadedPrice{}, model.ErrPriceNotFound
	}
	if err != nil {
		return loadedPrice{}, fmt.Errorf("get price: %w", err)
	}
	out := loadedPrice{
		Price: model.Price{
			ID: row.ID, PlanID: row.PlanID, PlanCode: row.PlanCode, BillingType: row.BillingType,
			Interval: deref(row.Interval), Currency: row.Currency, AmountMinor: row.AmountMinor,
		},
		PlanName: row.PlanName, IsActive: row.IsActive,
	}
	for _, p := range s.payments.Providers() {
		_, err := r.GetProviderPrice(ctx, gen.GetProviderPriceParams{PlanPriceID: row.ID, Provider: string(p)})
		if repository.IsNoRows(err) {
			continue
		}
		if err != nil {
			return loadedPrice{}, fmt.Errorf("get provider price: %w", err)
		}
		out.Providers = append(out.Providers, p)
	}
	return out, nil
}

// externalPriceID is the provider's ID for a price (offer nil) or an installment offer.
func externalPriceID(ctx context.Context, r repository.Repository, planPriceID uuid.UUID, offerID *uuid.UUID, p payment.Provider) (gen.GetProviderPriceRow, error) {
	pp, err := r.GetProviderPrice(ctx, gen.GetProviderPriceParams{
		PlanPriceID: planPriceID, InstallmentOfferID: offerID, Provider: string(p),
	})
	if repository.IsNoRows(err) {
		return gen.GetProviderPriceRow{}, model.ErrNoProviderForPrice
	}
	if err != nil {
		return gen.GetProviderPriceRow{}, fmt.Errorf("get provider price: %w", err)
	}
	return pp, nil
}

// oneTimeProduct is the provider product a payment-mode checkout of
// planPriceID charges, for providers that need one (Creem); "" when the price
// has none on p, which providers that charge a bare amount (Stripe) accept.
func oneTimeProduct(ctx context.Context, r repository.Repository, planPriceID uuid.UUID, p payment.Provider) (string, error) {
	pp, err := externalPriceID(ctx, r, planPriceID, nil, p)
	if errors.Is(err, model.ErrNoProviderForPrice) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return deref(pp.ExternalPriceID), nil
}

// deref returns *p, or the zero value for nil.
func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
