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

// needsProductCaps makes the shared fake charge one-off payments through a
// provider product, as Creem does, until the test ends.
func (e *t3Env) needsProductCaps() {
	e.t.Helper()
	caps := e.fake.Capabilities()
	e.t.Cleanup(func() { e.fake.SetCapabilities(caps) })
	creemLike := caps
	creemLike.PaymentNeedsProduct = true
	e.fake.SetCapabilities(creemLike)
}

// On a provider whose one-off payments charge a product (Creem), an app_link
// offer is on sale only once its price has that product.
func TestCatalogHidesAnOfferWithoutItsPriceProduct(t *testing.T) {
	e := newT3Env(t)
	e.needsProductCaps()
	p := e.plan(t3PlanOpts{Monthly: 1000})
	r := repository.NewRepository(e.pool)
	lifetime, err := r.CreatePlanPrice(e.ctx, gen.CreatePlanPriceParams{
		PlanID: p.ID, BillingType: model.BillingOneTime, Currency: "USD", AmountMinor: 30000,
	})
	if err != nil {
		t.Fatal(err)
	}
	offerID, err := r.CreateInstallmentOffer(e.ctx, gen.CreateInstallmentOfferParams{
		PlanPriceID: lifetime, Count: 3, TotalMinor: 33000, GraceDays: 7, MaxMissed: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	link := model.CollectionAppLink
	if _, err := r.CreateProviderPrice(e.ctx, gen.CreateProviderPriceParams{
		PlanPriceID: lifetime, InstallmentOfferID: &offerID, Provider: string(payment.Stripe), Collection: &link,
	}); err != nil {
		t.Fatal(err)
	}
	offerSellers := func() []model.OfferProvider {
		c, err := e.svc.Catalog(e.ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, pl := range c.Plans {
			for _, pr := range pl.Prices {
				for _, o := range pr.InstallmentOffers {
					if o.ID == offerID {
						return o.Providers
					}
				}
			}
		}
		t.Fatal("offer not in the catalog")
		return nil
	}
	if got := offerSellers(); len(got) != 0 {
		t.Fatalf("offer sold by %+v without its price's product", got)
	}
	if _, err := e.svc.CreateCheckout(e.ctx, e.ws, e.user, model.CheckoutInput{
		PlanPriceID: lifetime, InstallmentOfferID: &offerID, IdempotencyKey: uuid.NewString(),
	}); !errors.Is(err, model.ErrNoProviderForPrice) {
		t.Fatalf("checkout without the product: err = %v", err)
	}

	ext := "prod_" + uuid.NewString()
	if _, err := r.CreateProviderPrice(e.ctx, gen.CreateProviderPriceParams{
		PlanPriceID: lifetime, Provider: string(payment.Stripe), ExternalPriceID: &ext,
	}); err != nil {
		t.Fatal(err)
	}
	if got := offerSellers(); len(got) != 1 || got[0].Collection != model.CollectionAppLink {
		t.Fatalf("offer sellers with the product = %+v", got)
	}
}
