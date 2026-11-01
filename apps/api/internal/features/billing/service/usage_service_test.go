package service

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

// onPlan puts the test workspace on a plan with these limits.
func (e *t3Env) onPlan(limits string) {
	e.t.Helper()
	p := e.plan(t3PlanOpts{Monthly: 1000, Limits: limits})
	reason := "usage test"
	e.inTx(func(tx pgx.Tx) error {
		_, err := e.svc.ActivateGrant(e.ctx, tx, e.ws, model.GrantInput{PlanID: p.ID, Source: model.GrantManual, Reason: &reason})
		return err
	})
}

func (e *t3Env) consume(n int) bool {
	e.t.Helper()
	_, ok, err := e.svc.Consume(e.ctx, e.ws, "broadcasts", n)
	if err != nil {
		e.t.Fatal(err)
	}
	return ok
}

// used is this month's broadcasts count.
func (e *t3Env) used() int {
	e.t.Helper()
	u, err := e.svc.Usage(e.ctx, e.ws)
	if err != nil {
		e.t.Fatal(err)
	}
	for _, r := range u.Resources {
		if r.Resource == "broadcasts" {
			return r.Used
		}
	}
	e.t.Fatal("broadcasts is not in the usage")
	return 0
}

func TestConsumeStopsAtTheCap(t *testing.T) {
	e := newT3Env(t)
	e.onPlan(`{"members": 3, "whatsapp_numbers": 1, "broadcasts": 2}`)

	for range 2 {
		if !e.consume(1) {
			t.Fatal("consume under the cap refused")
		}
	}
	if e.consume(1) {
		t.Error("third broadcast allowed past a cap of 2")
	}
	if got := e.used(); got != 2 {
		t.Errorf("used = %d, want 2", got)
	}
	if e.consume(5) {
		t.Error("consuming more than the whole cap at once allowed")
	}
}

func TestConsumeFirstUseLargerThanTheCap(t *testing.T) {
	e := newT3Env(t)
	e.onPlan(`{"members": 3, "whatsapp_numbers": 1, "broadcasts": 2}`)

	if e.consume(3) {
		t.Error("first consume of 3 allowed past a cap of 2")
	}
	if got := e.used(); got != 0 {
		t.Errorf("used = %d, want 0", got)
	}
}

func TestConsumeUnlimitedAndNotIncluded(t *testing.T) {
	e := newT3Env(t)
	e.onPlan(`{"members": 3, "whatsapp_numbers": 1, "broadcasts": -1}`)
	for range 5 {
		if !e.consume(10) {
			t.Fatal("unlimited plan refused a broadcast")
		}
	}
	if got := e.used(); got != 50 {
		t.Errorf("used = %d, want 50 (unlimited still counts)", got)
	}

	e2 := newT3Env(t)
	e2.onPlan(`{"members": 3, "whatsapp_numbers": 1}`)
	if e2.consume(1) {
		t.Error("a plan without broadcasts allowed one")
	}
}

func TestConsumeIsAtomicUnderConcurrency(t *testing.T) {
	e := newT3Env(t)
	e.onPlan(`{"members": 3, "whatsapp_numbers": 1, "broadcasts": 5}`)

	var wg sync.WaitGroup
	var allowed atomic.Int32
	for range 20 {
		wg.Go(func() {
			if _, ok, err := e.svc.Consume(e.ctx, e.ws, "broadcasts", 1); err != nil {
				t.Error(err)
			} else if ok {
				allowed.Add(1)
			}
		})
	}
	wg.Wait()
	if allowed.Load() != 5 {
		t.Errorf("%d of 20 concurrent consumes allowed, want exactly 5", allowed.Load())
	}
	if got := e.used(); got != 5 {
		t.Errorf("used = %d, want 5", got)
	}
}

func TestReleaseGivesBackToTheMonthConsumed(t *testing.T) {
	e := newT3Env(t)
	e.onPlan(`{"members": 3, "whatsapp_numbers": 1, "broadcasts": 1}`)
	e.clock = time.Date(2026, 10, 31, 23, 59, 0, 0, time.UTC)

	period, ok, err := e.svc.Consume(e.ctx, e.ws, "broadcasts", 1)
	if err != nil || !ok {
		t.Fatalf("consume: ok=%v err=%v", ok, err)
	}
	if e.consume(1) {
		t.Fatal("second broadcast allowed past a cap of 1")
	}

	// A new month counts from 0, and a release afterwards still lands on October.
	e.tick(2 * time.Minute)
	if !e.consume(1) {
		t.Fatal("November refused its first broadcast")
	}
	if err := e.svc.Release(e.ctx, e.ws, "broadcasts", period, 1); err != nil {
		t.Fatal(err)
	}
	if got := e.used(); got != 1 {
		t.Errorf("November used = %d, want 1", got)
	}
	e.clock = period
	if got := e.used(); got != 0 {
		t.Errorf("October used = %d after release, want 0", got)
	}
	// Releasing more than was used never goes below 0.
	if err := e.svc.Release(e.ctx, e.ws, "broadcasts", period, 3); err != nil {
		t.Fatal(err)
	}
	if got := e.used(); got != 0 {
		t.Errorf("October used = %d, want 0", got)
	}
}

func TestUsageListsEveryMonthlyResource(t *testing.T) {
	e := newT3Env(t)
	e.onPlan(`{"members": 3, "whatsapp_numbers": 1, "broadcasts": 30, "api_calls": -1}`)
	e.clock = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	e.consume(2)

	u, err := e.svc.Usage(e.ctx, e.ws)
	if err != nil {
		t.Fatal(err)
	}
	if !u.PeriodStart.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) || !u.PeriodEnd.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("period = %v – %v, want October 2026", u.PeriodStart, u.PeriodEnd)
	}
	want := []model.MonthlyUsage{{Resource: "broadcasts", Used: 2, Limit: 30}, {Resource: "api_calls", Used: 0, Limit: model.Unlimited}}
	if len(u.Resources) != len(want) {
		t.Fatalf("resources = %+v, want %+v", u.Resources, want)
	}
	for i := range want {
		if u.Resources[i] != want[i] {
			t.Errorf("resource %d = %+v, want %+v", i, u.Resources[i], want[i])
		}
	}
}

func TestConsumeRefusesKeysThatArentMonthly(t *testing.T) {
	e := newT3Env(t)
	if _, _, err := e.svc.Consume(e.ctx, e.ws, "members", 1); err == nil {
		t.Error("consume of a count resource didn't fail")
	}
	if _, _, err := e.svc.Consume(e.ctx, e.ws, "nope", 1); err == nil {
		t.Error("consume of an unknown key didn't fail")
	}
	if _, _, err := e.svc.Consume(e.ctx, e.ws, "broadcasts", 0); err == nil {
		t.Error("consume of 0 didn't fail")
	}
}
