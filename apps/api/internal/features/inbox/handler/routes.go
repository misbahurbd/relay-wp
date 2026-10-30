package handler

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/service"
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

// MediaPath is the upload route, for its rate limit.
const MediaPath = "/v1/workspace/inbox/media"

// Register adds the inbox routes (docs/PHASE-7-INBOX.md section 6). Reading
// needs membership and is filtered by scope in the service; writes whose
// permission depends on the request (the message type, self-assign, closing
// your own chat) are checked in the service too (AD-10).
func Register(api huma.API, h *Handler, g Guards, security []map[string][]string) {
	ws := func(id, method, path, summary string, status int, mw ...Middleware) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: method, Path: "/v1/workspace/inbox" + path, Summary: summary,
			Tags: []string{"inbox"}, Security: security, DefaultStatus: status,
			Middlewares: append(huma.Middlewares{g.Auth, g.Member}, mw...),
		}
	}
	reply, notes := g.Can(authctx.PermInboxReply), g.Can(authctx.PermInboxNotes)
	templates := g.Can(authctx.PermInboxSendTemplate)
	ok, created, none := http.StatusOK, http.StatusCreated, http.StatusNoContent
	conv := "/conversations/{conversationId}"

	huma.Register(api, ws("list-inbox-conversations", http.MethodGet, "/conversations",
		"Your conversations for a tab, with the tab counts", ok), h.listConversations)
	huma.Register(api, ws("get-inbox-conversation", http.MethodGet, conv,
		"One conversation with its channel, capabilities, window, assignee, team and contact", ok), h.getConversation)
	huma.Register(api, ws("list-inbox-messages", http.MethodGet, conv+"/messages",
		"A page of the thread, newest first", ok), h.listMessages)
	huma.Register(api, ws("send-inbox-message", http.MethodPost, conv+"/messages",
		"Send text or a file (inbox.reply, open window) or a template (inbox.send_template, channels with templates); "+
			"it goes out in the background", created),
		h.sendMessage)
	huma.Register(api, ws("add-inbox-note", http.MethodPost, conv+"/notes",
		"Write a private note and notify the members it mentions", created, notes), h.addNote)
	huma.Register(api, ws("react-inbox-message", http.MethodPost, conv+"/messages/{messageId}/reaction",
		"React to a message with an emoji, or remove your reaction (channels with reactions)", none, reply), h.react)
	huma.Register(api, ws("read-inbox-conversation", http.MethodPost, conv+"/read",
		"Clear the unread count and send the customer a read receipt (channels with read receipts)", none), h.markRead)
	huma.Register(api, ws("assign-inbox-conversation", http.MethodPost, conv+"/assign",
		"Assign or unassign (inbox.close_reassign; anyone may take an unassigned chat)", ok), h.assign)
	huma.Register(api, ws("set-inbox-conversation-status", http.MethodPost, conv+"/status",
		"Open, close or snooze (inbox.close_reassign; you may close a chat assigned to you)", ok), h.setStatus)

	upload := ws("upload-inbox-media", http.MethodPost, "/media", "Upload a file to send; returns its key", created, reply)
	upload.MaxBodyBytes = service.MaxUploadBytes + 1<<20 // room for the multipart envelope
	huma.Register(api, upload, h.uploadMedia)
	media := ws("get-inbox-media", http.MethodGet, conv+"/messages/{messageId}/media", "Stream a message's file", ok)
	media.Responses = map[string]*huma.Response{"200": {
		Description: "The file, with its Content-Type and Content-Disposition",
		Content:     map[string]*huma.MediaType{"application/octet-stream": {Schema: &huma.Schema{Type: "string", Format: "binary"}}},
	}}
	huma.Register(api, media, h.media)

	huma.Register(api, ws("list-inbox-quick-replies", http.MethodGet, "/quick-replies", "Quick replies by shortcut", ok),
		h.listQuickReplies)
	huma.Register(api, ws("create-inbox-quick-reply", http.MethodPost, "/quick-replies", "Add a quick reply", created, reply),
		h.createQuickReply)
	huma.Register(api, ws("update-inbox-quick-reply", http.MethodPatch, "/quick-replies/{quickReplyId}",
		"Change a quick reply", ok, reply), h.updateQuickReply)
	huma.Register(api, ws("delete-inbox-quick-reply", http.MethodDelete, "/quick-replies/{quickReplyId}",
		"Delete a quick reply", none, reply), h.deleteQuickReply)

	huma.Register(api, ws("list-inbox-templates", http.MethodGet, "/templates",
		"Approved templates with the variables a send must fill (409 templates_unsupported for a channel without templates)",
		ok, templates), h.listTemplates)
	huma.Register(api, ws("list-inbox-mentions", http.MethodGet, "/mentions",
		"Notes that mention you, newest first", ok), h.listMentions)
	huma.Register(api, ws("mark-inbox-mentions-seen", http.MethodPost, "/mentions/seen",
		"Mark your mentions seen", ok), h.markMentionsSeen)

	events := ws("stream-inbox-events", http.MethodGet, "/events",
		"Live updates as Server-Sent Events: signals to refetch, never content", ok)
	events.Description = "Each event is `event: <kind>` (message, status, conversation, note or mention) with " +
		"`data: {\"ws\", \"kind\", \"conversation_id\", \"team_id\", \"assignee_id\", \"channel_type\", \"channel_id\", \"user_id\"}`. " +
		"Only chats you can open are sent. A `: ping` comment comes every 20 s; the stream ends after 15 minutes " +
		"or when the client falls behind, and EventSource reconnects."
	events.Responses = map[string]*huma.Response{"200": {
		Description: "An event stream",
		Content:     map[string]*huma.MediaType{"text/event-stream": {Schema: &huma.Schema{Type: "string"}}},
	}}
	huma.Register(api, events, h.events)
}
