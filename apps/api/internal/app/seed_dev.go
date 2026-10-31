package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	authservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/service"
	automationsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
	workspacesmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/workspaces/model"
)

// `relay dev seed` fills a local database for trying every feature: staff who can
// sign in to the CRM, the plan catalog and a few tenants. Running it again adds
// nothing new. A new feature adds its sample data here (CLAUDE.md, section 3).

// SeedPassword signs in every seeded account. It only ever exists on a local
// database: Seed refuses to run outside development.
const SeedPassword = "Relay-dev-2026!" //nolint:gosec // A known local-only sign-in, not a secret.

const seedStaffEmail = "admin@relay.test"

var seedNamespace = uuid.MustParse("c7a1e0de-5d6b-4c8e-9a43-2f1b7d3e6a11")

func seedID(parts ...string) uuid.UUID {
	return uuid.NewSHA1(seedNamespace, []byte(strings.Join(parts, "/")))
}

type seedPrice struct {
	interval string
	amount   int64
}

type seedPlan struct {
	code, name, description string
	sort, trialDays         int
	recommended             bool
	limits                  map[string]int
	prices                  []seedPrice
}

var seedPlans = []seedPlan{
	{
		code: "starter", name: "Starter", description: "For small shops getting started on WhatsApp.", sort: 10,
		limits: map[string]int{"members": 3, "whatsapp_numbers": 1, "contacts": 2000, "broadcasts": 5, "api_calls": 10000, "automations": 1},
		prices: []seedPrice{{"month", 2900}, {"year", 29000}},
	},
	{
		code: "growth", name: "Growth", description: "Automations and store connections for growing teams.", sort: 20, trialDays: 14, recommended: true,
		limits: map[string]int{
			"members": 10, "whatsapp_numbers": 3, "contacts": 20000, "broadcasts": 50, "api_calls": 100000,
			"automations": 1, "ecommerce": 1, "team_controls": 1,
		},
		prices: []seedPrice{{"month", 7900}, {"year", 79000}},
	},
	{
		code: "scale", name: "Scale", description: "Unlimited contacts and priority support.", sort: 30,
		limits: map[string]int{
			"members": -1, "whatsapp_numbers": 10, "contacts": -1, "broadcasts": -1, "api_calls": -1,
			"automations": 1, "ai_replies": 1, "ecommerce": 1, "team_controls": 1, "priority_support": 1,
		},
		prices: []seedPrice{{"month", 19900}, {"year", 199000}},
	},
}

type seedPerson struct{ email, first, last string }

type seedContact struct {
	name, phone, optIn string
	tags               []string
}

type seedTenant struct {
	name         string
	owner, agent seedPerson
	// plan is the catalog plan the workspace is on. Empty keeps the new workspace's trial.
	plan     string
	store    bool
	contacts []seedContact
}

var seedTenants = []seedTenant{
	{
		name: "Green Leaf Store", plan: "growth", store: true,
		owner: seedPerson{"owner@greenleaf.test", "Nadia", "Karim"}, agent: seedPerson{"agent@greenleaf.test", "Rafi", "Ahmed"},
		contacts: []seedContact{
			{"Ayesha Rahman", "+8801711000001", "in", []string{"vip", "new-customer"}},
			{"Tanvir Hasan", "+8801711000002", "in", []string{"new-customer"}},
			{"Sumi Das", "+8801711000003", "out", nil},
			{"Lina Roy", "+8801711000004", "pending", []string{"wholesale"}},
		},
	},
	{
		name: "Bloom Cafe", plan: "starter",
		owner: seedPerson{"owner@bloomcafe.test", "Maya", "Sen"}, agent: seedPerson{"agent@bloomcafe.test", "Joy", "Paul"},
		contacts: []seedContact{
			{"Hari Sen", "+8801722000001", "in", []string{"regular"}},
			{"Mim Akter", "+8801722000002", "pending", nil},
		},
	},
	{
		name:  "Nova Clinic",
		owner: seedPerson{"owner@novaclinic.test", "Omar", "Faruk"},
		contacts: []seedContact{
			{"Dina Roy", "+8801733000001", "in", []string{"patient"}},
		},
	},
}

type seedTemplate struct {
	name, category, text string
	examples             []string
}

var seedTemplates = []seedTemplate{
	{"order_confirmation", "utility", "Hi {{1}}, your order {{2}} for {{3}} is confirmed. Track it here: {{4}}", []string{"Sam", "#1001", "86.50 USD", "https://example.test/1001"}},
	{"cart_reminder", "marketing", "Hi {{1}}, you left items worth {{2}} in your cart. Finish here: {{3}}", []string{"Sam", "86.50 USD", "https://example.test/cart"}},
	{"welcome_series_1", "marketing", "Welcome {{1}}, here is our story.", []string{"Sam"}},
	{"welcome_series_2", "marketing", "Hi {{1}}, here are this week's popular picks.", []string{"Sam"}},
}

// seeder runs one seed in order; the first failure stops the rest.
type seeder struct {
	a    *App
	ctx  context.Context //nolint:containedctx // Lives for one command.
	hash string
	err  error
}

func (s *seeder) exec(sql string, args ...any) {
	if s.err != nil {
		return
	}
	if _, err := s.a.db.Exec(s.ctx, sql, args...); err != nil {
		s.err = fmt.Errorf("seed: %w", err)
	}
}

// Seed fills the database and writes the sign-ins to out.
func (a *App) Seed(ctx context.Context, out io.Writer) error {
	if !a.cfg.IsDevelopment() {
		return errors.New("seed runs only in development")
	}
	hash, err := authservice.HashPassword(SeedPassword)
	if err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	s := &seeder{a: a, ctx: ctx, hash: hash}
	s.plans()
	staff := seedStaffEmail
	if len(a.cfg.Admin.Emails) > 0 {
		staff = strings.TrimSpace(a.cfg.Admin.Emails[0])
	}
	s.user(seedPerson{staff, "Relay", "Admin"})
	for _, t := range seedTenants {
		if err := s.tenant(t); err != nil {
			return err
		}
	}
	if s.err != nil {
		return s.err
	}
	return seedReport(out, staff, slices.Contains(a.cfg.Admin.Emails, staff))
}

func seedReport(out io.Writer, staff string, allowed bool) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Seeded. Every account signs in with password %s\n\nCRM staff: %s\n", SeedPassword, staff)
	if !allowed {
		fmt.Fprintf(&b, "  Add ADMIN_EMAILS=%s and restart the API to open the CRM.\n", staff)
	}
	for _, t := range seedTenants {
		plan := "trial"
		if t.plan != "" {
			plan = t.plan
		}
		fmt.Fprintf(&b, "\n%s (%s)\n  Owner: %s\n", t.name, plan, t.owner.email)
		if t.agent.email != "" {
			fmt.Fprintf(&b, "  Agent: %s\n", t.agent.email)
		}
	}
	b.WriteString("\nSet WHATSAPP_FAKE=true so sends get delivered ticks.\n")
	if _, err := io.WriteString(out, b.String()); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

func (s *seeder) plans() {
	for _, p := range seedPlans {
		limits, err := json.Marshal(p.limits)
		if err != nil {
			s.err = fmt.Errorf("seed plan: %w", err)
			return
		}
		s.exec(`INSERT INTO plans (code, name, description, sort_order, trial_days, recommended, limits)
			VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (code, version) DO NOTHING`,
			p.code, p.name, p.description, p.sort, p.trialDays, p.recommended, limits)
		for _, price := range p.prices {
			s.exec(`INSERT INTO plan_prices (plan_id, billing_type, interval, currency, amount_minor)
				SELECT id, 'recurring', $2, 'USD', $3 FROM plans WHERE code = $1 AND version = 1
				ON CONFLICT DO NOTHING`, p.code, price.interval, price.amount)
		}
	}
}

// user makes a verified account, or finds it.
func (s *seeder) user(p seedPerson) uuid.UUID {
	id := seedID("user", strings.ToLower(p.email))
	s.exec(`INSERT INTO users (id, email, password_hash, email_verified_at) VALUES ($1, $2, $3, now())
		ON CONFLICT DO NOTHING`, id, p.email, s.hash)
	if s.err != nil {
		return uuid.Nil
	}
	if err := s.a.db.QueryRow(s.ctx, `SELECT id FROM users WHERE email = $1`, p.email).Scan(&id); err != nil {
		s.err = fmt.Errorf("seed user: %w", err)
		return uuid.Nil
	}
	s.exec(`INSERT INTO user_profiles (user_id, first_name, last_name) VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, id, p.first, p.last)
	s.exec(`INSERT INTO user_preferences (user_id, timezone) VALUES ($1, 'Asia/Dhaka') ON CONFLICT DO NOTHING`, id)
	return id
}

// workspace creates the tenant through the service, so it gets what a signup gets.
func (s *seeder) workspace(t seedTenant, owner uuid.UUID) uuid.UUID {
	if s.err != nil {
		return uuid.Nil
	}
	var id uuid.UUID
	err := s.a.db.QueryRow(s.ctx, `SELECT id FROM workspaces WHERE created_by = $1 AND name = $2`, owner, t.name).Scan(&id)
	if err == nil {
		return id
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		s.err = fmt.Errorf("seed workspace: %w", err)
		return uuid.Nil
	}
	w, err := s.a.workspaces.Create(s.ctx, owner, workspacesmodel.CreateInput{
		Name: t.name, FirstGoal: "automate", StartMode: workspacesmodel.StartSetup,
	})
	if err != nil {
		s.err = fmt.Errorf("seed workspace: %w", err)
		return uuid.Nil
	}
	return w.ID
}

func (s *seeder) tenant(t seedTenant) error {
	owner := s.user(t.owner)
	ws := s.workspace(t, owner)
	s.exec(`UPDATE workspace_onboarding SET completed_at = COALESCE(completed_at, now()) WHERE workspace_id = $1`, ws)
	s.plan(ws, owner, t.plan)
	if t.agent.email != "" {
		agent := s.user(t.agent)
		s.exec(`INSERT INTO workspace_members (workspace_id, user_id, role_id)
			SELECT $1, $2, id FROM roles WHERE workspace_id = $1 AND name = 'Agent'
			ON CONFLICT DO NOTHING`, ws, agent)
	}
	templates := s.whatsapp(ws)
	if t.store {
		s.exec(`INSERT INTO integration_connections (id, workspace_id, provider, status, account_label, config)
			VALUES ($1, $2, 'shopify', 'connected', $3, $4) ON CONFLICT DO NOTHING`, seedID("store", ws.String()), ws,
			ws.String()[:8]+".myshopify.com", `{"shop":"`+ws.String()[:8]+`.myshopify.com","oauth":true}`)
	}
	tags := s.contacts(ws, t.contacts)
	s.exec(`INSERT INTO quick_replies (id, workspace_id, shortcut, title, body) VALUES ($1, $2, 'hours', 'Opening hours', 'We are open Saturday to Thursday, 10 AM to 8 PM.')
		ON CONFLICT DO NOTHING`, seedID("reply", ws.String()), ws)
	if s.err != nil {
		return s.err
	}
	if _, err := s.a.SeedInbox(s.ctx, ws); err != nil {
		return fmt.Errorf("seed inbox: %w", err)
	}
	// The sample chats are history, not customers waiting for a first reply.
	s.exec(`DELETE FROM river_job WHERE kind = 'automations_inbound' AND state = 'available' AND args::text LIKE '%' || $1 || '%'`, ws.String())
	s.automations(ws, owner, t, templates, tags)
	return s.err
}

// plan moves the workspace to a catalog plan by a manual grant, as staff would.
func (s *seeder) plan(ws, owner uuid.UUID, code string) {
	if s.err != nil || code == "" {
		return
	}
	s.exec(`UPDATE plan_grants g SET status = 'ended', ends_at = now()
		WHERE g.workspace_id = $1 AND g.status = 'active' AND g.source <> 'manual'`, ws)
	s.exec(`INSERT INTO plan_grants (workspace_id, plan_id, source, status, starts_at, created_by, reason)
		SELECT $1, id, 'manual', 'active', now(), $2, 'Dev seed' FROM plans WHERE code = $3 AND version = 1
		ON CONFLICT DO NOTHING`, ws, owner, code)
	s.exec(`UPDATE workspaces SET plan = $2, trial_ends_at = NULL, activated_at = COALESCE(activated_at, now()) WHERE id = $1`, ws, code)
}

// whatsapp connects the workspace's fake number and approves its templates.
func (s *seeder) whatsapp(ws uuid.UUID) map[string]uuid.UUID {
	account := seedID("waba", ws.String())
	s.exec(`INSERT INTO whatsapp_accounts (id, workspace_id, waba_id, name, access_token)
		VALUES ($1, $2, $3, 'Sample account', '\x00') ON CONFLICT DO NOTHING`, account, ws, "seed-"+ws.String())
	s.exec(`INSERT INTO whatsapp_numbers (id, workspace_id, account_id, phone_number_id, display_phone_number, number_type, verified_name, name_status, status)
		VALUES ($1, $2, $3, $4, '+1 555 010 0100', 'new', 'Sample number', 'approved', 'connected') ON CONFLICT DO NOTHING`,
		devChannelID(ws), ws, account, "seed-"+ws.String())
	ids := make(map[string]uuid.UUID, len(seedTemplates))
	for _, t := range seedTemplates {
		ids[t.name] = seedID("template", ws.String(), t.name)
		parts, err := json.Marshal(map[string]any{
			"header": map[string]string{"type": "none"}, "body": map[string]any{"text": t.text, "examples": t.examples},
			"footer": map[string]string{"text": ""}, "buttons": []any{}, "auth": map[string]any{},
		})
		if err != nil {
			s.err = fmt.Errorf("seed template: %w", err)
			return nil
		}
		s.exec(`INSERT INTO message_templates (id, workspace_id, account_id, name, language, category, status, meta_template_id, components)
			VALUES ($1, $2, $3, $4, 'en', $5, 'approved', $6, $7) ON CONFLICT DO NOTHING`,
			ids[t.name], ws, account, t.name, t.category, "seed-"+t.name, parts)
	}
	return ids
}

func (s *seeder) contacts(ws uuid.UUID, contacts []seedContact) map[string]uuid.UUID {
	tags := map[string]uuid.UUID{}
	for _, c := range contacts {
		id := seedID("contact", ws.String(), c.phone)
		s.exec(`INSERT INTO contacts (id, workspace_id, name, phone, source, opt_in, opt_in_source, opt_in_at)
			VALUES ($1, $2, $3, $4, 'manual', $5, CASE WHEN $5 = 'pending' THEN NULL ELSE 'manual' END, CASE WHEN $5 = 'pending' THEN NULL ELSE now() END)
			ON CONFLICT DO NOTHING`, id, ws, c.name, c.phone, c.optIn)
		for _, name := range c.tags {
			tag := seedID("tag", ws.String(), name)
			tags[name] = tag
			s.exec(`INSERT INTO contact_tags (id, workspace_id, name) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, tag, ws, name)
			s.exec(`INSERT INTO contact_tag_links (workspace_id, contact_id, tag_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, ws, id, tag)
		}
	}
	return tags
}

func (s *seeder) automations(ws, owner uuid.UUID, t seedTenant, templates, tags map[string]uuid.UUID) {
	if s.err != nil {
		return
	}
	on := map[automationsmodel.Kind]automationsmodel.Settings{
		automationsmodel.KindWelcome: automationsmodel.Default(automationsmodel.KindWelcome),
		automationsmodel.KindKeyword: &automationsmodel.KeywordSettings{Rules: []automationsmodel.KeywordRule{
			{Keywords: "price, cost", Match: automationsmodel.MatchAny, Reply: "Our price list is on the website."},
		}},
	}
	if t.store {
		order := templates["order_confirmation"]
		cart := templates["cart_reminder"]
		first, second := templates["welcome_series_1"], templates["welcome_series_2"]
		vip := tags["vip"]
		on[automationsmodel.KindOrder] = &automationsmodel.OrderSettings{
			Trigger: automationsmodel.TriggerShopifyOrderCreated, TemplateID: &order,
			Variables: []automationsmodel.Variable{
				{Source: automationsmodel.SourceCustomerFirstName},
				{Source: automationsmodel.SourceOrderNumber},
				{Source: automationsmodel.SourceOrderTotal},
				{Source: automationsmodel.SourceTrackingURL},
			},
		}
		on[automationsmodel.KindCart] = &automationsmodel.CartSettings{
			DelayMinutes: 60, TemplateID: &cart, SecondReminder: true,
			Variables: []automationsmodel.Variable{
				{Source: automationsmodel.SourceCustomerFirstName}, {Source: automationsmodel.SourceCartTotal}, {Source: automationsmodel.SourceCheckoutURL},
			},
		}
		on[automationsmodel.KindDrip] = &automationsmodel.DripSettings{
			TagID: &vip, StopOnReply: true,
			Steps: []automationsmodel.DripStep{{Day: 0, TemplateID: &first}, {Day: 2, TemplateID: &second}},
		}
	}
	for _, kind := range automationsmodel.Kinds {
		settings, ok := on[kind]
		if !ok {
			continue
		}
		if _, err := s.a.automations.Save(s.ctx, ws, owner, kind, settings); err != nil {
			s.err = fmt.Errorf("seed %s: %w", kind, err)
			return
		}
		if _, err := s.a.automations.SetEnabled(s.ctx, ws, owner, kind, true); err != nil {
			s.err = fmt.Errorf("seed %s: %w", kind, err)
			return
		}
	}
}
