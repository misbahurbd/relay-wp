package service

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

func TestCatalogListsEnabledProvidersOnly(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	r := repository.NewRepository(e.pool)
	lifetime := e.price(p.ID, model.BillingOneTime, "", 30000)
	offerID, err := r.CreateInstallmentOffer(e.ctx, gen.CreateInstallmentOfferParams{
		PlanPriceID: lifetime, Count: 3, TotalMinor: 33000, GraceDays: 7, MaxMissed: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	collection := model.CollectionProviderSchedule
	ext := "price_" + uuid.NewString()
	if _, err := r.CreateProviderPrice(e.ctx, gen.CreateProviderPriceParams{
		PlanPriceID: lifetime, InstallmentOfferID: &offerID, Provider: string(payment.Stripe), ExternalPriceID: &ext,
		Collection: &collection,
	}); err != nil {
		t.Fatal(err)
	}
	// Creem sells the monthly price too, but isn't enabled.
	creemExt := "creem_" + uuid.NewString()
	if _, err := r.CreateProviderPrice(e.ctx, gen.CreateProviderPriceParams{
		PlanPriceID: p.Monthly, Provider: string(payment.Creem), ExternalPriceID: &creemExt,
	}); err != nil {
		t.Fatal(err)
	}

	c, err := e.svc.Catalog(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Providers) != 1 || c.Providers[0].Provider != payment.Stripe {
		t.Fatalf("providers = %+v", c.Providers)
	}
	i := slices.IndexFunc(c.Plans, func(x model.Plan) bool { return x.ID == p.ID })
	if i < 0 {
		t.Fatal("plan missing from catalog")
	}
	plan := c.Plans[i]
	if len(plan.Prices) != 3 || plan.Limits.Of("members") != 10 {
		t.Fatalf("plan = %+v", plan)
	}
	for _, pr := range plan.Prices {
		if !slices.Equal(pr.Providers, []payment.Provider{payment.Stripe}) {
			t.Errorf("price %s providers = %v", pr.ID, pr.Providers)
		}
		if pr.ID == lifetime && (len(pr.InstallmentOffers) != 1 || pr.InstallmentOffers[0].Providers[0].Collection != collection) {
			t.Errorf("lifetime offers = %+v", pr.InstallmentOffers)
		}
	}

	offer, err := e.svc.InstallmentOffer(e.ctx, offerID)
	if err != nil || offer.Count != 3 || len(offer.Providers) != 1 {
		t.Fatalf("offer = %+v, %v", offer, err)
	}
	if _, err := e.svc.InstallmentOffer(e.ctx, uuid.New()); !errors.Is(err, model.ErrOfferNotFound) {
		t.Fatalf("unknown offer: %v", err)
	}
	if _, err := e.svc.Price(e.ctx, uuid.New()); !errors.Is(err, model.ErrPriceNotFound) {
		t.Fatalf("unknown price: %v", err)
	}
}
