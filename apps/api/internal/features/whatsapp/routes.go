package whatsapp

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// Middleware is a huma middleware (middleware.RequireAuth, middleware.RequireWorkspace).
type Middleware = func(huma.Context, func(huma.Context))

// Register adds the WhatsApp routes. member and owner run after requireAuth and
// check the {workspaceId} in the path. The webhook routes are public: Meta signs them.
func Register(api huma.API, h *Handler, requireAuth, member, owner Middleware, security []map[string][]string) {
	tags := []string{"whatsapp"}
	op := func(id, method, path, summary string, status int, mw ...Middleware) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: method, Path: path, Summary: summary, Tags: tags, DefaultStatus: status,
			Security: security, Middlewares: append(huma.Middlewares{requireAuth}, mw...),
		}
	}
	const ws = "/v1/workspaces/{workspaceId}/whatsapp"

	huma.Register(api, op("whatsapp-signup-config", http.MethodGet, "/v1/whatsapp/signup-config",
		"Public values for opening Meta's Embedded Signup", http.StatusOK), h.signupConfig)
	huma.Register(api, op("connect-whatsapp", http.MethodPost, ws+"/connections",
		"Finish Embedded Signup: save the number and start setting it up", http.StatusAccepted, owner), h.connect)
	huma.Register(api, op("list-whatsapp-numbers", http.MethodGet, ws+"/numbers",
		"The workspace's numbers and their setup progress", http.StatusOK, member), h.numbers)
	huma.Register(api, op("get-whatsapp-number", http.MethodGet, ws+"/numbers/{numberId}",
		"One number", http.StatusOK, member), h.number)
	huma.Register(api, op("send-test-message", http.MethodPost, ws+"/numbers/{numberId}/test-messages",
		"Send the hello_world template to check the number works", http.StatusAccepted, owner), h.sendTest)
	huma.Register(api, op("get-test-message", http.MethodGet, ws+"/test-messages/{messageId}",
		"A test message's delivery status", http.StatusOK, member), h.testMessage)

	webhook := func(id, method, summary string, status int) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: method, Path: "/v1/webhooks/whatsapp", Summary: summary, Tags: tags,
			DefaultStatus: status,
		}
	}
	huma.Register(api, webhook("whatsapp-webhook-verify", http.MethodGet,
		"Meta's webhook subscription handshake", http.StatusOK), h.verifyWebhook)
	huma.Register(api, webhook("whatsapp-webhook", http.MethodPost,
		"Meta's WhatsApp webhook (signed with the app secret)", http.StatusOK), h.receiveWebhook)
}
