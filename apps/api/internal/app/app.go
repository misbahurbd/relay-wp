// Package app is the composition root: it builds dependencies and wires features together.
// It is the only package allowed to know about every feature.
package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	authhandler "github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/handler"
	authservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/service"
	automationsservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/service"
	billingservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/service"
	contactsservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/service"
	inboxservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/service"
	integrationsservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/service"
	onboardingservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/onboarding/service"
	profilesservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/profiles/service"
	teamservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/team/service"
	templatesservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/service"
	whatsappservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/service"
	workspacesservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/workspaces/service"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/httpx"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/jwt"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/mailer"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/middleware"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/queue"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/realtime"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/storage"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// Mode selects what the process runs.
type Mode string

// Run modes. Start with ModeAll on one small instance; split api and worker when load requires it.
const (
	ModeAPI    Mode = "api"
	ModeWorker Mode = "worker"
	ModeAll    Mode = "all"
)

// App holds every long-lived dependency, built once at startup.
type App struct {
	cfg  config.Config
	log  *slog.Logger
	mode Mode

	db       *pgxpool.Pool
	directDB *pgxpool.Pool
	queue    *queue.Client
	mailer   mailer.Mailer
	storage  storage.Storage
	jwt      *jwt.Manager

	auth         *authservice.Service
	accounts     *authservice.Accounts
	profiles     *profilesservice.Service
	directory    *workspacesservice.Directory
	workspaces   *workspacesservice.Service
	onboarding   *onboardingservice.Service
	whatsapp     *whatsappservice.Service
	team         *teamservice.Service
	access       *teamservice.Resolver
	billing      *billingservice.Service
	contacts     *contactsservice.Service
	templates    *templatesservice.Service
	automations  *automationsservice.Service
	integrations *integrationsservice.Service
	inbox        *inboxservice.Service
	// live fans inbox signals out to SSE streams; API processes only.
	live *realtime.Hub

	// payments is the payment gateway registry; tests set it to fakes before New builds features.
	payments *payment.Registry

	// oauth overrides the provider config built from cfg; tests point it at a fake provider.
	oauth *authservice.ProvidersConfig
	// graphURL overrides the Meta Graph API address; tests point it at a fake server.
	graphURL string
	// Tests point store and Google calls at fakes on localhost.
	allowPrivateTargets bool
	shopifyURL          string
	googleURL           string
	// inboxChannelsOverride replaces the inbox's channel registry; tests set fakes.
	inboxChannelsOverride inboxservice.Channels
}

// New connects to every dependency. Any misconfiguration fails here, not on the first request.
func New(ctx context.Context, cfg config.Config, log *slog.Logger, mode Mode) (_ *App, err error) {
	a := &App{cfg: cfg, log: log, mode: mode}
	defer func() {
		if err != nil {
			a.Close()
		}
	}()

	if a.db, err = database.New(ctx, cfg.Database.URL, cfg.Database.MaxConns); err != nil {
		return nil, err
	}
	if a.directDB, err = database.New(ctx, cfg.Database.DirectURL, cfg.Database.DirectMaxConns); err != nil {
		return nil, fmt.Errorf("direct connection: %w", err)
	}
	if a.mailer, err = mailer.New(cfg.Mailer, log); err != nil {
		return nil, err
	}
	if a.storage, err = storage.New(ctx, cfg.Storage, cfg.IsDevelopment()); err != nil {
		return nil, err
	}

	// Features need the queue to enqueue, and workers need the features, so the
	// queue gets the registry first and feature workers are added once built.
	// River looks a worker up when it works a job, so nothing runs in between.
	workers := a.workers()
	queueWorkers := workers
	if mode == ModeAPI {
		queueWorkers = nil // insert-only: the API enqueues, workers run elsewhere
	}
	if a.queue, err = queue.New(a.directDB, queueWorkers, cfg.Queue.MaxWorkers, log); err != nil {
		return nil, err
	}
	if err := a.buildFeatures(); err != nil {
		return nil, err
	}
	if mode != ModeAPI {
		a.addFeatureWorkers(workers)
	}
	return a, nil
}

// buildFeatures wires the feature services. Features never import each other;
// each gets what it needs from here through small interfaces.
// A worker process builds everything except the HTTP-only auth service.
func (a *App) buildFeatures() (err error) {
	a.accounts = authservice.NewAccounts(a.db)
	a.profiles = profilesservice.New(a.db, a.accounts)
	a.directory = workspacesservice.NewDirectory(a.db)
	a.onboarding = onboardingservice.New(a.db, a.directory, whatsappservice.NewChannels(a.db), a.accounts, a.queue,
		onboardingservice.Config{AppBaseURL: a.cfg.AppBaseURL})
	a.team = teamservice.New(a.db, a.directory, a.accounts, a.queue, teamservice.Config{AppBaseURL: a.cfg.AppBaseURL})
	a.access = teamservice.NewResolver(a.db)
	a.contacts = contactsservice.New(a.db, a.storage, a.queue, a.access)
	a.workspaces = workspacesservice.New(a.db, a.accounts, a.onboarding, a.team)
	if err := a.buildWhatsApp(); err != nil {
		return err
	}
	// Templates reach Meta through whatsapp, and hear its status webhooks (AD-9).
	a.templates = templatesservice.New(a.db, metaTemplates{wa: a.whatsapp}, a.storage)
	a.automations = automationsservice.New(a.db, a.queue)
	a.whatsapp.SetTemplateStatusSink(a.templates)
	if err := a.buildBilling(); err != nil {
		return err
	}
	if err := a.buildIntegrations(); err != nil {
		return err
	}
	a.buildInbox()
	if a.mode == ModeWorker {
		return nil
	}

	if a.jwt, err = jwt.New(a.cfg.Auth); err != nil {
		return err
	}

	var breach authservice.BreachChecker
	if a.cfg.Auth.PasswordBreachCheck {
		breach = authservice.NewHIBP()
	}
	oauthCfg := authservice.ProvidersConfig{
		Google:   authservice.GoogleConfig{ClientID: a.cfg.Auth.GoogleClientID, ClientSecret: a.cfg.Auth.GoogleClientSecret},
		Facebook: authservice.FacebookConfig{AppID: a.cfg.Auth.FacebookAppID, AppSecret: a.cfg.Auth.FacebookAppSecret},
	}
	if a.oauth != nil {
		oauthCfg = *a.oauth
	}
	providers, err := authservice.NewProviders(oauthCfg)
	if err != nil {
		return err
	}
	if a.log != nil {
		a.log.Info("social sign-in providers", "enabled", providers.Names())
	}
	emailCodeKey, err := a.secretKey("EMAIL_CODE_KEY", a.cfg.Auth.EmailCodeKey)
	if err != nil {
		return err
	}
	a.auth, err = authservice.New(a.db, a.jwt, a.queue, a.profiles, a.team, breach, providers, authservice.Config{
		RefreshTTL:       a.cfg.Auth.RefreshTokenTTL,
		SessionMaxAge:    a.cfg.Auth.SessionMaxAge,
		AppBaseURL:       a.cfg.AppBaseURL,
		APIBaseURL:       a.cfg.Auth.APIBaseURL,
		AllowedRedirects: a.cfg.Auth.AllowedRedirects,
		ReauthMaxAge:     a.cfg.Auth.ReauthMaxAge,
		EmailCodeKey:     emailCodeKey,
		TermsVersion:     a.cfg.Auth.TermsVersion,
	})
	return err
}

func (a *App) buildWhatsApp() error {
	wa := a.cfg.WhatsApp
	if wa.Enabled && wa.TokenEncryptionKey == "" {
		// A random development key would leave stored tokens unreadable after a restart.
		return errors.New("TOKEN_ENCRYPTION_KEY is required when WHATSAPP_ENABLED")
	}
	tokenKey, err := a.secretKey("TOKEN_ENCRYPTION_KEY", wa.TokenEncryptionKey)
	if err != nil {
		return err
	}
	base := a.graphURL
	if base == "" {
		base = whatsappservice.GraphBaseURL
	}
	graph := whatsappservice.NewGraph(&http.Client{Timeout: 15 * time.Second}, base+"/"+wa.GraphVersion, wa.AppID, wa.AppSecret)
	a.whatsapp, err = whatsappservice.New(a.db, graph, a.queue, a.onboarding, whatsappservice.Config{
		Enabled: wa.Enabled, AppID: wa.AppID, AppSecret: wa.AppSecret, ESConfigID: wa.ESConfigID,
		GraphVersion: wa.GraphVersion, WebhookVerifyToken: wa.WebhookVerifyToken, TokenKey: tokenKey,
	}, a.log)
	return err
}

// buildBilling builds the payment gateways for PAYMENT_PROVIDERS and the billing service.
func (a *App) buildBilling() error {
	if a.payments == nil {
		reg, err := payment.New(a.cfg.Payment, a.cfg.IsDevelopment())
		if err != nil {
			return err
		}
		a.payments = reg
	}
	if a.log != nil {
		a.log.Info("payment providers", "enabled", a.payments.Providers())
	}
	a.billing = billingservice.New(a.db, a.payments, a.queue, billingservice.Config{
		AppBaseURL: a.cfg.AppBaseURL, TrialPlan: a.cfg.Trial.Plan,
	}, a.log)
	a.team.SetPlanLimits(a.billing)
	a.whatsapp.SetPlanLimits(a.billing)
	a.contacts.SetPlanLimits(a.billing)
	a.workspaces.SetBilling(deletionGuard{a.billing})
	a.workspaces.SetTrials(a.billing)
	return nil
}

// buildInbox wires the inbox to whatsapp, contacts, templates, team and
// integrations (docs/PHASE-7-INBOX.md AD-1), and gives an API process its
// live-update hub (ADR 0009).
func (a *App) buildInbox() {
	a.inbox = inboxservice.New(a.db, a.queue, a.storage, inboxservice.Deps{
		Contacts:  inboxContacts{contacts: a.contacts},
		Channels:  a.inboxChannels(),
		Templates: inboxTemplates{templates: a.templates},
		Accounts:  inboxAccounts{wa: a.whatsapp},
		Events:    a.integrations,
		Members:   a.access,
	}, a.log)
	a.whatsapp.SetInboundSink(inboxSink{inbox: a.inbox})
	a.inbox.SetInboundHook(automationsHook{automations: a.automations})
	a.automations.Connect(automationsservice.Deps{
		Inbox: automationsInbox{inbox: a.inbox}, Contacts: automationsContacts{a.contacts}, Plans: a.billing,
	})
	if a.mode != ModeWorker && a.directDB != nil {
		a.live = realtime.New(a.directDB, a.log, realtime.InboxChannel)
		a.inbox.SetSubscriber(a.live)
	}
}

// deletionGuard gives the workspaces service billing's answer on whether a
// workspace may be deleted, in the workspaces feature's own type.
type deletionGuard struct{ billing *billingservice.Service }

func (g deletionGuard) DeletionCheck(ctx context.Context, workspaceID uuid.UUID) (workspacesservice.DeletionCheck, error) {
	c, err := g.billing.DeletionCheck(ctx, workspaceID)
	return workspacesservice.DeletionCheck{Allowed: c.Allowed, Reason: c.Reason}, err
}

// staff is the admin tools' guard: a verified account whose email is in
// ADMIN_EMAILS. Nil (everyone refused) when the list is empty.
func (a *App) staff() middleware.StaffChecker {
	if a.profiles == nil || len(a.cfg.Admin.Emails) == 0 {
		return nil
	}
	emails := make(map[string]bool, len(a.cfg.Admin.Emails))
	for _, e := range a.cfg.Admin.Emails {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			emails[e] = true
		}
	}
	return staffList{profiles: a.profiles, emails: emails}
}

type staffList struct {
	profiles *profilesservice.Service
	emails   map[string]bool
}

func (s staffList) IsStaff(ctx context.Context, userID uuid.UUID) (bool, error) {
	me, err := s.profiles.Me(ctx, userID)
	if err != nil {
		return false, err
	}
	return me.EmailVerified && s.emails[strings.ToLower(me.Email)], nil
}

func (a *App) buildIntegrations() error {
	// Store credentials and webhook secrets use the same key as Meta tokens.
	key, err := a.secretKey("TOKEN_ENCRYPTION_KEY", a.cfg.WhatsApp.TokenEncryptionKey)
	if err != nil {
		return err
	}
	google := integrationsservice.GoogleConfig{
		ClientID: a.cfg.Auth.GoogleClientID, ClientSecret: a.cfg.Auth.GoogleClientSecret,
		RedirectURL: a.cfg.Auth.APIBaseURL + "/v1/integrations/google/callback",
	}
	if a.googleURL != "" {
		google.AuthURL, google.TokenURL, google.UserInfoURL = a.googleURL+"/auth", a.googleURL+"/token", a.googleURL+"/userinfo"
	}
	a.integrations, err = integrationsservice.New(a.db, a.queue, integrationsservice.Config{
		SecretKey:           key,
		AllowPrivateTargets: a.cfg.IsDevelopment() || a.allowPrivateTargets,
		AppBaseURL:          a.cfg.AppBaseURL,
		APIBaseURL:          a.cfg.Auth.APIBaseURL,
		ShopifyBaseURL:      a.shopifyURL,
		Google:              google,
		Shopify: integrationsservice.ShopifyConfig{
			ClientID: a.cfg.Integrations.ShopifyClientID, ClientSecret: a.cfg.Integrations.ShopifyClientSecret,
		},
	}, a.log)
	if err != nil {
		return err
	}
	a.integrations.SetCommerceSink(automationsCommerce{automations: a.automations})
	if a.cfg.IsDevelopment() {
		a.warnIntegrationSetup()
	}
	return nil
}

// warnIntegrationSetup reminds a developer which one-click connections still need setup
// (docs/PHASE-5-INTEGRATIONS.md); without it those apps fall back to pasted credentials.
func (a *App) warnIntegrationSetup() {
	if a.log == nil {
		return
	}
	api := strings.TrimRight(a.cfg.Auth.APIBaseURL, "/")
	if a.cfg.Integrations.ShopifyClientID == "" || a.cfg.Integrations.ShopifyClientSecret == "" {
		a.log.Warn("integrations: Shopify one-click connect is off; create a Shopify Partner app and set SHOPIFY_CLIENT_ID and SHOPIFY_CLIENT_SECRET",
			"redirect_url", api+"/v1/integrations/shopify/callback")
	}
	if !strings.HasPrefix(api, "https://") {
		a.log.Warn("integrations: WooCommerce one-click connect is off; API_BASE_URL must be a public https address the store can reach",
			"api_base_url", api)
	}
	if a.cfg.Auth.GoogleClientID == "" || a.cfg.Auth.GoogleClientSecret == "" {
		a.log.Warn("integrations: Google Sheets can't connect; set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET (the Google sign-in app)",
			"redirect_url", api+"/v1/integrations/google/callback")
	}
}

// secretKey decodes a base64 32-byte key from config. Development gets a random
// key when it is empty, so a fresh checkout runs; anything it protected is
// unreadable after a restart, so `task setup` writes real ones to .env.
func (a *App) secretKey(name, value string) ([]byte, error) {
	if value == "" {
		if !a.cfg.IsDevelopment() {
			return nil, fmt.Errorf("%s is required", name)
		}
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if a.log != nil {
			a.log.Warn("using a random key for this run; run `task keygen:env` to keep one", "var", name)
		}
		return key, nil
	}
	key, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("%s must be 32 bytes, base64-encoded", name)
	}
	return key, nil
}

// Close releases database pools.
func (a *App) Close() {
	if a.db != nil {
		a.db.Close()
	}
	if a.directDB != nil {
		a.directDB.Close()
	}
}

// Run serves HTTP and/or works jobs (depending on the mode) until ctx is cancelled.
func (a *App) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)

	if a.mode != ModeAPI {
		// River gets an uncancelled context; it is stopped gracefully below.
		if err := a.queue.Start(context.WithoutCancel(gctx)); err != nil {
			return fmt.Errorf("start queue: %w", err)
		}
		a.log.Info("queue worker started", "max_workers", a.cfg.Queue.MaxWorkers)
		g.Go(func() error {
			<-gctx.Done()
			stopCtx, cancel := context.WithTimeout(context.WithoutCancel(gctx), a.cfg.HTTP.ShutdownTimeout)
			defer cancel()
			a.log.Info("queue worker stopping")
			if err := a.queue.Stop(stopCtx); err != nil {
				return fmt.Errorf("stop queue: %w", err)
			}
			return nil
		})
	}

	if a.live != nil {
		g.Go(func() error { return a.live.Run(gctx) })
	}

	if a.mode != ModeWorker {
		handler, err := a.Handler()
		if err != nil {
			return err
		}
		srv := httpx.NewServer(a.cfg.HTTPAddr, handler)
		g.Go(func() error { return httpx.Serve(gctx, srv, a.cfg.HTTP.ShutdownTimeout, a.log) })
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}

// Handler builds the HTTP router with middleware and every route.
func (a *App) Handler() (http.Handler, error) {
	ipr, err := middleware.NewClientIPResolver(a.cfg.HTTP.TrustedProxies)
	if err != nil {
		return nil, err
	}

	r := chi.NewRouter()
	r.Use(
		chimw.RequestID,
		ipr.Middleware,
		middleware.RequestLogger(a.log),
		middleware.Recoverer,
		middleware.SecurityHeaders,
		middleware.CORS(a.cfg.HTTP.CORSAllowedOrigins),
		middleware.RateLimit(a.cfg.HTTP.RateLimitRequests, a.cfg.HTTP.RateLimitWindow, "/healthz", "/readyz"),
		middleware.RouteLimits(append(authhandler.RateLimits(), routeLimits()...)...),
	)
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteError(w, apperr.NotFound("not_found", "Route not found"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteStatus(w, http.StatusMethodNotAllowed, "Method not allowed")
	})

	a.registerRoutes(newAPI(r))
	return r, nil
}

// OpenAPI returns the API spec without connecting to anything.
func OpenAPI() *huma.OpenAPI {
	api := newAPI(chi.NewRouter())
	(&App{}).registerRoutes(api)
	return api.OpenAPI()
}

func newAPI(r chi.Router) huma.API {
	httpx.InstallErrorHandler()
	cfg := huma.DefaultConfig("Relay API", "0.1.0")
	cfg.CreateHooks = nil // drop the "$schema" link huma adds to every response body
	cfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearer": {Type: "http", Scheme: "bearer", BearerFormat: "JWT", Description: "Mobile and API clients"},
		"cookie": {Type: "apiKey", In: "cookie", Name: "relay_access", Description: "Web app (httpOnly cookie)"},
	}
	return humachi.New(r, cfg)
}
