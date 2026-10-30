package service

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

func TestAdminBuildsASellableCatalog(t *testing.T) {
	e := newT3Env(t)
	code := "adm_" + uuid.NewString()[:8]
	planID, err := e.svc.CreatePlan(e.ctx, model.NewPlan{
		Code: code, Name: "Admin plan", TrialDays: 14, Limits: model.Limits{Members: 10, WhatsAppNumbers: model.Unlimited},
	})
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
	v2, err := e.svc.CreatePlan(e.ctx, model.NewPlan{Code: code, Name: "Admin plan v2"})
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
	if _, err := e.svc.CreatePlan(e.ctx, model.NewPlan{Code: "free", Name: "Free"}); !isInvalidInput(err) {
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
