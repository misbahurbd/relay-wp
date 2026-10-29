// Package service holds templates' business rules (no HTTP, no SQL): drafts, the AD-7
// validation rules, submitting to Meta through MetaTemplates, syncing and status updates.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/storage"
)

// MetaTemplates is the workspace's WhatsApp Business account on Meta. Implemented in
// internal/app on top of features/whatsapp, which resolves the account and its token.
type MetaTemplates interface {
	// Submit creates the template on Meta, or edits it when t.MetaID is set.
	// It returns Meta's template ID and our status for it.
	Submit(ctx context.Context, workspaceID uuid.UUID, t model.RemoteTemplate) (metaID, status string, err error)
	// Delete removes the template from Meta.
	Delete(ctx context.Context, workspaceID uuid.UUID, metaID, name string) error
	// List returns every template on the workspace's account.
	List(ctx context.Context, workspaceID uuid.UUID) ([]model.RemoteTemplate, error)
	// UploadSample uploads a media header sample and returns its handle for a submit.
	UploadSample(ctx context.Context, workspaceID uuid.UUID, contentType string, data io.Reader, size int64) (handle string, err error)
}

// Service manages message templates.
type Service struct {
	pool  *pgxpool.Pool
	meta  MetaTemplates
	store storage.Storage
}

// New builds the service.
func New(pool *pgxpool.Pool, meta MetaTemplates, store storage.Storage) *Service {
	return &Service{pool: pool, meta: meta, store: store}
}

func (s *Service) load(ctx context.Context, r repository.Repository, workspaceID, id uuid.UUID) (model.Template, error) {
	row, err := r.GetTemplate(ctx, gen.GetTemplateParams{WorkspaceID: workspaceID, ID: id})
	if repository.IsNoRows(err) {
		return model.Template{}, model.ErrNotFound
	}
	if err != nil {
		return model.Template{}, fmt.Errorf("get template: %w", err)
	}
	return toTemplate(row)
}

func toTemplate(r gen.GetTemplateRow) (model.Template, error) {
	t := model.Template{
		ID: r.ID, Name: r.Name, Language: r.Language, Category: r.Category, Status: r.Status,
		MetaTemplateID: r.MetaTemplateID, RejectedReason: r.RejectedReason, SubmittedAt: r.SubmittedAt,
		CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if err := json.Unmarshal(r.Components, &t.Components); err != nil {
		return model.Template{}, fmt.Errorf("decode components: %w", err)
	}
	normalizeComponents(&t.Components)
	if r.HeaderMediaKey != nil && r.HeaderMediaType != nil && r.HeaderMediaSize != nil {
		t.HeaderSample = &model.Sample{Key: *r.HeaderMediaKey, ContentType: *r.HeaderMediaType, Size: *r.HeaderMediaSize}
	}
	t.Warnings = warnings(t.Components)
	return t, nil
}

func draftOf(t model.Template) model.Draft {
	return model.Draft{Name: t.Name, Language: t.Language, Category: t.Category, Components: t.Components}
}

// editable reports whether a template's content may change in its status (AD-8).
func editable(status string) error {
	switch status {
	case model.StatusPending:
		return model.ErrInReview
	case model.StatusPaused, model.StatusDisabled:
		return model.ErrNotEditable
	}
	return nil
}
