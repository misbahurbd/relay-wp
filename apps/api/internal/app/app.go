// Package app is the composition root: it builds dependencies and wires features together.
// It is the only package allowed to know about every feature.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/profiles"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/httpx"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/jwt"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/mailer"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/middleware"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/queue"
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

	auth     *auth.Service
	accounts *auth.Accounts
	profiles *profiles.Service

	// oauth overrides the provider config built from cfg; tests point it at a fake provider.
	oauth *auth.ProvidersConfig
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

	workers := a.workers()
	if mode == ModeAPI {
		workers = nil // insert-only: the API enqueues, workers run elsewhere
	}
	if a.queue, err = queue.New(a.directDB, workers, cfg.Queue.MaxWorkers, log); err != nil {
		return nil, err
	}

	if mode != ModeWorker {
		if err := a.buildFeatures(); err != nil {
			return nil, err
		}
	}
	return a, nil
}

// buildFeatures wires the feature services. Features never import each other;
// each gets what it needs from here through small interfaces.
func (a *App) buildFeatures() (err error) {
	if a.jwt, err = jwt.New(a.cfg.Auth); err != nil {
		return err
	}
	a.accounts = auth.NewAccounts(a.db)
	a.profiles = profiles.New(a.db, a.accounts)

	var breach auth.BreachChecker
	if a.cfg.Auth.PasswordBreachCheck {
		breach = auth.NewHIBP()
	}
	oauthCfg := auth.ProvidersConfig{
		Google:   auth.GoogleConfig{ClientID: a.cfg.Auth.GoogleClientID, ClientSecret: a.cfg.Auth.GoogleClientSecret},
		Facebook: auth.FacebookConfig{AppID: a.cfg.Auth.FacebookAppID, AppSecret: a.cfg.Auth.FacebookAppSecret},
	}
	if a.oauth != nil {
		oauthCfg = *a.oauth
	}
	providers, err := auth.NewProviders(oauthCfg)
	if err != nil {
		return err
	}
	if a.log != nil {
		a.log.Info("social sign-in providers", "enabled", providers.Names())
	}
	a.auth, err = auth.New(a.db, a.jwt, a.queue, a.profiles, breach, providers, auth.Config{
		RefreshTTL:       a.cfg.Auth.RefreshTokenTTL,
		AppBaseURL:       a.cfg.AppBaseURL,
		APIBaseURL:       a.cfg.Auth.APIBaseURL,
		AllowedRedirects: a.cfg.Auth.AllowedRedirects,
		ReauthMaxAge:     a.cfg.Auth.ReauthMaxAge,
	})
	return err
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
		middleware.CORS(a.cfg.HTTP.CORSAllowedOrigins),
		middleware.RateLimit(a.cfg.HTTP.RateLimitRequests, a.cfg.HTTP.RateLimitWindow, "/healthz", "/readyz"),
		middleware.RouteLimits(auth.RateLimits()...),
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
