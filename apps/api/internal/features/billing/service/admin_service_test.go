package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
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
