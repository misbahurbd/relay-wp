package service

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

func TestAdminBuildsASellableCatalog(t *testing.T) {
	e := newT3Env(t)
	code := "adm_" + uuid.NewString()[:8]
	planID, err := e.svc.CreatePlan(e.ctx, model.NewPlan{Code: code, PlanDetails: model.PlanDetails{
		Name: "Admin plan", TrialDays: 14, Limits: model.Limits{"members": 10, "whatsapp_numbers": model.Unlimited},
	}})
	if err != nil {
		t.Fatal(err)
	}
	monthly, err := e.svc.CreatePrice(e.ctx, model.NewPrice{
		PlanID: planID, BillingType: model.BillingRecurring, Interval: model.IntervalMonth, Currency: "usd", AmountMinor: 2900,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.CreatePrice(e.ctx, model.NewPrice{
		PlanID: planID, BillingType: model.BillingRecurring, Interval: model.IntervalMonth, Currency: "USD", AmountMinor: 3900,
	}); !errors.Is(err, model.ErrCatalogDuplicate) {
		t.Fatalf("second active monthly USD price: err = %v", err)
	}
	ltd, err := e.svc.CreatePrice(e.ctx, model.NewPrice{
		PlanID: planID, BillingType: model.BillingOneTime, Currency: "USD", AmountMinor: 29900,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.CreateInstallmentOffer(e.ctx, model.NewInstallmentOffer{
		PlanPriceID: monthly, Count: 3, TotalMinor: 9000, GraceDays: 7, MaxMissed: 2,
	}); !isInvalidInput(err) {
		t.Fatalf("installments on a recurring price: err = %v", err)
	}
	offer, err := e.svc.CreateInstallmentOffer(e.ctx, model.NewInstallmentOffer{
		PlanPriceID: ltd, Count: 3, TotalMinor: 31000, GraceDays: 7, MaxMissed: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	ext := "price_" + uuid.NewString()
	if _, err := e.svc.CreateProviderPrice(e.ctx, model.NewProviderPrice{
		PlanPriceID: monthly, Provider: string(payment.Stripe), ExternalPriceID: &ext,
	}); err != nil {
		t.Fatal(err)
	}
	link := model.CollectionAppLink
	if _, err := e.svc.CreateProviderPrice(e.ctx, model.NewProviderPrice{
		PlanPriceID: ltd, InstallmentOfferID: &offer, Provider: string(payment.Stripe),
	}); !isInvalidInput(err) {
		t.Fatalf("offer without a collection: err = %v", err)
	}
	if _, err := e.svc.CreateProviderPrice(e.ctx, model.NewProviderPrice{
		PlanPriceID: monthly, InstallmentOfferID: &offer, Provider: string(payment.Stripe), Collection: &link,
	}); !errors.Is(err, model.ErrOfferNotFound) {
		t.Fatalf("offer of another price: err = %v", err)
	}

	// The public catalog sells the monthly price; the one-time one has no provider yet.
	c, err := e.svc.Catalog(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	var sold bool
	for _, p := range c.Plans {
		for _, pr := range p.Prices {
			if pr.ID == monthly && len(pr.Providers) == 1 {
				sold = true
			}
		}
	}
	if !sold {
		t.Fatal("the monthly price should be on sale through Stripe")
	}

	// A used code publishes the next version.
	v2, err := e.svc.CreatePlan(e.ctx, model.NewPlan{Code: code, PlanDetails: details("Admin plan v2")})
	if err != nil {
		t.Fatal(err)
	}
	ac, err := e.svc.AdminCatalog(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	versions := map[uuid.UUID]int{}
	for _, p := range ac.Plans {
		if p.Code == code {
			versions[p.ID] = p.Version
			if p.ID == planID && (len(p.Prices) != 2 || len(p.Prices[0].ProviderPrices)+len(p.Prices[1].ProviderPrices) != 1) {
				t.Errorf("v1 prices = %+v", p.Prices)
			}
		}
	}
	if versions[planID] != 1 || versions[v2] != 2 {
		t.Fatalf("versions = %v", versions)
	}

	if err := e.svc.DeactivateCatalogRow(e.ctx, CatalogPrice, monthly); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.DeactivateCatalogRow(e.ctx, CatalogPrice, monthly); !errors.Is(err, model.ErrCatalogRowNotFound) {
		t.Fatalf("deactivate twice: err = %v", err)
	}
	if _, err := e.svc.CreatePlan(e.ctx, model.NewPlan{Code: "free", PlanDetails: details("Free")}); !isInvalidInput(err) {
		t.Fatalf("a catalog plan called free: err = %v", err)
	}
}

func TestAdminGrantsAPlanByHand(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 2900})

	if err := e.svc.GrantPlanManually(e.ctx, model.ManualGrantInput{
		WorkspaceID: e.ws, PlanID: p.ID, CreatedBy: e.user,
	}); !isInvalidInput(err) {
		t.Fatalf("no reason: err = %v", err)
	}
	if err := e.svc.GrantPlanManually(e.ctx, model.ManualGrantInput{
		WorkspaceID: uuid.New(), PlanID: p.ID, Reason: "comp", CreatedBy: e.user,
	}); !errors.Is(err, model.ErrWorkspaceNotFound) {
		t.Fatalf("unknown workspace: err = %v", err)
	}
	if err := e.svc.GrantPlanManually(e.ctx, model.ManualGrantInput{
		WorkspaceID: e.ws, PlanID: p.ID, Reason: "partner comp", CreatedBy: e.user,
	}); err != nil {
		t.Fatal(err)
	}
	e.wantCache(e.ws, p.Code)
	ws, err := e.svc.AdminWorkspaces(e.ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	var listed bool
	for _, w := range ws {
		if w.ID == e.ws {
			listed = w.Grant != nil && w.Grant.Source == model.GrantManual && w.Grant.PlanCode == p.Code
		}
	}
	if !listed {
		t.Fatal("the workspace should be listed on its manual grant")
	}
	if err := e.svc.RestoreGrantManually(e.ctx, e.ws); !errors.Is(err, model.ErrGrantNotSuspended) {
		t.Fatalf("restore an active grant: err = %v", err)
	}
	if err := e.svc.EndGrantManually(e.ctx, e.ws); err != nil {
		t.Fatal(err)
	}
	e.wantCache(e.ws, model.FreePlan)
	if err := e.svc.EndGrantManually(e.ctx, e.ws); !errors.Is(err, model.ErrNoGrant) {
		t.Fatalf("end on free: err = %v", err)
	}

	// A paid plan is billing's: staff can't replace or end it.
	e.subscribe(p.Monthly)
	if err := e.svc.GrantPlanManually(e.ctx, model.ManualGrantInput{
		WorkspaceID: e.ws, PlanID: p.ID, Reason: "comp", CreatedBy: e.user,
	}); !errors.Is(err, model.ErrGrantFromPayment) {
		t.Fatalf("over a subscription: err = %v", err)
	}
	if err := e.svc.EndGrantManually(e.ctx, e.ws); !errors.Is(err, model.ErrGrantFromPayment) {
		t.Fatalf("end a subscription grant: err = %v", err)
	}
}

// isInvalidInput matches ErrInvalidInput with or without details.
func isInvalidInput(err error) bool {
	e, ok := apperr.As(err)
	return ok && e.Code == model.ErrInvalidInput.Code
}

func TestAdminPlanCarriesItsPricingTableDetails(t *testing.T) {
	e := newT3Env(t)
	code := "adm_" + uuid.NewString()[:8]
	d := details("Detailed")
	d.Description, d.Recommended = "  For growing teams.  ", true
	d.Features = []string{" 10,000 contacts ", "", "API access"}
	id, err := e.svc.CreatePlan(e.ctx, model.NewPlan{Code: code, PlanDetails: d})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := e.svc.AdminCatalog(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got *model.AdminPlan
	for i := range cat.Plans {
		if cat.Plans[i].ID == id {
			got = &cat.Plans[i]
		}
	}
	if got == nil || got.Description != "For growing teams." || !got.Recommended ||
		len(got.Features) != 2 || got.Features[0] != "10,000 contacts" || got.Features[1] != "API access" {
		t.Fatalf("plan = %+v", got)
	}

	tooMany := make([]string, 21)
	for i := range tooMany {
		tooMany[i] = "feature"
	}
	for name, edit := range map[string]func(*model.PlanDetails){
		"long description": func(d *model.PlanDetails) { d.Description = strings.Repeat("a", 201) },
		"too many":         func(d *model.PlanDetails) { d.Features = tooMany },
		"long feature":     func(d *model.PlanDetails) { d.Features = []string{strings.Repeat("a", 81)} },
		"no members":       func(d *model.PlanDetails) { delete(d.Limits, "members") },
	} {
		d := details("X")
		edit(&d)
		if _, err := e.svc.CreatePlan(e.ctx, model.NewPlan{Code: code, PlanDetails: d}); !isInvalidInput(err) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestPlanLimitsAreLiveAndOverridable(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 2900, Limits: `{"members": 5, "whatsapp_numbers": 1}`})
	if err := e.svc.GrantPlanManually(e.ctx, model.ManualGrantInput{
		WorkspaceID: e.ws, PlanID: p.ID, Reason: "partner comp", CreatedBy: e.user,
	}); err != nil {
		t.Fatal(err)
	}
	limit := func(key string) int {
		t.Helper()
		n, err := e.svc.Limit(e.ctx, e.ws, key)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	edit := details("Renamed")
	edit.Limits = model.Limits{"members": 8, "whatsapp_numbers": 2, "automations": model.Included}
	if err := e.svc.UpdatePlan(e.ctx, model.PlanEdit{
		PlanID: p.ID, PlanDetails: edit, Reason: "launch pricing", ChangedBy: e.user,
	}); err != nil {
		t.Fatal(err)
	}
	if limit("members") != 8 || limit("whatsapp_numbers") != 2 || limit("automations") != model.Included {
		t.Fatal("an existing grant should get the plan's edited limits")
	}
	if limit("contacts") != model.NotIncluded {
		t.Error("a resource the plan leaves out should not be included")
	}
	edit.Name = "Renamed again"
	if err := e.svc.UpdatePlan(e.ctx, model.PlanEdit{PlanID: p.ID, PlanDetails: edit, ChangedBy: e.user}); err != nil {
		t.Fatalf("a rename needs no reason: %v", err)
	}

	if err := e.svc.SetGrantOverrides(e.ctx, model.GrantOverridesEdit{
		WorkspaceID: e.ws, Overrides: model.Limits{"members": 20, "automations": model.NotIncluded},
		Reason: "bigger team", ChangedBy: e.user,
	}); err != nil {
		t.Fatal(err)
	}
	if limit("members") != 20 || limit("whatsapp_numbers") != 2 || limit("automations") != model.NotIncluded {
		t.Fatal("an override should win only for its own key")
	}

	var changes int
	if err := e.pool.QueryRow(e.ctx, `SELECT count(*) FROM limit_changes WHERE plan_id = $1 OR grant_id = $2`,
		p.ID, e.grant(e.ws).ID).Scan(&changes); err != nil {
		t.Fatal(err)
	}
	if changes != 2 {
		t.Errorf("limit changes = %d, want one per edit", changes)
	}

	// A new plan starts without the old grant's overrides.
	other := e.plan(t3PlanOpts{Monthly: 3900, Limits: `{"members": 10, "whatsapp_numbers": 1}`})
	if err := e.svc.GrantPlanManually(e.ctx, model.ManualGrantInput{
		WorkspaceID: e.ws, PlanID: other.ID, Reason: "moved", CreatedBy: e.user,
	}); err != nil {
		t.Fatal(err)
	}
	if limit("members") != 10 {
		t.Error("overrides should end with their grant")
	}
}

func TestLimitEditsAreChecked(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 2900})
	for name, c := range map[string]struct {
		limits model.Limits
		reason string
	}{
		"unknown key": {model.Limits{"members": 1, "whatsapp_numbers": 1, "memebrs": 3}, "typo"},
		"below -1":    {model.Limits{"members": -2, "whatsapp_numbers": 1}, "oops"},
		"no reason":   {model.Limits{"members": 3, "whatsapp_numbers": 1}, ""},
	} {
		d := details("X")
		d.Limits = c.limits
		if err := e.svc.UpdatePlan(e.ctx, model.PlanEdit{PlanID: p.ID, PlanDetails: d, Reason: c.reason, ChangedBy: e.user}); !isInvalidInput(err) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if err := e.svc.UpdatePlan(e.ctx, model.PlanEdit{
		PlanID: uuid.New(), PlanDetails: details("X"), Reason: "x", ChangedBy: e.user,
	}); !errors.Is(err, model.ErrPlanNotFound) {
		t.Errorf("unknown plan: err = %v", err)
	}
	if err := e.svc.SetGrantOverrides(e.ctx, model.GrantOverridesEdit{
		WorkspaceID: e.ws, Overrides: model.Limits{"members": 0}, Reason: "x", ChangedBy: e.user,
	}); !isInvalidInput(err) {
		t.Errorf("override members away: err = %v", err)
	}
	if err := e.svc.SetGrantOverrides(e.ctx, model.GrantOverridesEdit{
		WorkspaceID: e.ws, Overrides: model.Limits{"members": 3}, Reason: "x", ChangedBy: e.user,
	}); !errors.Is(err, model.ErrNoGrant) {
		t.Errorf("override on the free plan: err = %v", err)
	}
	if _, err := e.svc.Limit(e.ctx, e.ws, "seats"); err == nil {
		t.Error("a key outside the catalog should be an error")
	}

	var freeID uuid.UUID
	if err := e.pool.QueryRow(e.ctx, `SELECT id FROM plans WHERE code = 'free'`).Scan(&freeID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.GrantPlanManually(e.ctx, model.ManualGrantInput{
		WorkspaceID: e.ws, PlanID: freeID, Reason: "x", CreatedBy: e.user,
	}); !isInvalidInput(err) {
		t.Errorf("grant the free plan: err = %v", err)
	}
	if _, err := e.svc.CreatePrice(e.ctx, model.NewPrice{
		PlanID: freeID, BillingType: model.BillingRecurring, Interval: model.IntervalMonth, Currency: "USD", AmountMinor: 100,
	}); !isInvalidInput(err) {
		t.Errorf("price the free plan: err = %v", err)
	}
}

// details are valid plan details named name: the required resources and nothing else.
func details(name string) model.PlanDetails {
	return model.PlanDetails{Name: name, Limits: model.Limits{"members": 5, "whatsapp_numbers": 1}}
}

// pricesFor is the fake provider's create-price requests for one plan price.
func (e *t3Env) pricesFor(id uuid.UUID) []payment.PriceRequest {
	var out []payment.PriceRequest
	for _, p := range e.fake.Prices() {
		if p.PlanPriceID == id {
			out = append(out, p)
		}
	}
	return out
}

func TestAdminCreatesProviderPricesOnTheProvider(t *testing.T) {
	e := newT3Env(t)
	d := details("Growth")
	d.TrialDays, d.TrialRequiresCard = 14, true
	planID, err := e.svc.CreatePlan(e.ctx, model.NewPlan{Code: "adm_" + uuid.NewString()[:8], PlanDetails: d})
	if err != nil {
		t.Fatal(err)
	}
	newPrice := func(p model.NewPrice) uuid.UUID {
		t.Helper()
		p.PlanID, p.Currency = planID, "USD"
		id, err := e.svc.CreatePrice(e.ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	monthly := newPrice(model.NewPrice{BillingType: model.BillingRecurring, Interval: model.IntervalMonth, AmountMinor: 2900})
	yearly := newPrice(model.NewPrice{BillingType: model.BillingRecurring, Interval: model.IntervalYear, AmountMinor: 29000})
	ltd := newPrice(model.NewPrice{BillingType: model.BillingOneTime, AmountMinor: 29900})
	stripe := string(payment.Stripe)

	if _, err := e.svc.CreateProviderPrice(e.ctx, model.NewProviderPrice{PlanPriceID: monthly, Provider: stripe}); err != nil {
		t.Fatal(err)
	}
	got := e.pricesFor(monthly)
	want := payment.PriceRequest{
		PlanPriceID: monthly, Name: "Relay Growth (monthly)", AmountMinor: 2900, Currency: "USD", Interval: "month",
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("monthly request = %+v, want %+v", got, want)
	}
	row, err := repository.NewRepository(e.pool).GetProviderPrice(e.ctx, gen.GetProviderPriceParams{PlanPriceID: monthly, Provider: stripe})
	if err != nil {
		t.Fatal(err)
	}
	if id := deref(row.ExternalPriceID); !strings.HasPrefix(id, "fake_price_") {
		t.Fatalf("stored external id = %q, want the one the provider returned", id)
	}

	for _, c := range []struct {
		price uuid.UUID
		name  string
		inter string
	}{{yearly, "Relay Growth (yearly)", "year"}, {ltd, "Relay Growth (lifetime)", ""}} {
		if _, err := e.svc.CreateProviderPrice(e.ctx, model.NewProviderPrice{PlanPriceID: c.price, Provider: stripe}); err != nil {
			t.Fatal(err)
		}
		if got := e.pricesFor(c.price); len(got) != 1 || got[0].Name != c.name || got[0].Interval != c.inter {
			t.Errorf("%s request = %+v", c.name, got)
		}
	}

	if _, err := e.svc.CreateProviderPrice(e.ctx, model.NewProviderPrice{PlanPriceID: monthly, Provider: stripe}); !errors.Is(err, model.ErrCatalogDuplicate) {
		t.Fatalf("second active link: err = %v", err)
	}
	if n := len(e.pricesFor(monthly)); n != 1 {
		t.Fatalf("a refused duplicate still reached the provider: %d requests", n)
	}

	if _, err := e.svc.CreateProviderPrice(e.ctx, model.NewProviderPrice{PlanPriceID: monthly, Provider: string(payment.Creem)}); !isInvalidInput(err) {
		t.Fatalf("provider not enabled: err = %v", err)
	}
	if _, err := e.svc.CreateProviderPrice(e.ctx, model.NewProviderPrice{PlanPriceID: uuid.New(), Provider: stripe}); !errors.Is(err, model.ErrPriceNotFound) {
		t.Fatalf("unknown price: err = %v", err)
	}
}

func TestAdminProviderPriceKeepsTheManualPath(t *testing.T) {
	e := newT3Env(t)
	d := details("Starter")
	d.TrialDays = 7
	planID, err := e.svc.CreatePlan(e.ctx, model.NewPlan{Code: "adm_" + uuid.NewString()[:8], PlanDetails: d})
	if err != nil {
		t.Fatal(err)
	}
	monthly, err := e.svc.CreatePrice(e.ctx, model.NewPrice{
		PlanID: planID, BillingType: model.BillingRecurring, Interval: model.IntervalMonth, Currency: "USD", AmountMinor: 1900,
	})
	if err != nil {
		t.Fatal(err)
	}
	ltd, err := e.svc.CreatePrice(e.ctx, model.NewPrice{PlanID: planID, BillingType: model.BillingOneTime, Currency: "USD", AmountMinor: 19900})
	if err != nil {
		t.Fatal(err)
	}
	stripe := string(payment.Stripe)

	manual := "price_" + uuid.NewString()
	if _, err := e.svc.CreateProviderPrice(e.ctx, model.NewProviderPrice{PlanPriceID: monthly, Provider: stripe, ExternalPriceID: &manual}); err != nil {
		t.Fatal(err)
	}
	if n := len(e.pricesFor(monthly)); n != 0 {
		t.Fatalf("a pasted ID asked the provider to create %d prices", n)
	}
	blank := "  "
	if _, err := e.svc.CreateProviderPrice(e.ctx, model.NewProviderPrice{PlanPriceID: monthly, Provider: stripe, ExternalPriceID: &blank}); !errors.Is(err, model.ErrCatalogDuplicate) {
		t.Fatalf("a blank ID is the create path, and the price is already linked: err = %v", err)
	}

	if _, err := e.svc.CreateProviderPrice(e.ctx, model.NewProviderPrice{PlanPriceID: ltd, Provider: stripe, ExternalPriceID: &blank}); err != nil {
		t.Fatal(err)
	}
	if got := e.pricesFor(ltd); len(got) != 1 {
		t.Fatalf("ltd requests = %+v", got)
	}

	offer, err := e.svc.CreateInstallmentOffer(e.ctx, model.NewInstallmentOffer{
		PlanPriceID: ltd, Count: 3, TotalMinor: 21000, GraceDays: 7, MaxMissed: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	link := model.CollectionAppLink
	if _, err := e.svc.CreateProviderPrice(e.ctx, model.NewProviderPrice{
		PlanPriceID: ltd, InstallmentOfferID: &offer, Provider: stripe, Collection: &link,
	}); err != nil {
		t.Fatal(err)
	}
	if n := len(e.pricesFor(ltd)); n != 1 {
		t.Fatalf("an installment offer link changed the provider price count to %d, want the 1 from the plain link", n)
	}
}

func TestAdminProviderPriceFailuresLeaveNoLink(t *testing.T) {
	e := newT3Env(t)
	planID, err := e.svc.CreatePlan(e.ctx, model.NewPlan{Code: "adm_" + uuid.NewString()[:8], PlanDetails: details("Fail")})
	if err != nil {
		t.Fatal(err)
	}
	price, err := e.svc.CreatePrice(e.ctx, model.NewPrice{
		PlanID: planID, BillingType: model.BillingRecurring, Interval: model.IntervalMonth, Currency: "USD", AmountMinor: 900,
	})
	if err != nil {
		t.Fatal(err)
	}
	in := model.NewProviderPrice{PlanPriceID: price, Provider: string(payment.Stripe)}

	e.fake.FailNext(payment.ErrUnavailable)
	if _, err := e.svc.CreateProviderPrice(e.ctx, in); !errors.Is(err, model.ErrPaymentProviderDown) {
		t.Fatalf("provider down: err = %v", err)
	}
	e.fake.FailNext(payment.ErrInvalidRequest)
	if _, err := e.svc.CreateProviderPrice(e.ctx, in); !isInvalidInput(err) {
		t.Fatalf("provider refused: err = %v", err)
	}
	if _, err := e.svc.CreateProviderPrice(e.ctx, in); err != nil {
		t.Fatalf("retry after failures: %v", err)
	}

	other, err := e.svc.CreatePrice(e.ctx, model.NewPrice{
		PlanID: planID, BillingType: model.BillingRecurring, Interval: model.IntervalYear, Currency: "USD", AmountMinor: 9000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.DeactivateCatalogRow(e.ctx, CatalogPrice, other); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.CreateProviderPrice(e.ctx, model.NewProviderPrice{PlanPriceID: other, Provider: string(payment.Stripe)}); !isInvalidInput(err) {
		t.Fatalf("inactive price: err = %v", err)
	}
	if n := len(e.pricesFor(other)); n != 0 {
		t.Fatalf("an inactive price reached the provider: %d", n)
	}
}

// On a provider whose one-off payments charge a product (Creem), linking an
// app_link offer creates the price's product too, once.
func TestAdminOfferLinkCreatesThePriceProduct(t *testing.T) {
	e := newT3Env(t)
	e.needsProductCaps()
	planID, err := e.svc.CreatePlan(e.ctx, model.NewPlan{Code: "adm_" + uuid.NewString()[:8], PlanDetails: details("Offer")})
	if err != nil {
		t.Fatal(err)
	}
	ltd, err := e.svc.CreatePrice(e.ctx, model.NewPrice{PlanID: planID, BillingType: model.BillingOneTime, Currency: "USD", AmountMinor: 19900})
	if err != nil {
		t.Fatal(err)
	}
	offer, err := e.svc.CreateInstallmentOffer(e.ctx, model.NewInstallmentOffer{
		PlanPriceID: ltd, Count: 3, TotalMinor: 21000, GraceDays: 7, MaxMissed: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	stripe, link := string(payment.Stripe), model.CollectionAppLink
	in := model.NewProviderPrice{PlanPriceID: ltd, InstallmentOfferID: &offer, Provider: stripe, Collection: &link}
	if _, err := e.svc.CreateProviderPrice(e.ctx, in); err != nil {
		t.Fatal(err)
	}
	if got := e.pricesFor(ltd); len(got) != 1 || got[0].Name != "Relay Offer (lifetime)" || got[0].AmountMinor != 19900 {
		t.Fatalf("provider requests = %+v, want the lifetime product", got)
	}
	r := repository.NewRepository(e.pool)
	if pp, err := r.GetProviderPrice(e.ctx, gen.GetProviderPriceParams{PlanPriceID: ltd, Provider: stripe}); err != nil ||
		!strings.HasPrefix(deref(pp.ExternalPriceID), "fake_price_") {
		t.Fatalf("plain link = %+v, %v", pp, err)
	}

	if _, err := e.svc.CreateProviderPrice(e.ctx, in); !errors.Is(err, model.ErrCatalogDuplicate) {
		t.Fatalf("second offer link: err = %v", err)
	}
	if n := len(e.pricesFor(ltd)); n != 1 {
		t.Fatalf("a refused link still reached the provider: %d requests", n)
	}
}

// Creem reports no dispute outcome, so staff record it: won pays the invoice
// again, lost suspends the plan it paid for, and each decision stays on record.
func TestStaffClosesDisputes(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	e.subscribe(p.Monthly)
	var invoiceID uuid.UUID
	if err := e.pool.QueryRow(e.ctx, `SELECT id FROM invoices WHERE workspace_id = $1`, e.ws).Scan(&invoiceID); err != nil {
		t.Fatal(err)
	}
	staff := uuid.New()
	closeIt := func(won bool) error {
		return e.svc.CloseDispute(e.ctx, model.DisputeClose{InvoiceID: invoiceID, Won: won, Reason: "Bank decided", ClosedBy: staff})
	}
	status := func() string {
		var s string
		if err := e.pool.QueryRow(e.ctx, `SELECT status FROM invoices WHERE id = $1`, invoiceID).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	if err := closeIt(true); !errors.Is(err, model.ErrNotDisputed) {
		t.Fatalf("closing an undisputed invoice: %v", err)
	}
	e.exec(`UPDATE invoices SET status = 'disputed' WHERE id = $1`, invoiceID)
	listed, err := e.svc.DisputedInvoices(e.ctx)
	if err != nil || !slices.ContainsFunc(listed, func(d model.DisputedInvoice) bool { return d.ID == invoiceID }) {
		t.Fatalf("disputed invoice not listed: %v", err)
	}
	if err := e.svc.CloseDispute(e.ctx, model.DisputeClose{InvoiceID: invoiceID, Won: true, ClosedBy: staff}); !isInvalidInput(err) {
		t.Fatalf("no reason: %v", err)
	}

	if err := closeIt(true); err != nil {
		t.Fatal(err)
	}
	if got := status(); got != model.InvoicePaid {
		t.Fatalf("won dispute: invoice %s", got)
	}
	e.wantCache(e.ws, p.Code)

	e.exec(`UPDATE invoices SET status = 'disputed' WHERE id = $1`, invoiceID)
	if err := closeIt(false); err != nil {
		t.Fatal(err)
	}
	if g := e.grant(e.ws); g == nil || g.Status != model.GrantSuspended || !g.SuspendedFor(model.SuspendDispute) {
		t.Fatalf("lost dispute: grant %+v", g)
	}
	e.wantCache(e.ws, model.FreePlan)

	var recorded int
	if err := e.pool.QueryRow(e.ctx, `SELECT count(*) FROM payment_events WHERE workspace_id = $1
		AND event_type = 'dispute.closed' AND processed_at IS NOT NULL AND payload->>'closed_by' = $2`, e.ws, staff.String()).Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded != 2 {
		t.Fatalf("%d staff decisions on record, want 2", recorded)
	}
}

// A lost dispute is final: the invoice leaves the open-dispute list and
// staff can't close it again as won, which would mark it paid while the
// money is gone and the subscription canceled (N1).
func TestLostDisputeIsFinal(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	e.subscribe(p.Monthly)
	var invoiceID uuid.UUID
	if err := e.pool.QueryRow(e.ctx, `SELECT id FROM invoices WHERE workspace_id = $1`, e.ws).Scan(&invoiceID); err != nil {
		t.Fatal(err)
	}
	e.exec(`UPDATE invoices SET status = 'disputed' WHERE id = $1`, invoiceID)
	closeIt := func(won bool) error {
		return e.svc.CloseDispute(e.ctx, model.DisputeClose{InvoiceID: invoiceID, Won: won, Reason: "Bank decided", ClosedBy: uuid.New()})
	}
	if err := closeIt(false); err != nil {
		t.Fatal(err)
	}

	listed, err := e.svc.DisputedInvoices(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(listed, func(d model.DisputedInvoice) bool { return d.ID == invoiceID }) {
		t.Error("a lost dispute is still listed as open")
	}
	if err := closeIt(true); !errors.Is(err, model.ErrNotDisputed) {
		t.Errorf("closing a lost dispute again as won: err = %v, want ErrNotDisputed", err)
	}
	inv, err := e.svc.Invoice(e.ctx, e.ws, invoiceID)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Status != model.InvoiceDisputeLost {
		t.Errorf("invoice %s after a lost dispute, want %s", inv.Status, model.InvoiceDisputeLost)
	}
	e.wantCache(e.ws, model.FreePlan)
}

// A lost dispute cancels the subscription, so restoring its suspended grant
// would hand out a plan nobody bills again. Staff can't; they can grant a
// plan by hand instead, now that no payment owns the workspace's plan (G2c).
func TestStaffCannotRestoreAPlanWhoseSubscriptionEnded(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	e.subscribe(p.Monthly)
	var invoiceID uuid.UUID
	if err := e.pool.QueryRow(e.ctx, `SELECT id FROM invoices WHERE workspace_id = $1`, e.ws).Scan(&invoiceID); err != nil {
		t.Fatal(err)
	}
	e.exec(`UPDATE invoices SET status = 'disputed' WHERE id = $1`, invoiceID)
	if err := e.svc.CloseDispute(e.ctx, model.DisputeClose{
		InvoiceID: invoiceID, Reason: "Bank decided", ClosedBy: uuid.New(),
	}); err != nil {
		t.Fatal(err)
	}

	if err := e.svc.RestoreGrantManually(e.ctx, e.ws); !errors.Is(err, model.ErrGrantSubscriptionEnded) {
		t.Fatalf("restore over a canceled subscription: err = %v, want ErrGrantSubscriptionEnded", err)
	}
	e.wantCache(e.ws, model.FreePlan)
	ws, err := e.svc.AdminWorkspaces(e.ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range ws {
		if w.ID == e.ws && (w.Grant == nil || w.Grant.Billed || !slices.Equal(w.Grant.SuspendReasons, []string{model.SuspendDispute})) {
			t.Errorf("CRM row grant = %+v, want unbilled and suspended for the dispute", w.Grant)
		}
	}

	ends := e.clock.Add(30 * 24 * time.Hour)
	if err := e.svc.GrantPlanManually(e.ctx, model.ManualGrantInput{
		WorkspaceID: e.ws, PlanID: p.ID, EndsAt: &ends, Reason: "settled with the customer", CreatedBy: e.user,
	}); err != nil {
		t.Fatalf("manual grant over an ended subscription's grant: %v", err)
	}
	e.wantCache(e.ws, p.Code)
}

// A plan publish the CRM retries (a reload, a lost reply) finds what it
// already created instead of adding a second version or price.
func TestPlanPublishRetryCreatesNothingTwice(t *testing.T) {
	e := newT3Env(t)
	key := uuid.New()
	in := model.NewPlan{Code: "pub_" + uuid.NewString()[:8], PublishKey: &key, PlanDetails: details("Publish")}
	first, err := e.svc.CreatePlan(e.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.svc.CreatePlan(e.ctx, in)
	if err != nil || again != first {
		t.Fatalf("retried plan = %s, %v; want %s", again, err, first)
	}
	other := in
	other.Code = "pub_" + uuid.NewString()[:8]
	if _, err := e.svc.CreatePlan(e.ctx, other); !errors.Is(err, model.ErrCatalogDuplicate) {
		t.Fatalf("same key, other plan: %v", err)
	}

	price := model.NewPrice{PlanID: first, BillingType: model.BillingRecurring, Interval: model.IntervalMonth, Currency: "USD", AmountMinor: 2900}
	p1, err := e.svc.CreatePrice(e.ctx, price)
	if err != nil {
		t.Fatal(err)
	}
	if p2, err := e.svc.CreatePrice(e.ctx, price); err != nil || p2 != p1 {
		t.Fatalf("retried price = %s, %v; want %s", p2, err, p1)
	}
	price.AmountMinor = 3900
	if _, err := e.svc.CreatePrice(e.ctx, price); !errors.Is(err, model.ErrCatalogDuplicate) {
		t.Fatalf("a different monthly price for the same plan: %v", err)
	}
}

// Overrides are written only on the workspace's own grant.
func TestSetGrantOverridesIsScopedToTheWorkspace(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 2900})
	if err := e.svc.GrantPlanManually(e.ctx, model.ManualGrantInput{
		WorkspaceID: e.ws, PlanID: p.ID, Reason: "comp", CreatedBy: e.user,
	}); err != nil {
		t.Fatal(err)
	}
	n, err := repository.NewRepository(e.pool).SetGrantOverrides(e.ctx, gen.SetGrantOverridesParams{
		WorkspaceID: e.workspace(), GrantID: e.grant(e.ws).ID, Overrides: []byte(`{"members": 99}`),
	})
	if err != nil || n != 0 {
		t.Fatalf("another workspace changed the grant: %d rows, %v", n, err)
	}
}

// A product the provider created for a link that then failed to save is
// reused by the retry, so no orphan is left on the provider (Creem keeps
// every product it is asked to make).
func TestFailedProviderLinkReusesItsProduct(t *testing.T) {
	e := newT3Env(t)
	planID, err := e.svc.CreatePlan(e.ctx, model.NewPlan{Code: "orph_" + uuid.NewString()[:8], PlanDetails: details("Orphan")})
	if err != nil {
		t.Fatal(err)
	}
	price, err := e.svc.CreatePrice(e.ctx, model.NewPrice{
		PlanID: planID, Currency: "USD", BillingType: model.BillingRecurring, Interval: model.IntervalMonth, AmountMinor: 2900,
	})
	if err != nil {
		t.Fatal(err)
	}
	// The link's insert fails once, after the provider made the product.
	e.exec(`CREATE FUNCTION fail_link_` + strings.ReplaceAll(price.String(), "-", "") + `() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN IF NEW.plan_price_id = '` + price.String() + `' THEN RAISE EXCEPTION 'link lost'; END IF; RETURN NEW; END $$`)
	trigger := "fail_link_" + strings.ReplaceAll(price.String(), "-", "")
	e.exec(`CREATE TRIGGER ` + trigger + ` BEFORE INSERT ON provider_prices FOR EACH ROW EXECUTE FUNCTION ` + trigger + `()`)
	if _, err := e.svc.CreateProviderPrice(e.ctx, model.NewProviderPrice{PlanPriceID: price, Provider: string(payment.Stripe)}); err == nil {
		t.Fatal("link saved despite the failing insert")
	}
	e.exec(`DROP TRIGGER ` + trigger + ` ON provider_prices`)
	e.exec(`DROP FUNCTION ` + trigger + `()`)

	if _, err := e.svc.CreateProviderPrice(e.ctx, model.NewProviderPrice{PlanPriceID: price, Provider: string(payment.Stripe)}); err != nil {
		t.Fatal(err)
	}
	if n := len(e.pricesFor(price)); n != 1 {
		t.Errorf("the provider made %d products for one link, want 1", n)
	}
}
