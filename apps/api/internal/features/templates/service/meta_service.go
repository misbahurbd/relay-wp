package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// Header sample files Meta accepts, per header type (AD-8).
type sampleRule struct {
	kind    string
	ext     string
	maxSize int64
	magic   func([]byte) bool
}

const mb = 1 << 20

var sampleRules = map[string]sampleRule{
	"image/jpeg":      {model.HeaderImage, ".jpg", 5 * mb, func(b []byte) bool { return bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}) }},
	"image/png":       {model.HeaderImage, ".png", 5 * mb, func(b []byte) bool { return bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")) }},
	"video/mp4":       {model.HeaderVideo, ".mp4", 16 * mb, func(b []byte) bool { return len(b) >= 8 && string(b[4:8]) == "ftyp" }},
	"application/pdf": {model.HeaderDocument, ".pdf", 100 * mb, func(b []byte) bool { return bytes.HasPrefix(b, []byte("%PDF-")) }},
}

// MaxSampleSize is the largest header sample accepted (a PDF document).
const MaxSampleSize = 100 * mb

var sampleHelp = map[string]string{
	model.HeaderImage:    "Upload a JPG or PNG image of 5 MB or less",
	model.HeaderVideo:    "Upload an MP4 video of 16 MB or less",
	model.HeaderDocument: "Upload a PDF of 100 MB or less",
}

// sampleKind is the header type a sample's content type fits, or "".
func sampleKind(contentType string) string { return sampleRules[contentType].kind }

func sampleErr(headerType string) error {
	msg, ok := sampleHelp[headerType]
	if !ok {
		msg = "Choose an image, video or document header first"
	}
	return model.ErrInvalidSample.WithDetails(apperr.Detail{Field: "file", Message: msg})
}

// UploadSample stores the example file for a media header. It replaces any
// earlier sample. The file must match the header type and Meta's size limit,
// and its first bytes must match its declared type.
func (s *Service) UploadSample(ctx context.Context, workspaceID, id uuid.UUID, contentType string, data io.Reader, size int64) (model.Template, error) {
	r := repository.NewRepository(s.pool)
	t, err := s.load(ctx, r, workspaceID, id)
	if err != nil {
		return model.Template{}, err
	}
	if err := editable(t.Status); err != nil {
		return model.Template{}, err
	}
	headerType := t.Components.Header.Type
	rule, ok := sampleRules[contentType]
	if !ok || rule.kind != headerType || size <= 0 || size > rule.maxSize {
		return model.Template{}, sampleErr(headerType)
	}
	head := make([]byte, 16)
	n, err := io.ReadFull(data, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return model.Template{}, sampleErr(headerType)
	}
	if !rule.magic(head[:n]) {
		return model.Template{}, sampleErr(headerType)
	}
	key := fmt.Sprintf("templates/%s/%s/header-sample%s", workspaceID, id, rule.ext)
	if err := s.store.Put(ctx, key, io.MultiReader(bytes.NewReader(head[:n]), data), size, contentType); err != nil {
		return model.Template{}, fmt.Errorf("store sample: %w", err)
	}
	if err := r.SetTemplateSample(ctx, gen.SetTemplateSampleParams{
		WorkspaceID: workspaceID, ID: id, HeaderMediaKey: &key, HeaderMediaType: &contentType, HeaderMediaSize: &size,
	}); err != nil {
		return model.Template{}, fmt.Errorf("save sample: %w", err)
	}
	if old := t.HeaderSample; old != nil && old.Key != key {
		s.removeSample(ctx, old.Key)
	}
	return s.load(ctx, r, workspaceID, id)
}

func (s *Service) removeSample(ctx context.Context, key string) {
	if err := s.store.Delete(ctx, key); err != nil {
		slog.WarnContext(ctx, "delete template sample", "key", key, "err", err)
	}
}

// Submit validates the template completely (AD-7) and sends it to Meta: a
// create, or an edit once Meta knows it. The template is then pending review.
func (s *Service) Submit(ctx context.Context, workspaceID, id uuid.UUID) (model.Template, error) {
	r := repository.NewRepository(s.pool)
	t, err := s.load(ctx, r, workspaceID, id)
	if err != nil {
		return model.Template{}, err
	}
	if err := editable(t.Status); err != nil {
		return model.Template{}, err
	}
	if err := validate(draftOf(t), t.HeaderSample, true); err != nil {
		return model.Template{}, err
	}
	remote := model.RemoteTemplate{
		Name: t.Name, Language: t.Language, Category: t.Category, Status: t.Status, Components: t.Components,
	}
	if t.MetaTemplateID != nil {
		remote.MetaID = *t.MetaTemplateID
	}
	if sampleKind := t.Components.Header.Type; sampleKind == model.HeaderImage || sampleKind == model.HeaderVideo || sampleKind == model.HeaderDocument {
		if remote.HeaderHandle, err = s.uploadSampleToMeta(ctx, workspaceID, *t.HeaderSample); err != nil {
			return model.Template{}, err
		}
	}
	metaID, status, err := s.meta.Submit(ctx, workspaceID, remote)
	if err != nil {
		return model.Template{}, err
	}
	if status != model.StatusApproved && status != model.StatusRejected {
		status = model.StatusPending
	}
	if err := r.MarkTemplateSubmitted(ctx, gen.MarkTemplateSubmittedParams{
		WorkspaceID: workspaceID, ID: id, MetaTemplateID: &metaID, Status: status,
	}); err != nil {
		return model.Template{}, fmt.Errorf("mark submitted: %w", err)
	}
	return s.load(ctx, r, workspaceID, id)
}

func (s *Service) uploadSampleToMeta(ctx context.Context, workspaceID uuid.UUID, sample model.Sample) (string, error) {
	body, err := s.store.Get(ctx, sample.Key)
	if err != nil {
		return "", fmt.Errorf("open sample: %w", err)
	}
	defer func() { _ = body.Close() }()
	return s.meta.UploadSample(ctx, workspaceID, sample.ContentType, body, sample.Size)
}

// Delete removes a template, from Meta first when Meta knows it.
func (s *Service) Delete(ctx context.Context, workspaceID, id uuid.UUID) error {
	r := repository.NewRepository(s.pool)
	t, err := s.load(ctx, r, workspaceID, id)
	if err != nil {
		return err
	}
	if t.MetaTemplateID != nil {
		if err := s.meta.Delete(ctx, workspaceID, *t.MetaTemplateID, t.Name); err != nil {
			return err
		}
	}
	if err := r.DeleteTemplate(ctx, gen.DeleteTemplateParams{WorkspaceID: workspaceID, ID: id}); err != nil {
		return fmt.Errorf("delete template: %w", err)
	}
	if t.HeaderSample != nil {
		s.removeSample(ctx, t.HeaderSample.Key)
	}
	return nil
}

// Sync pulls every template from Meta and upserts it by (name, language), so
// templates made elsewhere, such as hello_world, appear. It returns how many it saved.
func (s *Service) Sync(ctx context.Context, workspaceID uuid.UUID) (int, error) {
	remote, err := s.meta.List(ctx, workspaceID)
	if err != nil {
		return 0, err
	}
	saved := 0
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		for _, rt := range remote {
			if !syncable(rt) {
				continue
			}
			c := rt.Components
			normalizeComponents(&c)
			components, err := json.Marshal(c)
			if err != nil {
				return fmt.Errorf("encode components: %w", err)
			}
			id, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("template id: %w", err)
			}
			var reason *string
			if rt.Status == model.StatusRejected && rt.RejectedReason != "" {
				reason = &rt.RejectedReason
			}
			metaID := rt.MetaID
			if err := r.UpsertSyncedTemplate(ctx, gen.UpsertSyncedTemplateParams{
				ID: id, WorkspaceID: workspaceID, Name: rt.Name, Language: rt.Language, Category: rt.Category,
				Status: rt.Status, MetaTemplateID: &metaID, RejectedReason: reason, Components: components,
			}); err != nil {
				return fmt.Errorf("save %s: %w", rt.Name, err)
			}
			saved++
		}
		return nil
	})
	return saved, err
}

// syncable skips what our table can't hold: unknown statuses or categories, odd names.
func syncable(t model.RemoteTemplate) bool {
	return t.MetaID != "" && nameRe.MatchString(t.Name) && t.Language != "" && len(t.Language) <= 15 &&
		slices.Contains(model.Categories, t.Category) && slices.Contains(model.Statuses, t.Status) &&
		t.Status != model.StatusDraft
}

// TemplateStatusChanged records Meta's review result from its webhook. A
// reason is kept only for a rejection. Unknown templates are ignored.
func (s *Service) TemplateStatusChanged(ctx context.Context, workspaceID uuid.UUID, metaTemplateID, status, reason string) error {
	if !slices.Contains(model.Statuses, status) || status == model.StatusDraft || metaTemplateID == "" {
		return nil
	}
	var why *string
	if reason != "" {
		why = &reason
	}
	if _, err := repository.NewRepository(s.pool).SetTemplateStatusByMetaID(ctx, gen.SetTemplateStatusByMetaIDParams{
		WorkspaceID: workspaceID, MetaTemplateID: &metaTemplateID, Status: status, RejectedReason: why,
	}); err != nil {
		return fmt.Errorf("set template status: %w", err)
	}
	return nil
}
