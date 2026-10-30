package app

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	authhandler "github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/handler"
	billinghandler "github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/handler"
	contactshandler "github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/handler"
	inboxhandler "github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/handler"
	integrationshandler "github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/handler"
	onboardinghandler "github.com/misbahurbd/relay-wp/apps/api/internal/features/onboarding/handler"
	profileshandler "github.com/misbahurbd/relay-wp/apps/api/internal/features/profiles/handler"
	teamhandler "github.com/misbahurbd/relay-wp/apps/api/internal/features/team/handler"
	templateshandler "github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/handler"
	whatsapphandler "github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/handler"
	workspaceshandler "github.com/misbahurbd/relay-wp/apps/api/internal/features/workspaces/handler"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/middleware"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

type statusOutput struct {
	Body struct {
		Status string `json:"status" example:"ok"`
	}
}

func ok() *statusOutput {
	out := &statusOutput{}
	out.Body.Status = "ok"
	return out
}

// registerRoutes registers system routes, then each feature's routes.
// a may be zero-valued when only the OpenAPI spec is needed; handlers are never called then.
func (a *App) registerRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "health",
		Method:      http.MethodGet,
		Path:        "/healthz",
		Summary:     "Liveness check",
		Description: "Returns ok while the process is running. Does not check dependencies.",
		Tags:        []string{"system"},
	}, func(context.Context, *struct{}) (*statusOutput, error) {
		return ok(), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "ready",
		Method:      http.MethodGet,
		Path:        "/readyz",
		Summary:     "Readiness check",
		Description: "Returns ok when the database answers; 503 otherwise.",
		Tags:        []string{"system"},
		Errors:      []int{http.StatusServiceUnavailable},
	}, func(ctx context.Context, _ *struct{}) (*statusOutput, error) {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := a.db.Ping(pingCtx); err != nil {
			return nil, apperr.Unavailable("database_unavailable", "Database is not reachable").Wrap(err)
		}
		return ok(), nil
	})

	origins := a.cfg.HTTP.CORSAllowedOrigins
	var (
		verifier middleware.TokenVerifier
		sessions middleware.SessionChecker
		keys     middleware.KeyVerifier
	)
	if a.jwt != nil {
		verifier, sessions = a.jwt, a.accounts
	}
	if a.integrations != nil {
		keys = a.integrations
	}
	requireAuth := middleware.RequireAuth(api, verifier, sessions, keys, origins)

	authhandler.Register(api, authhandler.NewHandler(a.auth, authhandler.CookieConfig{
		Domain: a.cfg.Auth.CookieDomain, Secure: a.cfg.Auth.CookieSecure,
	}, origins), requireAuth)
	profileshandler.Register(api, profileshandler.NewHandler(a.profiles), requireAuth, authhandler.Security)

	var resolver middleware.AccessResolver
	if a.access != nil {
		resolver = a.access
	}
	member := middleware.RequireWorkspace(api, resolver)
	can := func(perms ...authctx.Permission) teamhandler.Middleware {
		return middleware.RequirePermission(api, perms...)
	}
	// Setup steps stay with the owner; connecting numbers is integrations work.
	owner := chain(member, middleware.RequireOwner(api))
	integrations := chain(member, can(authctx.PermIntegrationsManage))
	workspaceshandler.Register(api, workspaceshandler.NewHandler(a.workspaces), requireAuth, member, owner, authhandler.Security)
	onboardinghandler.Register(api, onboardinghandler.NewHandler(a.onboarding), requireAuth, member, owner, authhandler.Security)
	whatsapphandler.Register(api, whatsapphandler.NewHandler(a.whatsapp), requireAuth, member, integrations, authhandler.Security)
	teamhandler.Register(api, teamhandler.NewHandler(a.team), teamhandler.Guards{Auth: requireAuth, Member: member, Can: can}, authhandler.Security)
	billinghandler.Register(api, billinghandler.NewHandler(a.billing), billinghandler.Guards{
		Auth: requireAuth, Member: member, Can: can, Staff: middleware.RequireStaff(api, a.staff()),
	}, authhandler.Security)
	contactshandler.Register(api, contactshandler.NewHandler(a.contacts), contactshandler.Guards{Auth: requireAuth, Member: member, Can: can}, authhandler.Security)
	templateshandler.Register(api, templateshandler.NewHandler(a.templates),
		templateshandler.Guards{Auth: requireAuth, Member: member, Can: can}, authhandler.Security)
	inboxhandler.Register(api, inboxhandler.NewHandler(a.inbox),
		inboxhandler.Guards{Auth: requireAuth, Member: member, Can: can}, authhandler.Security)
	integrationshandler.Register(api, integrationshandler.NewHandler(a.integrations, integrationshandler.CookieConfig{
		Domain: a.cfg.Auth.CookieDomain, Secure: a.cfg.Auth.CookieSecure,
	}), integrationshandler.Guards{
		Auth: requireAuth, Session: middleware.RequireSession(api), Member: member,
		Keys:   chain(member, can(authctx.PermIntegrationsAPIKeys)),
		Manage: integrations,
	}, authhandler.Security)
}

// chain runs middlewares in order as one.
func chain(mws ...teamhandler.Middleware) teamhandler.Middleware {
	return func(ctx huma.Context, next func(huma.Context)) {
		run := next
		for i := len(mws) - 1; i >= 0; i-- {
			mw, after := mws[i], run
			run = func(c huma.Context) { mw(c, after) }
		}
		run(ctx)
	}
}

// routeLimits are per-IP limits for Phase 3 endpoints, on top of the global limit.
func routeLimits() []middleware.RouteLimit {
	return []middleware.RouteLimit{
		{Method: http.MethodPost, Path: "/v1/workspaces", Requests: 10, Window: time.Hour},
		{Method: http.MethodPost, Path: "/v1/invitations/preview", Requests: 30, Window: time.Minute},
		{Method: http.MethodPost, Path: "/v1/invitations/accept", Requests: 10, Window: time.Minute},
		{Method: http.MethodPost, Path: "/v1/invitations/request-renewal", Requests: 5, Window: time.Hour},
		{Method: http.MethodGet, Path: "/v1/integrations/shopify/callback", Requests: 30, Window: time.Minute},
		{Method: http.MethodPost, Path: "/v1/integrations/woocommerce/callback", Requests: 30, Window: time.Minute},
		{Method: http.MethodPost, Path: inboxhandler.MediaPath, Requests: 60, Window: time.Minute},
	}
}
