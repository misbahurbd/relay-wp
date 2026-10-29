package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

const importFailed = "Something went wrong while importing. Try again."

// RunImport works a started import: it upserts the valid rows in batches of
// 500, one transaction per batch, and stores the skipped rows as a CSV. A
// failure the file causes ends the import; any other is retried, and marks the
// import failed on the last attempt.
func (s *Service) RunImport(ctx context.Context, workspaceID, id uuid.UUID, lastAttempt bool) error {
	row, err := s.loadImport(ctx, s.pool, workspaceID, id)
	if errors.Is(err, model.ErrImportNotFound) {
		return nil // the workspace was deleted
	}
	if err != nil {
		return err
	}
	if row.Status != model.ImportImporting {
		return nil
	}
	err = s.runImport(ctx, workspaceID, row)
	if err == nil {
		return nil
	}
	ae, permanent := apperr.As(err)
	if !permanent && !lastAttempt {
		return err
	}
	msg := importFailed
	if permanent {
		msg = ae.Message
	}
	if ferr := repository.NewRepository(s.pool).FinishContactImport(ctx, gen.FinishContactImportParams{
		WorkspaceID: workspaceID, ID: id, Status: model.ImportFailed, Error: &msg,
	}); ferr != nil {
		return errors.Join(err, fmt.Errorf("mark import failed: %w", ferr))
	}
	if permanent {
		return nil
	}
	return err
}

func (s *Service) runImport(ctx context.Context, workspaceID uuid.UUID, row gen.GetContactImportRow) error {
	r := repository.NewRepository(s.pool)
	if err := r.ResetContactImportProgress(ctx, gen.ResetContactImportProgressParams{WorkspaceID: workspaceID, ID: row.ID}); err != nil {
		return fmt.Errorf("reset import: %w", err)
	}
	plan, t, err := s.readImport(ctx, row)
	if err != nil {
		return err
	}
	tags, err := s.importTags(ctx, r, workspaceID, row.TagName, plan)
	if err != nil {
		return err
	}

	var valid, skipped []plannedRow
	for _, p := range plan {
		if p.reason == "" {
			valid = append(valid, p)
		} else {
			skipped = append(skipped, p)
		}
	}
	for start := 0; start < len(valid); start += model.ImportBatchSize {
		batch := valid[start:min(start+model.ImportBatchSize, len(valid))]
		if err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
			return s.importBatch(ctx, repository.NewRepository(tx), workspaceID, row.ID, batch, row.TagName, tags)
		}); err != nil {
			return err
		}
	}

	var skippedKey *string
	if len(skipped) > 0 {
		key, err := s.storeSkipped(ctx, workspaceID, row.ID, t.header, skipped)
		if err != nil {
			return err
		}
		skippedKey = &key
	}
	if err := r.FinishContactImport(ctx, gen.FinishContactImportParams{
		WorkspaceID: workspaceID, ID: row.ID, Status: model.ImportDone,
		Skipped: int32(len(skipped)), SkippedKey: skippedKey, //nolint:gosec // at most MaxImportRows
	}); err != nil {
		return fmt.Errorf("finish import: %w", err)
	}
	return nil
}

// importTags creates the import's tag and every tag named in a tag column, and
// returns their ids by lower-case name.
func (s *Service) importTags(ctx context.Context, r repository.Repository, workspaceID uuid.UUID, tagName *string, plan []plannedRow) (map[string]uuid.UUID, error) {
	names := map[string]string{}
	if tagName != nil {
		names[strings.ToLower(*tagName)] = *tagName
	}
	for _, p := range plan {
		if p.reason != "" {
			continue
		}
		for _, t := range p.tags {
			if _, ok := names[strings.ToLower(t)]; !ok {
				names[strings.ToLower(t)] = t
			}
		}
	}
	out := make(map[string]uuid.UUID, len(names))
	for key, name := range names {
		t, err := upsertTag(ctx, r, workspaceID, name)
		if err != nil {
			return nil, err
		}
		out[key] = t.ID
	}
	return out, nil
}

func (s *Service) importBatch(ctx context.Context, r repository.Repository, workspaceID, importID uuid.UUID,
	batch []plannedRow, tagName *string, tags map[string]uuid.UUID,
) error {
	p := gen.UpsertImportedContactsParams{
		WorkspaceID: workspaceID,
		Ids:         make([]uuid.UUID, len(batch)), Names: make([]string, len(batch)), Phones: make([]string, len(batch)),
		Emails: make([]string, len(batch)), Attributes: make([]string, len(batch)),
	}
	for i, row := range batch {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("contact id: %w", err)
		}
		attrs, err := json.Marshal(row.attrs)
		if err != nil {
			return fmt.Errorf("contact attributes: %w", err)
		}
		p.Ids[i], p.Names[i], p.Phones[i], p.Emails[i], p.Attributes[i] = id, row.name, row.phone, row.email, string(attrs)
	}
	upserted, err := r.UpsertImportedContacts(ctx, p)
	if err != nil {
		return fmt.Errorf("upsert contacts: %w", err)
	}
	byPhone := make(map[string]uuid.UUID, len(upserted))
	var created, updated int32
	for _, u := range upserted {
		byPhone[u.Phone] = u.ID
		if u.Inserted {
			created++
		} else {
			updated++
		}
	}

	var contactIDs, tagIDs []uuid.UUID
	for _, row := range batch {
		contactID, ok := byPhone[row.phone]
		if !ok {
			continue
		}
		names := row.tags
		if tagName != nil {
			names = append([]string{*tagName}, names...)
		}
		for _, name := range names {
			if tagID, ok := tags[strings.ToLower(name)]; ok {
				contactIDs, tagIDs = append(contactIDs, contactID), append(tagIDs, tagID)
			}
		}
	}
	if len(contactIDs) > 0 {
		if _, err := r.AddContactTagLinks(ctx, gen.AddContactTagLinksParams{
			WorkspaceID: workspaceID, ContactIds: contactIDs, TagIds: tagIDs,
		}); err != nil {
			return fmt.Errorf("tag imported contacts: %w", err)
		}
	}
	if err := r.AddContactImportProgress(ctx, gen.AddContactImportProgressParams{
		WorkspaceID: workspaceID, ID: importID, Created: created, Updated: updated,
	}); err != nil {
		return fmt.Errorf("import progress: %w", err)
	}
	return nil
}

// storeSkipped writes the skipped rows, as they were in the file plus the reason, to storage.
func (s *Service) storeSkipped(ctx context.Context, workspaceID, importID uuid.UUID, header []string, rows []plannedRow) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(append(append([]string{"row"}, header...), "error")); err != nil {
		return "", fmt.Errorf("skipped rows: %w", err)
	}
	for _, p := range rows {
		// Cells stay as they were in the file, so the fixed CSV can be imported again.
		rec := make([]string, 0, len(p.cells)+2)
		rec = append(append(rec, strconv.Itoa(p.line)), p.cells...)
		if err := w.Write(append(rec, p.reason)); err != nil {
			return "", fmt.Errorf("skipped rows: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", fmt.Errorf("skipped rows: %w", err)
	}
	key := importKey(workspaceID, importID, "skipped.csv")
	if err := s.store.Put(ctx, key, &buf, int64(buf.Len()), "text/csv"); err != nil {
		return "", fmt.Errorf("store skipped rows: %w", err)
	}
	return key, nil
}
