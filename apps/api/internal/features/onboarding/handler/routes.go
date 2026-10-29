package handler

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// Middleware is a huma middleware (middleware.RequireAuth, middleware.RequireWorkspace).
type Middleware = func(huma.Context, func(huma.Context))

// Register adds the onboarding routes. member and owner run after requireAuth and
// check the X-Workspace-ID header.
func Register(api huma.API, h *Handler, requireAuth, member, owner Middleware, security []map[string][]string) {
	op := func(id, method, path, summary string, mw Middleware) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: method, Path: "/v1/workspace/onboarding" + path, Summary: summary,
			Tags: []string{"onboarding"}, Security: security, Middlewares: huma.Middlewares{requireAuth, mw},
		}
	}
	huma.Register(api, op("get-onboarding", http.MethodGet, "",
		"Setup progress, checklist and this member's tour state", member), h.get)
	huma.Register(api, op("skip-whatsapp", http.MethodPost, "/skip-whatsapp",
		"Finish setup without connecting WhatsApp", owner), h.skipWhatsApp)
	huma.Register(api, op("complete-onboarding", http.MethodPost, "/complete",
		"Finish setup after connecting WhatsApp", owner), h.complete)
	huma.Register(api, op("update-onboarding-member", http.MethodPatch, "/me",
		"Record that this member saw the tour or hid the checklist", member), h.updateMemberState)
}
