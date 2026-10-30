package service

import (
	"errors"
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

func TestPaymentMethodBeforeAnythingIsBought(t *testing.T) {
	e := newT3Env(t)
	pm, err := e.svc.PaymentMethod(e.ctx, e.ws)
	if err != nil || pm.Card != nil || pm.Provider != "" || pm.CanManage {
		t.Fatalf("no customer: %+v, %v", pm, err)
	}
	if _, err := e.svc.PortalURL(e.ctx, e.ws); !errors.Is(err, model.ErrNoCardCustomer) {
		t.Fatalf("portal with no customer: %v", err)
	}
}

func TestPaymentMethodReadsTheSubscribersCard(t *testing.T) {
	e := newT3Env(t)
	e.subscribe(e.plan(t3PlanOpts{Monthly: 2900}).Monthly)

	pm, err := e.svc.PaymentMethod(e.ctx, e.ws)
	if err != nil {
		t.Fatal(err)
	}
	want := model.Card{
		Brand: payment.FakeCard.Brand, Last4: payment.FakeCard.Last4,
		ExpMonth: payment.FakeCard.ExpMonth, ExpYear: payment.FakeCard.ExpYear,
	}
	if pm.Provider != string(payment.Stripe) || !pm.CanManage || !pm.CardDetails || pm.Card == nil || *pm.Card != want {
		t.Fatalf("payment method = %+v", pm)
	}

	url, err := e.svc.PortalURL(e.ctx, e.ws)
	if err != nil || url != "http://app.test/billing?from=portal" {
		t.Fatalf("portal = %q, %v", url, err)
	}
}

func TestPaymentMethodSurvivesAProviderOutage(t *testing.T) {
	e := newT3Env(t)
	e.subscribe(e.plan(t3PlanOpts{Monthly: 2900}).Monthly)

	e.fake.FailNext(payment.ErrUnavailable)
	pm, err := e.svc.PaymentMethod(e.ctx, e.ws)
	if err != nil || pm.Card != nil || !pm.CanManage {
		t.Fatalf("outage: %+v, %v", pm, err)
	}
	e.fake.FailNext(payment.ErrUnsupported)
	if pm, err := e.svc.PaymentMethod(e.ctx, e.ws); err != nil || pm.Card != nil || pm.CardDetails {
		t.Fatalf("no card details: %+v, %v", pm, err)
	}
	e.fake.FailNext(payment.ErrUnavailable)
	if _, err := e.svc.PortalURL(e.ctx, e.ws); !errors.Is(err, model.ErrPaymentProviderDown) {
		t.Fatalf("portal outage: %v", err)
	}
}
