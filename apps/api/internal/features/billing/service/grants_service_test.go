package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database/dbtest"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// t3Env is one workspace with a billing profile, a billing service on the
// fake gateway (as Stripe), and a clock the test moves.
type t3Env struct {
	t     *testing.T
	ctx   context.Context
	pool  *pgxpool.Pool
	svc   *Service
	fake  *payment.Fake
	clock time.Time
	ws    uuid.UUID
	user  uuid.UUID
}

var (
	t3FakeOnce sync.Once
	t3Fake     *payment.Fake
)

func newT3Env(t *testing.T) *t3Env {
	t.Helper()
	pool := dbtest.New(t)
	// One fake for the whole run: its IDs ("fake_cs_1", …) must stay unique in the shared database.
	t3FakeOnce.Do(func() { t3Fake = payment.NewFake(payment.Stripe, "whsec_t3") })
	fake := t3Fake
	reg, err := payment.NewRegistry(fake)
	if err != nil {
		t.Fatal(err)
	}
	e := &t3Env{
		t: t, ctx: context.Background(), pool: pool, fake: fake,
		svc:   New(pool, reg, nil, Config{AppBaseURL: "http://app.test"}, slog.New(slog.DiscardHandler)),
		clock: time.Now().UTC().Truncate(time.Second),
	}
	e.svc.now = func() time.Time { return e.clock }
	fake.SetClock(func() time.Time { return e.clock })
	e.ws, e.user = e.workspace(), uuid.New()
	e.exec(`INSERT INTO users (id, email) VALUES ($1, $2)`, e.user, e.user.String()+"@t3.test")
	if _, err := e.svc.UpdateProfile(e.ctx, e.ws, model.ProfileInput{LegalName: "Acme Ltd", Email: "billing@acme.test"}); err != nil {
		t.Fatal(err)
	}
	return e
}

// workspace creates an empty workspace; its cached plan starts as the column default.
func (e *t3Env) workspace() uuid.UUID {
	id := uuid.New()
	e.exec(`INSERT INTO workspaces (id, name) VALUES ($1, 'T3 billing')`, id)
	return id
}

func (e *t3Env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(e.ctx, sql, args...); err != nil {
		e.t.Fatal(err)
	}
}

func (e *t3Env) tick(d time.Duration) { e.clock = e.clock.Add(d) }

// t3Plan is a plan with a monthly and a yearly price, both sold through the fake.
type t3Plan struct {
	ID, Monthly, Yearly uuid.UUID
	Code                string
}

type t3PlanOpts struct {
	Monthly      int64
	Limits       string
	TrialDays    int32
	RequiresCard bool
}

func (e *t3Env) plan(o t3PlanOpts) t3Plan {
	e.t.Helper()
	r := repository.NewRepository(e.pool)
	if o.Limits == "" {
		o.Limits = `{"members": 10, "whatsapp_numbers": 3}`
	}
	code := "t3_" + uuid.NewString()[:8]
	id, err := r.CreatePlan(e.ctx, gen.CreatePlanParams{
		Code: code, Version: 1, Name: "Plan " + code, TrialDays: o.TrialDays, TrialRequiresCard: o.RequiresCard,
		Limits: []byte(o.Limits),
	})
	if err != nil {
		e.t.Fatal(err)
	}
	p := t3Plan{ID: id, Code: code}
	p.Monthly = e.price(id, model.BillingRecurring, model.IntervalMonth, o.Monthly)
	p.Yearly = e.price(id, model.BillingRecurring, model.IntervalYear, o.Monthly*10)
	return p
}

// price adds a price to plan and sells it through the fake (as Stripe).
func (e *t3Env) price(planID uuid.UUID, billingType, interval string, amount int64) uuid.UUID {
	e.t.Helper()
	r := repository.NewRepository(e.pool)
	params := gen.CreatePlanPriceParams{PlanID: planID, BillingType: billingType, Currency: "USD", AmountMinor: amount}
	if interval != "" {
		params.Interval = &interval
	}
	id, err := r.CreatePlanPrice(e.ctx, params)
	if err != nil {
		e.t.Fatal(err)
	}
	ext := "price_" + uuid.NewString()
	if _, err := r.CreateProviderPrice(e.ctx, gen.CreateProviderPriceParams{
		PlanPriceID: id, Provider: string(payment.Stripe), ExternalPriceID: &ext,
	}); err != nil {
		e.t.Fatal(err)
	}
	return id
}

// checkout starts a checkout for priceID with code (may be empty).
func (e *t3Env) checkout(priceID uuid.UUID, code string) model.StartedCheckout {
	e.t.Helper()
	c, err := e.svc.CreateCheckout(e.ctx, e.ws, e.user, model.CheckoutInput{
		PlanPriceID: priceID, DiscountCode: code, IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		e.t.Fatalf("create checkout: %v", err)
	}
	return c
}

// pay completes a started checkout as the provider's webhook would.
func (e *t3Env) pay(c model.StartedCheckout) payment.Event {
	e.t.Helper()
	ev, err := e.fake.CompleteCheckout(*c.Session.ExternalSessionID)
	if err != nil {
		e.t.Fatal(err)
	}
	e.inTx(func(tx pgx.Tx) error { return e.svc.CompleteCheckout(e.ctx, tx, e.ws, c.Session.ID, ev) })
	return ev
}

// subscribe buys priceID and returns the live subscription.
func (e *t3Env) subscribe(priceID uuid.UUID) model.Subscription {
	e.t.Helper()
	e.pay(e.checkout(priceID, ""))
	sub, err := e.svc.Subscription(e.ctx, e.ws)
	if err != nil {
		e.t.Fatal(err)
	}
	return sub
}

func (e *t3Env) inTx(fn func(tx pgx.Tx) error) {
	e.t.Helper()
	if err := database.WithTx(e.ctx, e.pool, fn); err != nil {
		e.t.Fatal(err)
	}
}

// cache reads workspaces.plan and trial_ends_at.
func (e *t3Env) cache(ws uuid.UUID) (string, *time.Time) {
	e.t.Helper()
	var plan string
	var trialEnds *time.Time
	if err := e.pool.QueryRow(e.ctx, `SELECT plan, trial_ends_at FROM workspaces WHERE id = $1`, ws).
		Scan(&plan, &trialEnds); err != nil {
		e.t.Fatal(err)
	}
	return plan, trialEnds
}

// wantCache checks the cached plan matches both want and the current grant (Q8).
func (e *t3Env) wantCache(ws uuid.UUID, want string) {
	e.t.Helper()
	got, _ := e.cache(ws)
	cur, err := e.svc.CurrentPlan(e.ctx, ws)
	if err != nil {
		e.t.Fatal(err)
	}
	if got != want || cur.PlanCode != want {
		e.t.Fatalf("workspaces.plan = %q, current plan = %q; want %q", got, cur.PlanCode, want)
	}
}

func (e *t3Env) grant(ws uuid.UUID) *model.Grant {
	e.t.Helper()
	g, err := currentGrant(e.ctx, repository.NewRepository(e.pool), ws)
	if err != nil {
		e.t.Fatal(err)
	}
	return g
}

func TestGrantChangesKeepThePlanCache(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000, Limits: `{"members": 12}`})
	reason := "support gift"

	var g model.Grant
	e.inTx(func(tx pgx.Tx) error {
		var err error
		g, err = e.svc.ActivateGrant(e.ctx, tx, e.ws, model.GrantInput{PlanID: p.ID, Source: model.GrantManual, Reason: &reason})
		return err
	})
	e.wantCache(e.ws, p.Code)
	if n, _ := e.svc.MemberLimit(e.ctx, e.ws); n != 12 {
		t.Errorf("member limit = %d, want 12", n)
	}
	if n, _ := e.svc.NumberLimit(e.ctx, e.ws); n != model.Unlimited {
		t.Errorf("number limit = %d, want unlimited", n)
	}

	e.inTx(func(tx pgx.Tx) error { return e.svc.SuspendGrant(e.ctx, tx, e.ws, g.ID) })
	e.wantCache(e.ws, model.FreePlan)
	if n, _ := e.svc.MemberLimit(e.ctx, e.ws); n != model.FreeLimits.Members {
		t.Errorf("suspended member limit = %d, want free %d", n, model.FreeLimits.Members)
	}

	e.inTx(func(tx pgx.Tx) error { return e.svc.RestoreGrant(e.ctx, tx, e.ws, g.ID) })
	e.wantCache(e.ws, p.Code)

	// A new grant replaces the current one.
	other := e.plan(t3PlanOpts{Monthly: 2000})
	e.inTx(func(tx pgx.Tx) error {
		_, err := e.svc.ActivateGrant(e.ctx, tx, e.ws, model.GrantInput{PlanID: other.ID, Source: model.GrantManual, Reason: &reason})
		return err
	})
	e.wantCache(e.ws, other.Code)

	cur := e.grant(e.ws)
	e.inTx(func(tx pgx.Tx) error { return e.svc.EndGrant(e.ctx, tx, e.ws, cur.ID) })
	e.wantCache(e.ws, model.FreePlan)
	if e.grant(e.ws) != nil {
		t.Fatal("grant still current after EndGrant")
	}
}

func TestRolledBackGrantChangeLeavesPlanCache(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	reason := "test"
	boom := errors.New("boom")
	before, _ := e.cache(e.ws)

	err := database.WithTx(e.ctx, e.pool, func(tx pgx.Tx) error {
		if _, err := e.svc.ActivateGrant(e.ctx, tx, e.ws, model.GrantInput{PlanID: p.ID, Source: model.GrantManual, Reason: &reason}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("tx = %v", err)
	}
	if got, _ := e.cache(e.ws); got != before || e.grant(e.ws) != nil {
		t.Fatalf("after rollback: plan %q (was %q), grant %v", got, before, e.grant(e.ws))
	}

	// Commit a grant, then roll back a suspension of it.
	e.inTx(func(tx pgx.Tx) error {
		_, err := e.svc.ActivateGrant(e.ctx, tx, e.ws, model.GrantInput{PlanID: p.ID, Source: model.GrantManual, Reason: &reason})
		return err
	})
	g := e.grant(e.ws)
	_ = database.WithTx(e.ctx, e.pool, func(tx pgx.Tx) error {
		if err := e.svc.SuspendGrant(e.ctx, tx, e.ws, g.ID); err != nil {
			return err
		}
		return boom
	})
	e.wantCache(e.ws, p.Code)
	if e.grant(e.ws).Status != model.GrantActive {
		t.Fatal("suspension survived its rollback")
	}
}

func TestCardlessTrialStartsAndExpires(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000, TrialDays: 1})

	var g *model.Grant
	e.inTx(func(tx pgx.Tx) error {
		var err error
		g, err = e.svc.StartTrial(e.ctx, tx, e.ws, p.Code)
		return err
	})
	if g == nil || g.Source != model.GrantFromTrial {
		t.Fatalf("trial grant = %+v", g)
	}
	e.wantCache(e.ws, p.Code)
	if _, ends := e.cache(e.ws); ends == nil || !ends.Equal(e.clock.AddDate(0, 0, 1)) {
		t.Fatalf("trial_ends_at = %v, want %v", ends, e.clock.AddDate(0, 0, 1))
	}

	e.tick(24*time.Hour + time.Second)
	n, err := e.svc.ExpireTrials(e.ctx)
	if err != nil || n < 1 {
		t.Fatalf("ExpireTrials = %d, %v", n, err)
	}
	e.wantCache(e.ws, model.FreePlan)
	if _, ends := e.cache(e.ws); ends != nil {
		t.Fatalf("trial_ends_at = %v after expiry", ends)
	}
}

func TestStartTrialSkipsPlansWithoutCardlessTrial(t *testing.T) {
	e := newT3Env(t)
	card := e.plan(t3PlanOpts{Monthly: 1000, TrialDays: 7, RequiresCard: true})
	none := e.plan(t3PlanOpts{Monthly: 1000})
	for _, code := range []string{card.Code, none.Code} {
		e.inTx(func(tx pgx.Tx) error {
			g, err := e.svc.StartTrial(e.ctx, tx, e.ws, code)
			if g != nil {
				t.Errorf("%s: got a trial grant", code)
			}
			return err
		})
	}
	err := database.WithTx(e.ctx, e.pool, func(tx pgx.Tx) error {
		_, err := e.svc.StartTrial(e.ctx, tx, e.ws, "t3_no_such_plan")
		return err
	})
	if !errors.Is(err, model.ErrPlanNotFound) {
		t.Fatalf("unknown plan: %v", err)
	}
}

func TestActivateGrantChecksItsSource(t *testing.T) {
	e := newT3Env(t)
	p := e.plan(t3PlanOpts{Monthly: 1000})
	for name, in := range map[string]model.GrantInput{
		"manual without reason": {PlanID: p.ID, Source: model.GrantManual},
		"trial without end":     {PlanID: p.ID, Source: model.GrantFromTrial},
		"order without order":   {PlanID: p.ID, Source: model.GrantFromOrder},
	} {
		err := database.WithTx(e.ctx, e.pool, func(tx pgx.Tx) error {
			_, err := e.svc.ActivateGrant(e.ctx, tx, e.ws, in)
			return err
		})
		if !t3IsInvalidInput(err) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestDeletionCheck(t *testing.T) {
	e := newT3Env(t)
	if d, err := e.svc.DeletionCheck(e.ctx, e.ws); err != nil || !d.Allowed {
		t.Fatalf("free workspace: %+v, %v", d, err)
	}
	p := e.plan(t3PlanOpts{Monthly: 1000})
	e.subscribe(p.Monthly)
	if d, err := e.svc.DeletionCheck(e.ctx, e.ws); err != nil || d.Allowed || d.Reason != model.DeleteBlockedSubscription {
		t.Fatalf("subscribed workspace: %+v, %v", d, err)
	}
}
