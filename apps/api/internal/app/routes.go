package app

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/profiles"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/middleware"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
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
}
