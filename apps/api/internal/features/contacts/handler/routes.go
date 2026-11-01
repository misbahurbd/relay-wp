package handler

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
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

// csvResponse documents a streamed CSV download.
func csvResponse(desc string) map[string]*huma.Response {
	return map[string]*huma.Response{
		"200": {
			Description: desc,
			Content:     map[string]*huma.MediaType{"text/csv": {Schema: &huma.Schema{Type: "string"}}},
		},
	}
}

// Register adds the contacts routes (docs/PHASE-4-CONTACTS-TEMPLATES.md AD-4, AD-5).
func Register(api huma.API, h *Handler, g Guards, security []map[string][]string) {
	ws := func(id, method, path, summary string, status int, mw ...Middleware) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: method, Path: "/v1/workspace" + path, Summary: summary,
			Tags: []string{"contacts"}, Security: security, DefaultStatus: status,
			Middlewares: append(huma.Middlewares{g.Auth, g.Member}, mw...),
		}
	}
	view, edit := g.Can(authctx.PermContactsView), g.Can(authctx.PermContactsEdit)
	del, export, imp := g.Can(authctx.PermContactsDelete), g.Can(authctx.PermContactsExport), g.Can(authctx.PermContactsImport)
	ok, created, none := http.StatusOK, http.StatusCreated, http.StatusNoContent

	huma.Register(api, ws("list-contacts", http.MethodGet, "/contacts",
		"Contacts, newest first, with the total for the filter", ok, view), h.listContacts)
	huma.Register(api, ws("get-contacts-summary", http.MethodGet, "/contacts/summary",
		"How many contacts there are and how many opted in", ok, view), h.summary)
	exportOp := ws("export-contacts", http.MethodGet, "/contacts/export",
		"Download the contacts matching the filter as CSV", ok, export)
	exportOp.Responses = csvResponse("CSV: name, phone, email, opt_in, tags, source, language, created_at, then one column per custom field")
	huma.Register(api, exportOp, h.export)
	// The permission depends on the action, so the service checks it.
	huma.Register(api, ws("bulk-contacts", http.MethodPost, "/contacts/bulk",
		"Tag, untag, assign or delete up to 500 contacts (delete needs contacts.delete, the rest contacts.edit)", ok), h.bulk)
	huma.Register(api, ws("get-contact", http.MethodGet, "/contacts/{contactId}",
		"One contact with its tags and custom fields", ok, view), h.getContact)
	huma.Register(api, ws("create-contact", http.MethodPost, "/contacts",
		"Add a contact", created, edit), h.createContact)
	huma.Register(api, ws("update-contact", http.MethodPatch, "/contacts/{contactId}",
		"Change a contact's fields", ok, edit), h.updateContact)
	huma.Register(api, ws("delete-contact", http.MethodDelete, "/contacts/{contactId}",
		"Delete a contact", none, del), h.deleteContact)

	huma.Register(api, ws("list-contact-tags", http.MethodGet, "/contact-tags",
		"Contact tags with how many contacts carry each", ok, view), h.listTags)
	huma.Register(api, ws("create-contact-tag", http.MethodPost, "/contact-tags",
		"Create a tag, or get the one with this name", created, edit), h.createTag)

	// Segments (ADR 0014): saved rules that group contacts on their own.
	huma.Register(api, ws("list-contact-segments", http.MethodGet, "/contact-segments",
		"Segments by name, each with who matches it now", ok, view), h.listSegments)
	huma.Register(api, ws("create-contact-segment", http.MethodPost, "/contact-segments",
		"Save a segment", created, edit), h.createSegment)
	huma.Register(api, ws("preview-contact-segment", http.MethodPost, "/contact-segments/preview",
		"Who rules match right now, before saving", ok, view), h.previewSegment)
	huma.Register(api, ws("get-contact-segment", http.MethodGet, "/contact-segments/{segmentId}",
		"One segment with who matches it now", ok, view), h.getSegment)
	huma.Register(api, ws("update-contact-segment", http.MethodPut, "/contact-segments/{segmentId}",
		"Replace a segment's name, description and rules", ok, edit), h.updateSegment)
	huma.Register(api, ws("delete-contact-segment", http.MethodDelete, "/contact-segments/{segmentId}",
		"Delete a segment; its contacts stay", none, edit), h.deleteSegment)

	upload := ws("upload-contact-import", http.MethodPost, "/contacts/imports",
		"Upload a .csv or .xlsx file to import", created, imp)
	upload.MaxBodyBytes = model.MaxImportBytes + 1<<20 // room for the multipart envelope
	huma.Register(api, upload, h.uploadImport)
	huma.Register(api, ws("set-contact-import-mapping", http.MethodPut, "/contacts/imports/{importId}/mapping",
		"Say what each column becomes", ok, imp), h.setMapping)
	huma.Register(api, ws("preview-contact-import", http.MethodGet, "/contacts/imports/{importId}/preview",
		"What the import would do, counted from the whole file", ok, imp), h.previewImport)
	huma.Register(api, ws("start-contact-import", http.MethodPost, "/contacts/imports/{importId}/start",
		"Start importing; poll the import for progress", http.StatusAccepted, imp), h.startImport)
	huma.Register(api, ws("get-contact-import", http.MethodGet, "/contacts/imports/{importId}",
		"An import's status and counts", ok, imp), h.getImport)
	skipped := ws("download-contact-import-skipped", http.MethodGet, "/contacts/imports/{importId}/skipped",
		"Download the rows the import skipped, with the reason", ok, imp)
	skipped.Responses = csvResponse("CSV: row, the file's columns, error")
	huma.Register(api, skipped, h.skippedRows)
}
