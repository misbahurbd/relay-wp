package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database/dbtest"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/storage"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// fakeMeta records calls instead of reaching Meta.
type fakeMeta struct {
	mu        sync.Mutex
	submitted []model.RemoteTemplate
	deleted   []string
	uploads   [][]byte
	remote    []model.RemoteTemplate
	submitErr error
}

func (f *fakeMeta) Submit(_ context.Context, _ uuid.UUID, t model.RemoteTemplate) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.submitErr != nil {
		return "", "", f.submitErr
	}
	f.submitted = append(f.submitted, t)
	if t.MetaID != "" {
		return t.MetaID, model.StatusPending, nil
	}
	return "9000" + t.Name, model.StatusPending, nil
}

func (f *fakeMeta) Delete(_ context.Context, _ uuid.UUID, metaID, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, metaID+"/"+name)
	return nil
}

func (f *fakeMeta) List(context.Context, uuid.UUID) ([]model.RemoteTemplate, error) {
	return f.remote, nil
}

func (f *fakeMeta) UploadSample(_ context.Context, _ uuid.UUID, _ string, data io.Reader, _ int64) (string, error) {
	b, err := io.ReadAll(data)
	if err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.uploads = append(f.uploads, b)
	return "handle-1", nil
}

type env struct {
	svc  *Service
	meta *fakeMeta
	pool *pgxpool.Pool
	ws   uuid.UUID
	user uuid.UUID
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool := dbtest.New(t)
	ctx := t.Context()
	user, ws := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email) VALUES ($1, $2)`, user, user.String()+"@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, name) VALUES ($1, 'Acme')`, ws); err != nil {
		t.Fatal(err)
	}
	store, err := storage.New(ctx, config.Storage{Driver: "memory"}, false)
	if err != nil {
		t.Fatal(err)
	}
	meta := &fakeMeta{}
	return &env{svc: New(pool, meta, store), meta: meta, pool: pool, ws: ws, user: user}
}

func (e *env) create(t *testing.T, d model.Draft) model.Template {
	t.Helper()
	tpl, err := e.svc.Create(t.Context(), e.ws, e.user, d)
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	ae, ok := apperr.As(err)
	if !ok || ae.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}

func TestCreateDraftAndDuplicate(t *testing.T) {
	e := newEnv(t)
	tpl := e.create(t, validDraft())
	if tpl.Status != model.StatusDraft || tpl.MetaTemplateID != nil || *tpl.CreatedBy != e.user {
		t.Fatalf("template = %+v", tpl)
	}
	if len(tpl.Components.Body.Examples) != 2 || tpl.Components.Buttons[0].Value != "https://acme.com/t" {
		t.Fatalf("components = %+v", tpl.Components)
	}
	_, err := e.svc.Create(t.Context(), e.ws, e.user, validDraft())
	wantCode(t, err, "template_exists")

	other := validDraft()
	other.Language = "es"
	e.create(t, other) // same name, other language is fine
}

func TestSubmitSetsPendingAndPatchIsRefused(t *testing.T) {
	e := newEnv(t)
	tpl := e.create(t, validDraft())

	got, err := e.svc.Submit(t.Context(), e.ws, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.StatusPending || got.SubmittedAt == nil || got.MetaTemplateID == nil || *got.MetaTemplateID != "9000order_update" {
		t.Fatalf("after submit: %+v", got)
	}
	if len(e.meta.submitted) != 1 || e.meta.submitted[0].MetaID != "" || e.meta.submitted[0].Components.Body.Examples[1] != "#4821" {
		t.Fatalf("meta got %+v", e.meta.submitted)
	}

	name := "renamed"
	_, err = e.svc.Update(t.Context(), e.ws, tpl.ID, model.Patch{Name: &name})
	wantCode(t, err, "template_in_review")
	_, err = e.svc.Submit(t.Context(), e.ws, tpl.ID)
	wantCode(t, err, "template_in_review")
}

func TestSubmitValidatesCompletely(t *testing.T) {
	e := newEnv(t)
	d := validDraft()
	d.Components.Body.Examples = nil // fine for a draft
	tpl := e.create(t, d)
	_, err := e.svc.Submit(t.Context(), e.ws, tpl.ID)
	wantCode(t, err, "invalid_template")
	if len(e.meta.submitted) != 0 {
		t.Fatal("an invalid template reached Meta")
	}
}

func TestSubmitPassesMetaErrorsThrough(t *testing.T) {
	e := newEnv(t)
	tpl := e.create(t, validDraft())
	e.meta.submitErr = apperr.Conflict("whatsapp_not_connected", "Connect a WhatsApp number first.")
	_, err := e.svc.Submit(t.Context(), e.ws, tpl.ID)
	wantCode(t, err, "whatsapp_not_connected")
	got, err := e.svc.Get(t.Context(), e.ws, tpl.ID)
	if err != nil || got.Status != model.StatusDraft {
		t.Fatalf("status after a failed submit = %v, %v", got.Status, err)
	}
}

func TestEditApprovedKeepsApprovedThenResubmitEdits(t *testing.T) {
	e := newEnv(t)
	tpl := e.create(t, validDraft())
	if _, err := e.svc.Submit(t.Context(), e.ws, tpl.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.TemplateStatusChanged(t.Context(), e.ws, "9000order_update", model.StatusApproved, ""); err != nil {
		t.Fatal(err)
	}

	c := validDraft().Components
	c.Footer.Text = "New footer"
	got, err := e.svc.Update(t.Context(), e.ws, tpl.ID, model.Patch{Components: &c})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.StatusApproved || got.Components.Footer.Text != "New footer" {
		t.Fatalf("after edit: %+v", got)
	}
	lang := "es"
	_, err = e.svc.Update(t.Context(), e.ws, tpl.ID, model.Patch{Language: &lang})
	wantCode(t, err, "invalid_template")

	if got, err = e.svc.Submit(t.Context(), e.ws, tpl.ID); err != nil {
		t.Fatal(err)
	}
	if got.Status != model.StatusPending || e.meta.submitted[1].MetaID != "9000order_update" {
		t.Fatalf("resubmit: %+v, meta %+v", got, e.meta.submitted)
	}
}

func TestStatusSinkUpdatesStatusAndReason(t *testing.T) {
	e := newEnv(t)
	tpl := e.create(t, validDraft())
	if _, err := e.svc.Submit(t.Context(), e.ws, tpl.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.TemplateStatusChanged(t.Context(), e.ws, "9000order_update", model.StatusRejected, "INVALID_FORMAT"); err != nil {
		t.Fatal(err)
	}
	got, _ := e.svc.Get(t.Context(), e.ws, tpl.ID)
	if got.Status != model.StatusRejected || got.RejectedReason == nil || *got.RejectedReason != "INVALID_FORMAT" {
		t.Fatalf("after rejection: %+v", got)
	}

	// A rejected template can be edited and submitted again; approval clears the reason.
	if _, err := e.svc.Submit(t.Context(), e.ws, tpl.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.TemplateStatusChanged(t.Context(), e.ws, "9000order_update", model.StatusApproved, "ignored"); err != nil {
		t.Fatal(err)
	}
	got, _ = e.svc.Get(t.Context(), e.ws, tpl.ID)
	if got.Status != model.StatusApproved || got.RejectedReason != nil {
		t.Fatalf("after approval: %+v", got)
	}

	// Another workspace's webhook, or an unknown template, changes nothing.
	if err := e.svc.TemplateStatusChanged(t.Context(), uuid.Must(uuid.NewV7()), "9000order_update", model.StatusDisabled, ""); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.TemplateStatusChanged(t.Context(), e.ws, "123", model.StatusPaused, ""); err != nil {
		t.Fatal(err)
	}
	got, _ = e.svc.Get(t.Context(), e.ws, tpl.ID)
	if got.Status != model.StatusApproved {
		t.Fatalf("status changed by an unrelated webhook: %s", got.Status)
	}
}

func TestDeleteCallsMetaOnlyWhenSubmitted(t *testing.T) {
	e := newEnv(t)
	draft := e.create(t, validDraft())
	if err := e.svc.Delete(t.Context(), e.ws, draft.ID); err != nil {
		t.Fatal(err)
	}
	if len(e.meta.deleted) != 0 {
		t.Fatalf("a draft was deleted on Meta: %v", e.meta.deleted)
	}

	tpl := e.create(t, validDraft())
	if _, err := e.svc.Submit(t.Context(), e.ws, tpl.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Delete(t.Context(), e.ws, tpl.ID); err != nil {
		t.Fatal(err)
	}
	if len(e.meta.deleted) != 1 || e.meta.deleted[0] != "9000order_update/order_update" {
		t.Fatalf("meta deletes = %v", e.meta.deleted)
	}
	_, err := e.svc.Get(t.Context(), e.ws, tpl.ID)
	wantCode(t, err, "template_not_found")
}

func TestSyncUpsertsByNameAndLanguage(t *testing.T) {
	e := newEnv(t)
	local := e.create(t, validDraft()) // same name and language as a remote one below
	e.meta.remote = []model.RemoteTemplate{
		{
			MetaID: "111", Name: "hello_world", Language: "en_US", Category: model.CategoryUtility, Status: model.StatusApproved,
			Components: model.Components{Header: model.Header{Type: model.HeaderText, Text: "Hello World"}, Body: model.Body{Text: "Welcome!"}},
		},
		{
			MetaID: "222", Name: "order_update", Language: "en_US", Category: model.CategoryUtility, Status: model.StatusRejected,
			RejectedReason: "INVALID_FORMAT", Components: model.Components{Body: model.Body{Text: "Order shipped"}},
		},
		{
			MetaID: "333", Name: "otp_code", Language: "en_US", Category: model.CategoryAuthentication, Status: model.StatusApproved,
			Components: model.Components{Body: model.Body{Text: "{{1}} is your code."}},
		},
		{MetaID: "444", Name: "Bad Name", Language: "en_US", Category: model.CategoryUtility, Status: model.StatusApproved},
	}
	n, err := e.svc.Sync(t.Context(), e.ws)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("synced %d, want 3", n)
	}
	got, err := e.svc.Get(t.Context(), e.ws, local.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.StatusRejected || *got.MetaTemplateID != "222" || got.Components.Body.Text != "Order shipped" {
		t.Fatalf("local row not updated: %+v", got)
	}

	// Running it again updates rather than duplicates.
	if _, err := e.svc.Sync(t.Context(), e.ws); err != nil {
		t.Fatal(err)
	}
	items, next, counts, err := e.svc.List(t.Context(), e.ws, model.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || next != nil || counts.All != 3 || counts.Approved != 2 || counts.Rejected != 1 {
		t.Fatalf("list = %d items, counts %+v", len(items), counts)
	}
}

func TestListFiltersAndPages(t *testing.T) {
	e := newEnv(t)
	for _, name := range []string{"alpha", "beta", "gamma", "delta"} {
		d := validDraft()
		d.Name = name
		e.create(t, d)
	}
	d := validDraft()
	d.Name, d.Category = "promo_sale", model.CategoryMarketing
	e.create(t, d)

	page, next, counts, err := e.svc.List(t.Context(), e.ws, model.ListFilter{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || next == nil || page[0].Name != "promo_sale" || counts.All != 5 {
		t.Fatalf("first page: %d items, next %v, counts %+v", len(page), next, counts)
	}
	rest, next, _, err := e.svc.List(t.Context(), e.ws, model.ListFilter{Limit: 10, BeforeID: next})
	if err != nil || len(rest) != 3 || next != nil {
		t.Fatalf("second page: %d items, next %v, err %v", len(rest), next, err)
	}

	marketing := model.CategoryMarketing
	items, _, counts, _ := e.svc.List(t.Context(), e.ws, model.ListFilter{Category: &marketing})
	if len(items) != 1 || counts.All != 1 {
		t.Fatalf("category filter: %d items, counts %+v", len(items), counts)
	}
	q := "ALP"
	items, _, _, _ = e.svc.List(t.Context(), e.ws, model.ListFilter{Q: &q})
	if len(items) != 1 || items[0].Name != "alpha" {
		t.Fatalf("search: %+v", items)
	}
	wild := "%"
	items, _, _, _ = e.svc.List(t.Context(), e.ws, model.ListFilter{Q: &wild})
	if len(items) != 0 {
		t.Fatalf("%% matched literally nothing, got %d", len(items))
	}
	pending := model.StatusPending
	items, _, _, _ = e.svc.List(t.Context(), e.ws, model.ListFilter{Status: &pending})
	if len(items) != 0 {
		t.Fatalf("status filter: %d", len(items))
	}
}

var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{1}, 100)...)

func TestHeaderSampleUploadAndSubmit(t *testing.T) {
	e := newEnv(t)
	d := validDraft()
	d.Components.Header = model.Header{Type: model.HeaderImage}
	tpl := e.create(t, d)

	// Wrong type for the header, too big, and lying about the content are refused.
	_, err := e.svc.UploadSample(t.Context(), e.ws, tpl.ID, "video/mp4", bytes.NewReader(pngBytes), int64(len(pngBytes)))
	wantCode(t, err, "invalid_sample")
	_, err = e.svc.UploadSample(t.Context(), e.ws, tpl.ID, "image/png", bytes.NewReader(pngBytes), 6<<20)
	wantCode(t, err, "invalid_sample")
	_, err = e.svc.UploadSample(t.Context(), e.ws, tpl.ID, "image/jpeg", bytes.NewReader(pngBytes), int64(len(pngBytes)))
	wantCode(t, err, "invalid_sample")

	// Submitting without a sample is refused.
	_, err = e.svc.Submit(t.Context(), e.ws, tpl.ID)
	wantCode(t, err, "invalid_template")

	got, err := e.svc.UploadSample(t.Context(), e.ws, tpl.ID, "image/png", bytes.NewReader(pngBytes), int64(len(pngBytes)))
	if err != nil {
		t.Fatal(err)
	}
	if got.HeaderSample == nil || got.HeaderSample.ContentType != "image/png" || got.HeaderSample.Size != int64(len(pngBytes)) {
		t.Fatalf("sample = %+v", got.HeaderSample)
	}
	if _, err := e.svc.Submit(t.Context(), e.ws, tpl.ID); err != nil {
		t.Fatal(err)
	}
	if len(e.meta.uploads) != 1 || !bytes.Equal(e.meta.uploads[0], pngBytes) {
		t.Fatal("the stored sample didn't reach Meta intact")
	}
	if e.meta.submitted[0].HeaderHandle != "handle-1" {
		t.Fatalf("handle = %q", e.meta.submitted[0].HeaderHandle)
	}
}

func TestOtherWorkspaceCannotSeeTemplate(t *testing.T) {
	e := newEnv(t)
	tpl := e.create(t, validDraft())
	_, err := e.svc.Get(t.Context(), uuid.Must(uuid.NewV7()), tpl.ID)
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}
