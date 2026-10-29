package app

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/onboarding"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/profiles"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/workspaces"
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
	)
	if a.jwt != nil {
		verifier, sessions = a.jwt, a.accounts
	}
	requireAuth := middleware.RequireAuth(api, verifier, sessions, origins)

	auth.Register(api, auth.NewHandler(a.auth, auth.CookieConfig{
		Domain: a.cfg.Auth.CookieDomain, Secure: a.cfg.Auth.CookieSecure,
	}, origins), requireAuth)
	profiles.Register(api, profiles.NewHandler(a.profiles), requireAuth, auth.Security)

	var resolver middleware.AccessResolver
	if a.access != nil {
		resolver = a.access
	}
	member := middleware.RequireWorkspace(api, resolver)
	can := func(perms ...authctx.Permission) team.Middleware { return middleware.RequirePermission(api, perms...) }
	// Setup steps stay with the owner; connecting numbers is integrations work.
	owner := chain(member, middleware.RequireOwner(api))
	integrations := chain(member, can(authctx.PermIntegrationsManage))
	workspaces.Register(api, workspaces.NewHandler(a.workspaces), requireAuth, member, owner, auth.Security)
	onboarding.Register(api, onboarding.NewHandler(a.onboarding), requireAuth, member, owner, auth.Security)
	whatsapp.Register(api, whatsapp.NewHandler(a.whatsapp), requireAuth, member, integrations, auth.Security)
	team.Register(api, team.NewHandler(a.team), team.Guards{Auth: requireAuth, Member: member, Can: can}, auth.Security)
}

// chain runs middlewares in order as one.
func chain(mws ...team.Middleware) team.Middleware {
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
	}
}
