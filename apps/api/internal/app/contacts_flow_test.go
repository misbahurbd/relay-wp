package app

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	contactsservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/service"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/storage"
)

// contactsWorkspace is a team workspace whose app has in-memory file storage.
type contactsWorkspace struct {
	*teamWorkspace
	id uuid.UUID
}

func newContactsWorkspace(t *testing.T) *contactsWorkspace {
	t.Helper()
	e := newAuthEnv(t)
	store, err := storage.New(t.Context(), config.Storage{Driver: "memory"}, false)
	if err != nil {
		t.Fatal(err)
	}
	e.a.storage = store
	e.a.contacts = contactsservice.New(e.a.db, store, e.a.queue, e.a.access)
	if e.h, err = e.a.Handler(); err != nil {
		t.Fatal(err)
	}
	w := newTeamWorkspace(e)
	return &contactsWorkspace{teamWorkspace: w, id: uuid.MustParse(w.id)}
}

// upload posts a file as the multipart field "file".
func (b *browser) upload(path, name string, data []byte) resp {
	b.env.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", name)
	if err != nil {
		b.env.t.Fatal(err)
	}
	_, _ = fw.Write(data)
	_ = mw.Close()
	req := httptest.NewRequestWithContext(b.env.t.Context(), http.MethodPost, path, &body)
	req.RemoteAddr = b.ip + ":4321"
	req.Header.Set("User-Agent", b.ua)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Origin", testOrigin)
	if b.workspace != "" {
		req.Header.Set("X-Workspace-ID", b.workspace)
	}
	for _, c := range b.cookies {
		if strings.HasPrefix(path, c.Path) {
			req.AddCookie(c)
		}
	}
	rec := httptest.NewRecorder()
	b.env.h.ServeHTTP(rec, req)
	out := resp{code: rec.Code, raw: rec.Body.String(), header: rec.Header()}
	_ = json.Unmarshal(rec.Body.Bytes(), &out.body)
	return out
}

func items(r resp) []map[string]any {
	var out []map[string]any
	for _, it := range r.body["items"].([]any) {
		out = append(out, it.(map[string]any))
	}
	return out
}

// contactByPhone finds one contact through the list's phone search.
func (w *contactsWorkspace) contactByPhone(t *testing.T, phone string) map[string]any {
	t.Helper()
	r := w.owner.do("GET", w.base+"/contacts?q="+url.QueryEscape(phone), nil)
	expect(t, r, http.StatusOK)
	list := items(r)
	if len(list) != 1 {
		t.Fatalf("contacts with %s: %s", phone, r.raw)
	}
	got := w.owner.do("GET", w.base+"/contacts/"+list[0]["id"].(string), nil)
	expect(t, got, http.StatusOK)
	return got.body
}

func TestContactsCreateListAndFilter(t *testing.T) {
	w := newContactsWorkspace(t)
	o := w.owner

	dan := o.do("POST", w.base+"/contacts", map[string]any{"name": "Daniel Reed", "phone": "+44 7700 900412"})
	expect(t, dan, http.StatusCreated)
	if dan.body["phone"] != "+447700900412" || dan.body["phone_display"] != "+44 7700 900412" ||
		dan.body["opt_in"] != "pending" || dan.body["source"] != "manual" {
		t.Fatalf("created = %s", dan.raw)
	}
	// One number, two spellings: still one contact.
	expect(t, o.do("POST", w.base+"/contacts", map[string]any{"name": "Dan", "phone": "447700900412"}),
		http.StatusConflict, "contact_exists")
	expect(t, o.do("POST", w.base+"/contacts", map[string]any{"name": "Local", "phone": "07700 900412"}),
		http.StatusUnprocessableEntity, "invalid_phone")

	vip := o.do("POST", w.base+"/contact-tags", map[string]any{"name": "VIP"})
	expect(t, vip, http.StatusCreated)
	again := o.do("POST", w.base+"/contact-tags", map[string]any{"name": "vip"})
	expect(t, again, http.StatusOK)
	if again.body["id"] != vip.body["id"] {
		t.Fatalf("tag names are case-insensitive: %s vs %s", vip.raw, again.raw)
	}
	vipID := vip.body["id"].(string)

	ana := o.do("POST", w.base+"/contacts", map[string]any{
		"name": "Ana Silva", "phone": "+351 912 345 678", "email": "ana@example.com", "opt_in": "in",
		"tag_ids": []string{vipID}, "attributes": map[string]string{"City": "Lisbon"},
	})
	expect(t, ana, http.StatusCreated)
	if ana.body["opt_in_source"] != "manual" || ana.body["opt_in_at"] == nil || len(ana.body["tags"].([]any)) != 1 {
		t.Fatalf("ana = %s", ana.raw)
	}
	expect(t, o.do("POST", w.base+"/contacts", map[string]any{"name": "Bo", "phone": "+14155552671", "opt_in": "out"}),
		http.StatusCreated)

	// Inbox fills last_message_at later; set it by hand.
	if _, err := w.e.a.db.Exec(t.Context(), `UPDATE contacts SET last_message_at = CASE phone
		WHEN '+447700900412' THEN now() - interval '2 hours' WHEN '+14155552671' THEN now() - interval '100 days' END
		WHERE workspace_id = $1`, w.id); err != nil {
		t.Fatal(err)
	}

	count := func(query string, want int) []map[string]any {
		t.Helper()
		r := o.do("GET", w.base+"/contacts"+query, nil)
		expect(t, r, http.StatusOK)
		if got := int(r.body["total"].(float64)); got != want || len(items(r)) != want {
			t.Fatalf("%s: total %d, want %d; %s", query, got, want, r.raw)
		}
		return items(r)
	}
	all := count("", 3)
	if all[0]["name"] != "Bo" || all[2]["name"] != "Daniel Reed" {
		t.Fatalf("not newest first: %v, %v", all[0]["name"], all[2]["name"])
	}
	count("?opt_in=in", 1)
	count("?tag_id="+vipID, 1)
	count("?q=ANA", 1)
	count("?q=ana%40example", 1)
	count("?q=%2B44%207700", 1) // "+44 7700" matches the stored E.164
	count("?last_message=24h", 1)
	count("?last_message=over_90d", 1)
	count("?q=nobody", 0)

	// Paging keeps the total and ends without a cursor.
	p1 := o.do("GET", w.base+"/contacts?limit=2", nil)
	expect(t, p1, http.StatusOK)
	cursor, _ := p1.body["next_cursor"].(string)
	if len(items(p1)) != 2 || cursor == "" || p1.body["total"].(float64) != 3 {
		t.Fatalf("page 1 = %s", p1.raw)
	}
	p2 := o.do("GET", w.base+"/contacts?limit=2&cursor="+cursor, nil)
	expect(t, p2, http.StatusOK)
	if len(items(p2)) != 1 || p2.body["next_cursor"] != nil || items(p2)[0]["name"] != "Daniel Reed" {
		t.Fatalf("page 2 = %s", p2.raw)
	}
	expect(t, o.do("GET", w.base+"/contacts?cursor=nope", nil), http.StatusUnprocessableEntity, "invalid_cursor")

	sum := o.do("GET", w.base+"/contacts/summary", nil)
	expect(t, sum, http.StatusOK)
	if sum.body["total"] != 3.0 || sum.body["opted_in"] != 1.0 {
		t.Fatalf("summary = %s", sum.raw)
	}
}

func TestContactsEditBulkAndExport(t *testing.T) {
	w := newContactsWorkspace(t)
	o := w.owner
	dan := o.do("POST", w.base+"/contacts", map[string]any{"name": "Daniel", "phone": "+447700900412"})
	expect(t, dan, http.StatusCreated)
	ana := o.do("POST", w.base+"/contacts", map[string]any{"name": "=Ana", "phone": "+351912345678"})
	expect(t, ana, http.StatusCreated)
	danPath, anaID := w.base+"/contacts/"+dan.body["id"].(string), ana.body["id"].(string)

	expect(t, o.do("PATCH", danPath, map[string]any{"phone": "+351 912 345 678"}), http.StatusConflict, "contact_exists")
	edited := o.do("PATCH", danPath, map[string]any{"opt_in": "in", "attributes": map[string]string{"City": "Leeds"}})
	expect(t, edited, http.StatusOK)
	if edited.body["name"] != "Daniel" || edited.body["opt_in"] != "in" ||
		edited.body["attributes"].(map[string]any)["City"] != "Leeds" {
		t.Fatalf("edited = %s", edited.raw)
	}

	vip := o.do("POST", w.base+"/contact-tags", map[string]any{"name": "VIP"})
	expect(t, vip, http.StatusCreated)
	ids := []string{dan.body["id"].(string), anaID}
	tagged := o.do("POST", w.base+"/contacts/bulk", map[string]any{"action": "add_tag", "contact_ids": ids, "tag_id": vip.body["id"]})
	expect(t, tagged, http.StatusOK)
	if tagged.body["affected"] != 2.0 {
		t.Fatalf("add_tag = %s", tagged.raw)
	}
	tags := o.do("GET", w.base+"/contact-tags", nil)
	expect(t, tags, http.StatusOK)
	if !strings.Contains(tags.raw, `"contact_count":2`) {
		t.Fatalf("tags = %s", tags.raw)
	}
	expect(t, o.do("POST", w.base+"/contacts/bulk", map[string]any{"action": "add_tag", "contact_ids": ids}),
		http.StatusUnprocessableEntity, "invalid_input")

	members := o.do("GET", w.base+"/members", nil)
	expect(t, members, http.StatusOK)
	ownerID := members.body["members"].([]any)[0].(map[string]any)["user_id"].(string)
	assigned := o.do("POST", w.base+"/contacts/bulk", map[string]any{"action": "assign", "contact_ids": ids[:1], "user_id": ownerID})
	expect(t, assigned, http.StatusOK)
	got := o.do("GET", danPath, nil)
	expect(t, got, http.StatusOK)
	if a, _ := got.body["assigned_to"].(map[string]any); a == nil || a["id"] != ownerID || a["name"] != "Ada Lovelace" {
		t.Fatalf("assigned_to = %s", got.raw)
	}
	expect(t, o.do("POST", w.base+"/contacts/bulk", map[string]any{
		"action": "assign", "contact_ids": ids[:1], "user_id": uuid.NewString(),
	}), http.StatusUnprocessableEntity, "user_not_member")

	export := o.do("GET", w.base+"/contacts/export", nil)
	expect(t, export, http.StatusOK)
	if ct := export.header.Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("content type %q", ct)
	}
	rows, err := csv.NewReader(strings.NewReader(export.raw)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || strings.Join(rows[0], ",") != "name,phone,email,opt_in,tags,source,language,created_at,City" {
		t.Fatalf("export = %q", rows)
	}
	if rows[1][0] != "'=Ana" || rows[2][1] != "+447700900412" || rows[2][4] != "VIP" || rows[2][8] != "Leeds" {
		t.Fatalf("export rows = %q", rows[1:])
	}
	filtered := o.do("GET", w.base+"/contacts/export?opt_in=in", nil)
	expect(t, filtered, http.StatusOK)
	if n := strings.Count(strings.TrimSpace(filtered.raw), "\n"); n != 1 {
		t.Fatalf("filtered export has %d rows: %s", n, filtered.raw)
	}

	deleted := o.do("POST", w.base+"/contacts/bulk", map[string]any{"action": "delete", "contact_ids": []string{anaID}})
	expect(t, deleted, http.StatusOK)
	if deleted.body["affected"] != 1.0 {
		t.Fatalf("delete = %s", deleted.raw)
	}
	expect(t, o.do("DELETE", danPath, nil), http.StatusNoContent)
	expect(t, o.do("GET", danPath, nil), http.StatusNotFound, "contact_not_found")
}

func TestContactsPermissions(t *testing.T) {
	w := newContactsWorkspace(t)
	c := w.owner.do("POST", w.base+"/contacts", map[string]any{"name": "Daniel", "phone": "+447700900412"})
	expect(t, c, http.StatusCreated)
	ids := []string{c.body["id"].(string)}

	viewer, _ := w.join("viewer")
	expect(t, viewer.do("GET", w.base+"/contacts", nil), http.StatusOK)
	expect(t, viewer.do("POST", w.base+"/contacts", map[string]any{"name": "X", "phone": "+447700900413"}),
		http.StatusForbidden, "permission_denied")
	expect(t, viewer.do("GET", w.base+"/contacts/export", nil), http.StatusForbidden, "permission_denied")
	expect(t, viewer.upload(w.base+"/contacts/imports", "a.csv", []byte("phone\n+447700900413\n")),
		http.StatusForbidden, "permission_denied")
	expect(t, viewer.do("POST", w.base+"/contacts/bulk", map[string]any{"action": "assign", "contact_ids": ids}),
		http.StatusForbidden, "permission_denied")

	// Agents edit contacts but can't delete them.
	agent, _ := w.join("agent")
	expect(t, agent.do("POST", w.base+"/contacts/bulk", map[string]any{"action": "assign", "contact_ids": ids}), http.StatusOK)
	denied := agent.do("POST", w.base+"/contacts/bulk", map[string]any{"action": "delete", "contact_ids": ids})
	expect(t, denied, http.StatusForbidden, "permission_denied")
	if !strings.Contains(denied.raw, "contacts.delete") {
		t.Fatalf("denial should name the permission: %s", denied.raw)
	}
	expect(t, agent.do("DELETE", w.base+"/contacts/"+ids[0], nil), http.StatusForbidden, "permission_denied")

	// Another workspace's contact is not there.
	other := newTeamWorkspace(w.e)
	expect(t, other.owner.do("GET", other.base+"/contacts/"+ids[0], nil), http.StatusNotFound, "contact_not_found")
}

func TestContactImport(t *testing.T) {
	w := newContactsWorkspace(t)
	o := w.owner
	expect(t, o.do("POST", w.base+"/contacts", map[string]any{
		"name": "Unsubscribed", "phone": "+447700900412", "email": "keep@example.com", "opt_in": "out",
	}), http.StatusCreated)
	expect(t, o.do("POST", w.base+"/contacts", map[string]any{"name": "Pending", "phone": "+447700900413"}),
		http.StatusCreated)

	expect(t, o.upload(w.base+"/contacts/imports", "old.xls", []byte("x")), http.StatusUnprocessableEntity, "unsupported_file")
	expect(t, o.upload(w.base+"/contacts/imports", "big.csv", bytes.Repeat([]byte("a"), 10<<20+1)),
		http.StatusUnprocessableEntity, "file_too_large")

	file := "Name,Phone,Email,Tags,City\n" +
		"New Person,+44 7700 900414,new@example.com,\"VIP, Leeds\",Leeds\n" +
		"Updated,447700900413,,,York\n" +
		"Opted Out,+447700900412,,,\n" +
		"Bad,07700 900999,,,\n" +
		"Dup,+447700900414,,,\n"
	up := o.upload(w.base+"/contacts/imports", "contacts.csv", []byte(file))
	expect(t, up, http.StatusCreated)
	if up.body["row_count"] != 5.0 || up.body["file_name"] != "contacts.csv" {
		t.Fatalf("upload = %s", up.raw)
	}
	guesses := map[string]string{}
	for _, c := range up.body["columns"].([]any) {
		col := c.(map[string]any)
		guesses[col["name"].(string)] = col["guess"].(string)
	}
	if guesses["Name"] != "name" || guesses["Phone"] != "phone" || guesses["Email"] != "email" ||
		guesses["Tags"] != "tag" || guesses["City"] != "attribute" {
		t.Fatalf("guesses = %v", guesses)
	}
	path := w.base + "/contacts/imports/" + up.body["id"].(string)
	importID := uuid.MustParse(up.body["id"].(string))

	expect(t, o.do("GET", path+"/preview", nil), http.StatusConflict, "import_not_mapped")
	columns := []map[string]any{
		{"name": "Name", "field": "name"},
		{"name": "Phone", "field": "phone"},
		{"name": "Email", "field": "email"},
		{"name": "Tags", "field": "tag"},
		{"name": "City", "field": "attribute", "attribute_key": "City"},
	}
	expect(t, o.do("PUT", path+"/mapping", map[string]any{"columns": columns}), http.StatusUnprocessableEntity, "opt_in_required")
	mapped := o.do("PUT", path+"/mapping", map[string]any{"columns": columns, "tag_name": "March import", "opt_in_confirmed": true})
	expect(t, mapped, http.StatusOK)
	if mapped.body["status"] != "mapped" {
		t.Fatalf("mapped = %s", mapped.raw)
	}

	preview := o.do("GET", path+"/preview", nil)
	expect(t, preview, http.StatusOK)
	if preview.body["rows_found"] != 5.0 || preview.body["ready"] != 1.0 || preview.body["updates"] != 2.0 || preview.body["invalid"] != 2.0 {
		t.Fatalf("preview = %s", preview.raw)
	}
	var statuses []string
	for _, r := range preview.body["sample"].([]any) {
		statuses = append(statuses, r.(map[string]any)["status"].(string))
	}
	if strings.Join(statuses, ",") != "new,update,update,invalid,invalid" {
		t.Fatalf("sample statuses = %v", statuses)
	}

	started := o.do("POST", path+"/start", nil)
	expect(t, started, http.StatusAccepted)
	if started.body["status"] != "importing" {
		t.Fatalf("started = %s", started.raw)
	}
	expect(t, o.do("POST", path+"/start", nil), http.StatusConflict, "import_started")
	expect(t, o.do("PUT", path+"/mapping", map[string]any{"columns": columns, "opt_in_confirmed": true}),
		http.StatusConflict, "import_started")
	var jobs int
	if err := w.e.a.db.QueryRow(t.Context(),
		`SELECT count(id) FROM river_job WHERE kind = 'contacts_import' AND args->>'import_id' = $1`, importID.String(),
	).Scan(&jobs); err != nil || jobs != 1 {
		t.Fatalf("queued jobs = %d, %v", jobs, err)
	}

	// The test queue doesn't run workers; work the job the way the worker does.
	if err := w.e.a.contacts.RunImport(t.Context(), w.id, importID, false); err != nil {
		t.Fatal(err)
	}
	done := o.do("GET", path, nil)
	expect(t, done, http.StatusOK)
	if done.body["status"] != "done" || done.body["created"] != 1.0 || done.body["updated"] != 2.0 ||
		done.body["skipped"] != 2.0 || done.body["has_skipped_file"] != true {
		t.Fatalf("done = %s", done.raw)
	}

	added := w.contactByPhone(t, "447700900414")
	if added["name"] != "New Person" || added["opt_in"] != "in" || added["opt_in_source"] != "import" ||
		added["source"] != "import" || added["attributes"].(map[string]any)["City"] != "Leeds" {
		t.Fatalf("new contact = %v", added)
	}
	var tagNames []string
	for _, tg := range added["tags"].([]any) {
		tagNames = append(tagNames, tg.(map[string]any)["name"].(string))
	}
	if strings.Join(tagNames, ",") != "Leeds,March import,VIP" {
		t.Fatalf("tags = %v", tagNames)
	}
	updated := w.contactByPhone(t, "447700900413")
	if updated["name"] != "Updated" || updated["opt_in"] != "in" || updated["source"] != "manual" ||
		updated["attributes"].(map[string]any)["City"] != "York" {
		t.Fatalf("updated contact = %v", updated)
	}
	// An import never re-subscribes anyone, and empty cells keep what was there.
	out := w.contactByPhone(t, "447700900412")
	if out["opt_in"] != "out" || out["email"] != "keep@example.com" || out["name"] != "Opted Out" {
		t.Fatalf("opted-out contact = %v", out)
	}

	skipped := o.do("GET", path+"/skipped", nil)
	expect(t, skipped, http.StatusOK)
	rows, err := csv.NewReader(strings.NewReader(skipped.raw)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0][0] != "row" || rows[0][6] != "error" || rows[1][2] != "07700 900999" ||
		rows[2][0] != "5" || !strings.Contains(rows[2][6], "duplicate") {
		t.Fatalf("skipped = %q", rows)
	}

	// The worker is idempotent once the import is done.
	if err := w.e.a.contacts.RunImport(t.Context(), w.id, importID, false); err != nil {
		t.Fatal(err)
	}
}

// planCap is a billing plan with the same cap on every resource.
type planCap int

func (c planCap) Limit(context.Context, uuid.UUID, string) (int, error) { return int(c), nil }

func TestContactLimitStopsAddsAndImports(t *testing.T) {
	w := newContactsWorkspace(t)
	o := w.owner
	w.e.a.contacts.SetPlanLimits(planCap(2))

	expect(t, o.do("POST", w.base+"/contacts", map[string]any{"name": "One", "phone": "+447700900401"}), http.StatusCreated)
	expect(t, o.do("POST", w.base+"/contacts", map[string]any{"name": "Two", "phone": "+447700900402"}), http.StatusCreated)
	expect(t, o.do("POST", w.base+"/contacts", map[string]any{"name": "Three", "phone": "+447700900403"}),
		http.StatusConflict, "plan_contact_limit")

	file := "Name,Phone\nOne again,+447700900401\nThree,+447700900403\nFour,+447700900404\n"
	up := o.upload(w.base+"/contacts/imports", "more.csv", []byte(file))
	expect(t, up, http.StatusCreated)
	path := w.base + "/contacts/imports/" + up.body["id"].(string)
	columns := []map[string]any{{"name": "Name", "field": "name"}, {"name": "Phone", "field": "phone"}}
	expect(t, o.do("PUT", path+"/mapping", map[string]any{"columns": columns, "opt_in_confirmed": true}), http.StatusOK)
	expect(t, o.do("POST", path+"/start", nil), http.StatusAccepted)
	if err := w.e.a.contacts.RunImport(t.Context(), w.id, uuid.MustParse(up.body["id"].(string)), false); err != nil {
		t.Fatal(err)
	}
	done := o.do("GET", path, nil)
	expect(t, done, http.StatusOK)
	if done.body["created"] != 0.0 || done.body["updated"] != 1.0 || done.body["skipped"] != 2.0 {
		t.Fatalf("an import at the cap should only update: %s", done.raw)
	}

	w.e.a.contacts.SetPlanLimits(planCap(3))
	expect(t, o.do("POST", w.base+"/contacts", map[string]any{"name": "Three", "phone": "+447700900403"}), http.StatusCreated)
}
