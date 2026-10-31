package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database/dbtest"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

func TestDefaultsAreValid(t *testing.T) {
	for _, k := range model.Kinds {
		if d := model.Default(k).Validate(); len(d) > 0 {
			t.Errorf("%s defaults: %v", k, d)
		}
	}
}

func TestSettingsValidation(t *testing.T) {
	badAway := model.Default(model.KindAway).(*model.AwaySettings)
	badAway.Hours[0].From, badAway.Hours[0].To = "20:00", "10:00"
	badZone := model.Default(model.KindAway).(*model.AwaySettings)
	badZone.Timezone = "Mars/Base"
	missingDay := model.Default(model.KindAway).(*model.AwaySettings)
	missingDay.Hours = missingDay.Hours[1:]
	drip := model.Default(model.KindDrip).(*model.DripSettings)
	drip.Steps[1].Day = 0

	cases := []struct {
		name  string
		in    model.Settings
		field string
	}{
		{"welcome empty", &model.WelcomeSettings{Message: "  "}, "message"},
		{"welcome delay", &model.WelcomeSettings{Message: "hi", DelayMinutes: 2}, "delay_minutes"},
		{"away closes before it opens", badAway, "hours[0]"},
		{"away timezone", badZone, "timezone"},
		{"away missing day", missingDay, "hours"},
		{"keyword no reply", &model.KeywordSettings{Rules: []model.KeywordRule{{Keywords: "price", Match: model.MatchAny}}}, "rules[0].reply"},
		{"keyword no words", &model.KeywordSettings{Rules: []model.KeywordRule{{Match: model.MatchAny, Reply: "x"}}}, "rules[0].keywords"},
		{"keyword match", &model.KeywordSettings{Rules: []model.KeywordRule{{Keywords: "a", Match: "fuzzy", Reply: "x"}}}, "rules[0].match"},
		{"cart delay", &model.CartSettings{DelayMinutes: 5}, "delay_minutes"},
		{"cart variable", &model.CartSettings{DelayMinutes: 60, Variables: []model.Variable{{Source: model.SourceOrderNumber}}}, "variables[0]"},
		{"order trigger", &model.OrderSettings{Trigger: "shopify_refund"}, "trigger"},
		{"order custom text", &model.OrderSettings{Trigger: model.TriggerShopifyOrderPaid, Variables: []model.Variable{{Source: model.SourceCustom}}}, "variables[0]"},
		{"drip out of order", drip, "steps[1].day"},
		{"drip no steps", &model.DripSettings{}, "steps"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := c.in.Validate()
			for _, got := range d {
				if got.Field == c.field {
					return
				}
			}
			t.Fatalf("want a problem on %q, got %v", c.field, d)
		})
	}
}

type env struct {
	svc *Service
	ws  uuid.UUID
	usr uuid.UUID
}

func newEnv(t *testing.T) env {
	t.Helper()
	pool := dbtest.New(t)
	usr, ws := seed(t, pool)
	return env{svc: New(pool), ws: ws, usr: usr}
}

func seed(t *testing.T, pool *pgxpool.Pool) (user, workspace uuid.UUID) {
	t.Helper()
	user, workspace = uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email) VALUES ($1, $2)`, user, user.String()+"@relay.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, name) VALUES ($1, 'Automations')`, workspace); err != nil {
		t.Fatal(err)
	}
	return user, workspace
}

func TestListFillsDefaultsInPageOrder(t *testing.T) {
	e := newEnv(t)
	items, err := e.svc.List(t.Context(), e.ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != len(model.Kinds) {
		t.Fatalf("got %d kinds", len(items))
	}
	for i, a := range items {
		if a.Kind != model.Kinds[i] || a.Enabled || a.UpdatedAt != nil {
			t.Errorf("item %d = %+v", i, a)
		}
	}
}

func TestSaveRoundTripsAndKeepsEnabled(t *testing.T) {
	e := newEnv(t)
	if _, err := e.svc.SetEnabled(t.Context(), e.ws, e.usr, model.KindWelcome, true); err != nil {
		t.Fatal(err)
	}
	want := &model.WelcomeSettings{Message: "Hello", DelayMinutes: 5}
	if _, err := e.svc.Save(t.Context(), e.ws, e.usr, model.KindWelcome, want); err != nil {
		t.Fatal(err)
	}
	items, err := e.svc.List(t.Context(), e.ws)
	if err != nil {
		t.Fatal(err)
	}
	got := items[0]
	if s := got.Settings.(*model.WelcomeSettings); !got.Enabled || *s != *want {
		t.Fatalf("got %+v, settings %+v", got, s)
	}
}

func TestSaveRejectsInvalidSettings(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Save(t.Context(), e.ws, e.usr, model.KindWelcome, &model.WelcomeSettings{})
	if !apperr.IsKind(err, apperr.KindValidation) {
		t.Fatalf("err = %v", err)
	}
}

func TestEnableNeedsReadySettings(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.SetEnabled(t.Context(), e.ws, e.usr, model.KindKeyword, true)
	if !apperr.IsKind(err, apperr.KindConflict) {
		t.Fatalf("enable with no rules: err = %v", err)
	}
	// Turning off never needs complete settings.
	if _, err := e.svc.SetEnabled(t.Context(), e.ws, e.usr, model.KindKeyword, false); err != nil {
		t.Fatal(err)
	}
	rules := &model.KeywordSettings{Rules: []model.KeywordRule{{Keywords: "price", Match: model.MatchAny, Reply: "See the catalog"}}}
	if _, err := e.svc.Save(t.Context(), e.ws, e.usr, model.KindKeyword, rules); err != nil {
		t.Fatal(err)
	}
	a, err := e.svc.SetEnabled(t.Context(), e.ws, e.usr, model.KindKeyword, true)
	if err != nil || !a.Enabled {
		t.Fatalf("enable: %+v, %v", a, err)
	}
	again, err := e.svc.SetEnabled(t.Context(), e.ws, e.usr, model.KindKeyword, true)
	if err != nil || !again.Enabled {
		t.Fatalf("enable twice: %+v, %v", again, err)
	}
}
