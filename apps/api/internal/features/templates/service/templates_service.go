package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// List page sizes.
const (
	DefaultPageSize = 25
	MaxPageSize     = 100
)

var errNameTaken = model.ErrNameTaken.WithDetails(apperr.Detail{Field: "name", Message: model.ErrNameTaken.Message})

// Get returns one template.
func (s *Service) Get(ctx context.Context, workspaceID, id uuid.UUID) (model.Template, error) {
	return s.load(ctx, repository.NewRepository(s.pool), workspaceID, id)
}

// List returns one page of templates, newest first, the id to continue after,
// and the status tab counts for the same category and search.
func (s *Service) List(ctx context.Context, workspaceID uuid.UUID, f model.ListFilter) ([]model.Template, *uuid.UUID, model.Counts, error) {
	if f.Limit <= 0 || f.Limit > MaxPageSize {
		f.Limit = DefaultPageSize
	}
	q := likePattern(f.Q)
	r := repository.NewRepository(s.pool)
	rows, err := r.ListTemplates(ctx, gen.ListTemplatesParams{
		WorkspaceID: workspaceID, Status: f.Status, Category: f.Category, Q: q, BeforeID: f.BeforeID, PageSize: f.Limit + 1,
	})
	if err != nil {
		return nil, nil, model.Counts{}, fmt.Errorf("list templates: %w", err)
	}
	var next *uuid.UUID
	if len(rows) > int(f.Limit) {
		rows = rows[:f.Limit]
		next = &rows[len(rows)-1].ID
	}
	out := make([]model.Template, len(rows))
	for i, row := range rows {
		if out[i], err = toTemplate(gen.GetTemplateRow(row)); err != nil {
			return nil, nil, model.Counts{}, err
		}
	}
	c, err := r.CountTemplates(ctx, gen.CountTemplatesParams{WorkspaceID: workspaceID, Category: f.Category, Q: q})
	if err != nil {
		return nil, nil, model.Counts{}, fmt.Errorf("count templates: %w", err)
	}
	return out, next, model.Counts{All: c.AllCount, Approved: c.Approved, Pending: c.Pending, Rejected: c.Rejected}, nil
}

// likePattern escapes a search so ILIKE matches it literally.
func likePattern(q *string) *string {
	if q == nil {
		return nil
	}
	s := strings.TrimSpace(*q)
	if s == "" {
		return nil
	}
	s = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
	return &s
}

// Create saves a new draft. A draft may be incomplete, but what it has must be valid.
func (s *Service) Create(ctx context.Context, workspaceID, userID uuid.UUID, d model.Draft) (model.Template, error) {
	normalize(&d)
	if err := validate(d, nil, false); err != nil {
		return model.Template{}, err
	}
	components, err := json.Marshal(d.Components)
	if err != nil {
		return model.Template{}, fmt.Errorf("encode components: %w", err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return model.Template{}, fmt.Errorf("template id: %w", err)
	}
	r := repository.NewRepository(s.pool)
	err = r.CreateTemplate(ctx, gen.CreateTemplateParams{
		ID: id, WorkspaceID: workspaceID, Name: d.Name, Language: d.Language, Category: d.Category,
		Components: components, CreatedBy: &userID,
	})
	if repository.IsUnique(err, repository.UniqueNameLanguage) {
		return model.Template{}, errNameTaken
	}
	if err != nil {
		return model.Template{}, fmt.Errorf("create template: %w", err)
	}
	return s.load(ctx, r, workspaceID, id)
}

// Update edits a draft, rejected or approved template. A template Meta is
// reviewing can't change, and a submitted one keeps its name and language.
// An approved template stays approved until it is submitted again.
func (s *Service) Update(ctx context.Context, workspaceID, id uuid.UUID, p model.Patch) (model.Template, error) {
	r := repository.NewRepository(s.pool)
	t, err := s.load(ctx, r, workspaceID, id)
	if err != nil {
		return model.Template{}, err
	}
	if err := editable(t.Status); err != nil {
		return model.Template{}, err
	}
	d := draftOf(t)
	if p.Name != nil {
		d.Name = *p.Name
	}
	if p.Language != nil {
		d.Language = *p.Language
	}
	if p.Category != nil {
		d.Category = *p.Category
	}
	if p.Components != nil {
		d.Components = *p.Components
	}
	normalize(&d)
	if t.MetaTemplateID != nil {
		var locked problems
		if d.Name != t.Name {
			locked.add("name", "A submitted template's name can't change")
		}
		if d.Language != t.Language {
			locked.add("language", "A submitted template's language can't change")
		}
		if len(locked) > 0 {
			return model.Template{}, model.ErrInvalidTemplate.WithDetails(locked...)
		}
	}
	if err := validate(d, nil, false); err != nil {
		return model.Template{}, err
	}
	components, err := json.Marshal(d.Components)
	if err != nil {
		return model.Template{}, fmt.Errorf("encode components: %w", err)
	}
	n, err := r.UpdateTemplate(ctx, gen.UpdateTemplateParams{
		WorkspaceID: workspaceID, ID: id, Name: d.Name, Language: d.Language, Category: d.Category, Components: components,
	})
	if repository.IsUnique(err, repository.UniqueNameLanguage) {
		return model.Template{}, errNameTaken
	}
	if err != nil {
		return model.Template{}, fmt.Errorf("update template: %w", err)
	}
	if n == 0 {
		return model.Template{}, model.ErrInReview // submitted while we were editing
	}
	return s.load(ctx, r, workspaceID, id)
}
