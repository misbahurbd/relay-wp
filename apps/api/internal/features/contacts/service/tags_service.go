package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// ListTags returns the workspace's tags by name, with how many contacts carry each.
func (s *Service) ListTags(ctx context.Context, workspaceID uuid.UUID) ([]model.Tag, error) {
	rows, err := repository.NewRepository(s.pool).ListContactTags(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	out := make([]model.Tag, len(rows))
	for i, t := range rows {
		out[i] = model.Tag{ID: t.ID, Name: t.Name, ContactCount: t.ContactCount}
	}
	return out, nil
}

// CreateTag returns the tag with this name, creating it when there is none.
// created is false when the name was already taken (case-insensitive).
func (s *Service) CreateTag(ctx context.Context, workspaceID uuid.UUID, name string) (model.Tag, bool, error) {
	name, err := cleanTagName(name)
	if err != nil {
		return model.Tag{}, false, err
	}
	t, err := upsertTag(ctx, repository.NewRepository(s.pool), workspaceID, name)
	if err != nil {
		return model.Tag{}, false, err
	}
	return model.Tag{ID: t.ID, Name: t.Name}, t.Created, nil
}

func cleanTagName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" || utf8.RuneCountInString(name) > model.MaxTagLen {
		return "", model.ErrInvalidInput.WithDetails(apperr.Detail{
			Field: "body.name", Message: fmt.Sprintf("Use 1 to %d characters", model.MaxTagLen),
		})
	}
	return name, nil
}

func upsertTag(ctx context.Context, r repository.Repository, workspaceID uuid.UUID, name string) (gen.UpsertContactTagRow, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return gen.UpsertContactTagRow{}, fmt.Errorf("tag id: %w", err)
	}
	t, err := r.UpsertContactTag(ctx, gen.UpsertContactTagParams{ID: id, WorkspaceID: workspaceID, Name: name})
	if err != nil {
		return gen.UpsertContactTagRow{}, fmt.Errorf("upsert tag: %w", err)
	}
	return t, nil
}
