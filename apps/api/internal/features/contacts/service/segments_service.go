package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// MaxAudienceSegments is how many segments one broadcast audience may name.
const MaxAudienceSegments = 10

// Segments lists the workspace's segments by name, each with who matches it now.
func (s *Service) Segments(ctx context.Context, workspaceID uuid.UUID) ([]model.Segment, error) {
	repo := repository.NewRepository(s.pool)
	rows, err := repo.ListSegments(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list segments: %w", err)
	}
	out := make([]model.Segment, len(rows))
	for i, r := range rows {
		if out[i], err = toSegment(r.ID, r.Name, r.Description, r.Rules, r.CreatedAt, r.UpdatedAt); err != nil {
			return nil, err
		}
		if out[i].Count, err = repo.CountSegment(ctx, workspaceID, out[i].Rules); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Segment returns one segment with who matches it now.
func (s *Service) Segment(ctx context.Context, workspaceID, id uuid.UUID) (model.Segment, error) {
	repo := repository.NewRepository(s.pool)
	r, err := repo.GetSegment(ctx, gen.GetSegmentParams{WorkspaceID: workspaceID, ID: id})
	if repository.IsNoRows(err) {
		return model.Segment{}, model.ErrSegmentNotFound
	}
	if err != nil {
		return model.Segment{}, fmt.Errorf("get segment: %w", err)
	}
	seg, err := toSegment(r.ID, r.Name, r.Description, r.Rules, r.CreatedAt, r.UpdatedAt)
	if err != nil {
		return model.Segment{}, err
	}
	seg.Count, err = repo.CountSegment(ctx, workspaceID, seg.Rules)
	return seg, err
}

// CreateSegment saves a new segment.
func (s *Service) CreateSegment(ctx context.Context, workspaceID, userID uuid.UUID, in model.SegmentInput) (model.Segment, error) {
	in, raw, err := s.checkSegment(ctx, workspaceID, in)
	if err != nil {
		return model.Segment{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return model.Segment{}, fmt.Errorf("new id: %w", err)
	}
	var createdBy *uuid.UUID
	if userID != uuid.Nil {
		createdBy = &userID
	}
	err = repository.NewRepository(s.pool).InsertSegment(ctx, gen.InsertSegmentParams{
		ID: id, WorkspaceID: workspaceID, Name: in.Name, Description: in.Description, Rules: raw, CreatedBy: createdBy,
	})
	if repository.IsUnique(err, repository.SegmentNameConstraint) {
		return model.Segment{}, model.ErrSegmentNameTaken
	}
	if err != nil {
		return model.Segment{}, fmt.Errorf("insert segment: %w", err)
	}
	return s.Segment(ctx, workspaceID, id)
}

// UpdateSegment replaces a segment's name, description and rules.
func (s *Service) UpdateSegment(ctx context.Context, workspaceID, id uuid.UUID, in model.SegmentInput) (model.Segment, error) {
	in, raw, err := s.checkSegment(ctx, workspaceID, in)
	if err != nil {
		return model.Segment{}, err
	}
	n, err := repository.NewRepository(s.pool).UpdateSegment(ctx, gen.UpdateSegmentParams{
		WorkspaceID: workspaceID, ID: id, Name: in.Name, Description: in.Description, Rules: raw,
	})
	if repository.IsUnique(err, repository.SegmentNameConstraint) {
		return model.Segment{}, model.ErrSegmentNameTaken
	}
	if err != nil {
		return model.Segment{}, fmt.Errorf("update segment: %w", err)
	}
	if n == 0 {
		return model.Segment{}, model.ErrSegmentNotFound
	}
	return s.Segment(ctx, workspaceID, id)
}

// DeleteSegment removes a segment; its contacts stay.
func (s *Service) DeleteSegment(ctx context.Context, workspaceID, id uuid.UUID) error {
	n, err := repository.NewRepository(s.pool).DeleteSegment(ctx, gen.DeleteSegmentParams{WorkspaceID: workspaceID, ID: id})
	if err != nil {
		return fmt.Errorf("delete segment: %w", err)
	}
	if n == 0 {
		return model.ErrSegmentNotFound
	}
	return nil
}

// PreviewSegment is who rules match right now, before they are saved.
func (s *Service) PreviewSegment(ctx context.Context, workspaceID uuid.UUID, rules model.SegmentRules) (model.SegmentPreview, error) {
	if bad := rules.Check(); len(bad) > 0 {
		return model.SegmentPreview{}, model.ErrInvalidSegment.WithDetails(bad...)
	}
	if err := s.checkSegmentTags(ctx, workspaceID, rules); err != nil {
		return model.SegmentPreview{}, err
	}
	repo := repository.NewRepository(s.pool)
	count, err := repo.CountSegment(ctx, workspaceID, rules)
	if err != nil {
		return model.SegmentPreview{}, err
	}
	sample, err := repo.SegmentSample(ctx, workspaceID, rules, model.SegmentSampleSize)
	if err != nil {
		return model.SegmentPreview{}, err
	}
	return model.SegmentPreview{Count: count, Sample: sample}, nil
}

// SegmentMembers is every contact in any of the segments, once, for a
// broadcast's audience. model.ErrSegmentNotFound when one is gone (ADR 0014).
func (s *Service) SegmentMembers(ctx context.Context, workspaceID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error) {
	if len(ids) == 0 {
		return []uuid.UUID{}, nil
	}
	repo := repository.NewRepository(s.pool)
	rows, err := repo.SegmentRules(ctx, gen.SegmentRulesParams{WorkspaceID: workspaceID, Ids: ids})
	if err != nil {
		return nil, fmt.Errorf("segment rules: %w", err)
	}
	if len(rows) != len(dedupeIDs(ids)) {
		return nil, model.ErrSegmentNotFound
	}
	seen := map[uuid.UUID]bool{}
	out := []uuid.UUID{}
	for _, r := range rows {
		var rules model.SegmentRules
		if err := json.Unmarshal(r.Rules, &rules); err != nil {
			return nil, fmt.Errorf("decode segment rules: %w", err)
		}
		members, err := repo.SegmentMembers(ctx, workspaceID, rules)
		if err != nil {
			return nil, err
		}
		for _, id := range members {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out, nil
}

func (s *Service) checkSegment(ctx context.Context, workspaceID uuid.UUID, in model.SegmentInput) (model.SegmentInput, []byte, error) {
	in.Name = strings.Join(strings.Fields(in.Name), " ")
	in.Description = strings.TrimSpace(in.Description)
	var bad []apperr.Detail
	switch n := utf8.RuneCountInString(in.Name); {
	case n == 0:
		bad = append(bad, apperr.Detail{Field: "name", Message: "Name the segment"})
	case n > model.MaxSegmentName:
		bad = append(bad, apperr.Detail{Field: "name", Message: fmt.Sprintf("Use at most %d characters", model.MaxSegmentName)})
	}
	if utf8.RuneCountInString(in.Description) > model.MaxSegmentDescription {
		bad = append(bad, apperr.Detail{Field: "description", Message: fmt.Sprintf("Use at most %d characters", model.MaxSegmentDescription)})
	}
	bad = append(bad, in.Rules.Check()...)
	if len(bad) > 0 {
		return in, nil, model.ErrInvalidSegment.WithDetails(bad...)
	}
	if err := s.checkSegmentTags(ctx, workspaceID, in.Rules); err != nil {
		return in, nil, err
	}
	raw, err := json.Marshal(in.Rules)
	if err != nil {
		return in, nil, fmt.Errorf("encode segment rules: %w", err)
	}
	return in, raw, nil
}

// checkSegmentTags refuses a tag condition naming another workspace's tag, or one that's gone.
func (s *Service) checkSegmentTags(ctx context.Context, workspaceID uuid.UUID, rules model.SegmentRules) error {
	tags, err := s.ListTags(ctx, workspaceID)
	if err != nil {
		return err
	}
	ok := make(map[string]bool, len(tags))
	for _, t := range tags {
		ok[t.ID.String()] = true
	}
	var bad []apperr.Detail
	for gi, g := range rules.Groups {
		for ci, c := range g.Conditions {
			if c.Field == model.SegTag && !ok[strings.TrimSpace(c.Value)] {
				bad = append(bad, apperr.Detail{Field: fmt.Sprintf("rules.groups[%d].conditions[%d]", gi, ci), Message: "This tag no longer exists"})
			}
		}
	}
	if len(bad) > 0 {
		return model.ErrInvalidSegment.WithDetails(bad...)
	}
	return nil
}

func toSegment(id uuid.UUID, name, desc string, raw []byte, created, updated time.Time) (model.Segment, error) {
	seg := model.Segment{ID: id, Name: name, Description: desc, CreatedAt: created, UpdatedAt: updated}
	if err := json.Unmarshal(raw, &seg.Rules); err != nil {
		return seg, fmt.Errorf("decode segment rules: %w", err)
	}
	return seg, nil
}

func dedupeIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
