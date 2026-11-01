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
	// Can returns a middleware that requires every one of perms.
	Can func(perms ...authctx.Permission) Middleware
}

// Register adds the broadcast routes (docs/PHASE-9-BROADCASTS.md section 6).
// Reading needs broadcasts.reports, editing drafts broadcasts.create, and
// sending, scheduling and cancelling broadcasts.send.
func Register(api huma.API, h *Handler, g Guards, security []map[string][]string) {
	reports, create, send := g.Can(authctx.PermBroadcastsReports), g.Can(authctx.PermBroadcastsCreate), g.Can(authctx.PermBroadcastsSend)
	ws := func(id, method, path, summary string, status int, perm Middleware) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: method, Path: "/v1/workspace/broadcasts" + path, Summary: summary,
			Tags: []string{"broadcasts"}, Security: security, DefaultStatus: status,
			Middlewares: huma.Middlewares{g.Auth, g.Member, perm},
		}
	}
	ok, created, none := http.StatusOK, http.StatusCreated, http.StatusNoContent

	huma.Register(api, ws("list-broadcasts", http.MethodGet, "", "Broadcasts, newest first, with delivery counts", ok, reports), h.list)
	huma.Register(api, ws("create-broadcast", http.MethodPost, "", "Save a new draft", created, create), h.create)
	huma.Register(api, ws("preview-broadcast-audience", http.MethodPost, "/audience-preview",
		"How many opted-in contacts an audience reaches", ok, create), h.previewAudience)
	huma.Register(api, ws("get-broadcast", http.MethodGet, "/{broadcastId}", "One broadcast and its report", ok, reports), h.get)
	huma.Register(api, ws("update-broadcast", http.MethodPut, "/{broadcastId}",
		"Replace a draft or scheduled broadcast's content", ok, create), h.update)
	huma.Register(api, ws("delete-broadcast", http.MethodDelete, "/{broadcastId}", "Delete a draft", none, create), h.delete)
	huma.Register(api, ws("duplicate-broadcast", http.MethodPost, "/{broadcastId}/duplicate",
		"Copy a broadcast into a new draft", created, create), h.duplicate)
	huma.Register(api, ws("send-broadcast", http.MethodPost, "/{broadcastId}/send",
		"Send a draft now or schedule it; on a scheduled one, move the time or send now", ok, send), h.send)
	huma.Register(api, ws("cancel-broadcast", http.MethodPost, "/{broadcastId}/cancel",
		"Stop a scheduled or sending broadcast; messages already queued still go", ok, send), h.cancel)
	huma.Register(api, ws("list-broadcast-recipients", http.MethodGet, "/{broadcastId}/recipients",
		"Who a broadcast went to, and what happened", ok, reports), h.recipients)
}
