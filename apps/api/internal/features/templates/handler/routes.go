package handler

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/service"
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

// Register adds the message template routes (AD-8). Listing and reading need
// membership only; changes need the templates permissions.
func Register(api huma.API, h *Handler, g Guards, security []map[string][]string) {
	ws := func(id, method, path, summary string, status int, mw ...Middleware) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: method, Path: "/v1/workspaces/{workspaceId}/templates" + path, Summary: summary,
			Tags: []string{"templates"}, Security: security, DefaultStatus: status,
			Middlewares: append(huma.Middlewares{g.Auth, g.Member}, mw...),
		}
	}
	create, submit, remove := g.Can(authctx.PermTemplatesCreate), g.Can(authctx.PermTemplatesSubmit), g.Can(authctx.PermTemplatesDelete)
	ok, created, none := http.StatusOK, http.StatusCreated, http.StatusNoContent

	huma.Register(api, ws("list-templates", http.MethodGet, "",
		"Message templates with status tab counts, newest first", ok), h.list)
	huma.Register(api, ws("get-template", http.MethodGet, "/{templateId}", "One message template", ok), h.get)
	huma.Register(api, ws("create-template", http.MethodPost, "", "Save a new draft template", created, create), h.create)
	huma.Register(api, ws("update-template", http.MethodPatch, "/{templateId}",
		"Edit a draft, rejected or approved template", ok, create), h.update)

	sample := ws("upload-template-header-sample", http.MethodPost, "/{templateId}/header-sample",
		"Upload the example file for an image, video or document header", ok, create)
	sample.MaxBodyBytes = service.MaxSampleSize + 1<<20 // room for the multipart envelope
	huma.Register(api, sample, h.uploadSample)

	huma.Register(api, ws("submit-template", http.MethodPost, "/{templateId}/submit",
		"Send the template to Meta for review", ok, submit), h.submit)
	huma.Register(api, ws("delete-template", http.MethodDelete, "/{templateId}",
		"Delete a template, from Meta too once submitted", none, remove), h.delete)
	huma.Register(api, ws("sync-templates", http.MethodPost, "/sync",
		"Pull every template from the WhatsApp Business account", ok, create), h.sync)
}
