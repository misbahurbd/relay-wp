package handler

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Middleware is a huma middleware (middleware.RequireAuth, middleware.RequireWorkspace, …).
type Middleware = func(huma.Context, func(huma.Context))

// Guards are the middlewares the routes need, built in internal/app.
type Guards struct {
	Auth   Middleware
	Member Middleware
	// Plan requires the workspace's plan to include automations.
	Plan Middleware
	// Can returns a middleware that requires every one of perms.
	Can func(perms ...authctx.Permission) Middleware
}

// Register adds the automation routes. Reading needs membership and the plan;
// editing settings and turning an automation on or off have separate permissions.
func Register(api huma.API, h *Handler, g Guards, security []map[string][]string) {
	ws := func(id, method, path, summary string, mw ...Middleware) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: method, Path: "/v1/workspace/automations" + path, Summary: summary,
			Tags: []string{"automations"}, Security: security, DefaultStatus: http.StatusOK,
			Middlewares: append(huma.Middlewares{g.Auth, g.Member, g.Plan}, mw...),
		}
	}
	huma.Register(api, ws("list-automations", http.MethodGet, "", "The six automations in page order"), h.list)
	huma.Register(api, ws("save-automation", http.MethodPut, "/{kind}", "Replace an automation's settings",
		g.Can(authctx.PermAutomationsEdit)), h.save)
	huma.Register(api, ws("set-automation-enabled", http.MethodPut, "/{kind}/enabled",
		"Turn an automation on or off; turning on needs its settings complete",
		g.Can(authctx.PermAutomationsPublish)), h.setEnabled)
}
