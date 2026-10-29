package handler

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// Middleware is a huma middleware (middleware.RequireAuth, middleware.RequireWorkspace).
type Middleware = func(huma.Context, func(huma.Context))

// Register adds the workspace routes. member and owner run after requireAuth and
// check the X-Workspace-ID header.
func Register(api huma.API, h *Handler, requireAuth, member, owner Middleware, security []map[string][]string) {
	op := func(id, method, path, summary string, status int, mw ...Middleware) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: method, Path: path, Summary: summary, Tags: []string{"workspaces"},
			Security: security, Middlewares: append(huma.Middlewares{requireAuth}, mw...), DefaultStatus: status,
		}
	}
	huma.Register(api, op("list-workspaces", http.MethodGet, "/v1/workspaces",
		"My workspaces, with enough setup state to choose where to go", http.StatusOK), h.list)
	huma.Register(api, op("create-workspace", http.MethodPost, "/v1/workspaces",
		"Create a workspace from the Welcome screen and start its trial", http.StatusCreated), h.create)
	huma.Register(api, op("get-workspace", http.MethodGet, "/v1/workspace",
		"Business details, plan and trial", http.StatusOK, member), h.get)
	huma.Register(api, op("save-business", http.MethodPut, "/v1/workspace/onboarding/business",
		"Save setup step 1: business details", http.StatusOK, owner), h.saveBusiness)
}
