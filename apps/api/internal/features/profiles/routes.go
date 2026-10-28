package profiles

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// Register adds the profile routes. Every route needs a signed-in user.
func Register(api huma.API, h *Handler, requireAuth func(huma.Context, func(huma.Context)), security []map[string][]string) {
	op := func(id, method, path, summary string) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: method, Path: path, Summary: summary, Tags: []string{"profile"},
			Security: security, Middlewares: huma.Middlewares{requireAuth},
		}
	}
	huma.Register(api, op("get-me", http.MethodGet, "/v1/me", "The signed-in user, profile and preferences"), h.me)
	huma.Register(api, op("update-profile", http.MethodPatch, "/v1/me/profile", "Update name fields"), h.updateProfile)
	huma.Register(api, op("update-preferences", http.MethodPatch, "/v1/me/preferences", "Update locale and timezone"), h.updatePreferences)
}
