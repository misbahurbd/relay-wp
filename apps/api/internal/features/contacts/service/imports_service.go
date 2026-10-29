package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/storage"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

var contentTypes = map[string]string{
	model.FileCSV:  "text/csv",
	model.FileXLSX: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
}

func importKey(workspaceID, importID uuid.UUID, name string) string {
	return fmt.Sprintf("contact-imports/%s/%s/%s", workspaceID, importID, name)
}

// Upload stores an import file and reads its columns (AD-5 step 1). size is
// the size the client declared; the file is read to at most 10 MB either way.
func (s *Service) Upload(ctx context.Context, actor authctx.Access, fileName string, size int64, body io.Reader) (model.Upload, error) {
	fileName = truncate(filepath.Base(fileName), 255)
	kind, err := fileType(fileName)
	if err != nil {
		return model.Upload{}, err
	}
	if size > model.MaxImportBytes {
		return model.Upload{}, model.ErrFileTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(body, model.MaxImportBytes+1))
	if err != nil {
		return model.Upload{}, fmt.Errorf("read upload: %w", err)
	}
	if len(data) > model.MaxImportBytes {
		return model.Upload{}, model.ErrFileTooLarge
	}
	t, err := parseFile(kind, data)
	if err != nil {
		return model.Upload{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return model.Upload{}, fmt.Errorf("import id: %w", err)
	}
	key := importKey(actor.WorkspaceID, id, "source."+kind)
	if err := s.store.Put(ctx, key, bytes.NewReader(data), int64(len(data)), contentTypes[kind]); err != nil {
		return model.Upload{}, fmt.Errorf("store import file: %w", err)
	}
	columns, err := json.Marshal(t.header)
	if err != nil {
		return model.Upload{}, fmt.Errorf("import columns: %w", err)
	}
	userID := actor.UserID
	if err := repository.NewRepository(s.pool).CreateContactImport(ctx, gen.CreateContactImportParams{
		ID: id, WorkspaceID: actor.WorkspaceID, CreatedBy: &userID, FileName: fileName, FileType: kind,
		FileKey: key, RowCount: int32(len(t.rows)), Columns: columns, //nolint:gosec // at most MaxImportRows
	}); err != nil {
		return model.Upload{}, fmt.Errorf("create import: %w", err)
	}
	return model.Upload{ID: id, FileName: fileName, RowCount: len(t.rows), Columns: guessColumns(t)}, nil
}

func (s *Service) loadImport(ctx context.Context, db database.DBTX, workspaceID, id uuid.UUID) (gen.GetContactImportRow, error) {
	row, err := repository.NewRepository(db).GetContactImport(ctx, gen.GetContactImportParams{WorkspaceID: workspaceID, ID: id})
	if repository.IsNoRows(err) {
		return row, model.ErrImportNotFound
	}
	if err != nil {
		return row, fmt.Errorf("get import: %w", err)
	}
	return row, nil
}

func toImport(row gen.GetContactImportRow) model.Import {
	return model.Import{
		ID: row.ID, FileName: row.FileName, RowCount: int(row.RowCount), Status: row.Status,
		Created: int(row.CreatedCount), Updated: int(row.UpdatedCount), Skipped: int(row.SkippedCount),
		Error: row.Error, HasSkippedFile: row.SkippedKey != nil, CreatedAt: row.CreatedAt,
	}
}

// GetImport returns an import's progress (AD-5 step 5).
func (s *Service) GetImport(ctx context.Context, workspaceID, id uuid.UUID) (model.Import, error) {
	row, err := s.loadImport(ctx, s.pool, workspaceID, id)
	if err != nil {
		return model.Import{}, err
	}
	return toImport(row), nil
}

// statusError says why an import can't take the next step.
func statusError(status string) error {
	if status == model.ImportUploaded {
		return model.ErrImportNotMapped
	}
	return model.ErrImportStarted
}

// SetMapping saves what each column becomes (AD-5 step 2). It can change until the import starts.
func (s *Service) SetMapping(ctx context.Context, workspaceID, id uuid.UUID, m model.Mapping) (model.Import, error) {
	row, err := s.loadImport(ctx, s.pool, workspaceID, id)
	if err != nil {
		return model.Import{}, err
	}
	if row.Status != model.ImportUploaded && row.Status != model.ImportMapped {
		return model.Import{}, model.ErrImportStarted
	}
	var columns []string
	if err := json.Unmarshal(row.Columns, &columns); err != nil {
		return model.Import{}, fmt.Errorf("import columns: %w", err)
	}
	m, err = checkMapping(columns, m)
	if err != nil {
		return model.Import{}, err
	}
	mapping, err := json.Marshal(m.Columns)
	if err != nil {
		return model.Import{}, fmt.Errorf("import mapping: %w", err)
	}
	n, err := repository.NewRepository(s.pool).SetContactImportMapping(ctx, gen.SetContactImportMappingParams{
		WorkspaceID: workspaceID, ID: id, Mapping: mapping, TagName: m.TagName,
	})
	if err != nil {
		return model.Import{}, fmt.Errorf("save mapping: %w", err)
	}
	if n == 0 {
		return model.Import{}, model.ErrImportStarted
	}
	return s.GetImport(ctx, workspaceID, id)
}

// readImport loads the stored file and the mapping, and plans every row.
func (s *Service) readImport(ctx context.Context, row gen.GetContactImportRow) ([]plannedRow, table, error) {
	var columns []model.ColumnMapping
	if err := json.Unmarshal(row.Mapping, &columns); err != nil {
		return nil, table{}, fmt.Errorf("import mapping: %w", err)
	}
	f, err := s.store.Get(ctx, row.FileKey)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, table{}, model.ErrImportFileExpired
	}
	if err != nil {
		return nil, table{}, fmt.Errorf("read import file: %w", err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, model.MaxImportBytes+1))
	if err != nil {
		return nil, table{}, fmt.Errorf("read import file: %w", err)
	}
	t, err := parseFile(row.FileType, data)
	if err != nil {
		return nil, table{}, err
	}
	return planRows(t, columns), t, nil
}

// existingPhones returns the opt-in state of each phone that is already a
// contact, one query per 1,000 phones.
func (s *Service) existingPhones(ctx context.Context, workspaceID uuid.UUID, rows []plannedRow) (map[string]string, error) {
	var phones []string
	for _, p := range rows {
		if p.reason == "" {
			phones = append(phones, p.phone)
		}
	}
	out := make(map[string]string, len(phones))
	r := repository.NewRepository(s.pool)
	for start := 0; start < len(phones); start += model.PhoneLookupSize {
		chunk := phones[start:min(start+model.PhoneLookupSize, len(phones))]
		found, err := r.ExistingContactPhones(ctx, gen.ExistingContactPhonesParams{WorkspaceID: workspaceID, Phones: chunk})
		if err != nil {
			return nil, fmt.Errorf("existing phones: %w", err)
		}
		for _, f := range found {
			out[f.Phone] = f.OptIn
		}
	}
	return out, nil
}

// Preview says what starting the import would do, computed from the whole file (AD-5 step 3).
func (s *Service) Preview(ctx context.Context, workspaceID, id uuid.UUID) (model.Preview, error) {
	row, err := s.loadImport(ctx, s.pool, workspaceID, id)
	if err != nil {
		return model.Preview{}, err
	}
	if row.Status != model.ImportMapped {
		return model.Preview{}, statusError(row.Status)
	}
	plan, _, err := s.readImport(ctx, row)
	if err != nil {
		return model.Preview{}, err
	}
	existing, err := s.existingPhones(ctx, workspaceID, plan)
	if err != nil {
		return model.Preview{}, err
	}
	out := model.Preview{RowsFound: len(plan), Sample: []model.PreviewRow{}}
	for _, p := range plan {
		status := model.RowNew
		switch _, found := existing[p.phone]; {
		case p.reason != "":
			status = model.RowInvalid
			out.Invalid++
		case found:
			status = model.RowUpdate
			out.Updates++
		default:
			out.Ready++
		}
		if len(out.Sample) < model.PreviewSampleSize {
			out.Sample = append(out.Sample, model.PreviewRow{
				Row: p.line, Name: p.name, Phone: p.phone, Email: p.email, Status: status, Reason: p.reason,
			})
		}
	}
	return out, nil
}

// Start queues the import (AD-5 step 4).
func (s *Service) Start(ctx context.Context, workspaceID, id uuid.UUID) (model.Import, error) {
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		n, err := repository.NewRepository(tx).StartContactImport(ctx, gen.StartContactImportParams{WorkspaceID: workspaceID, ID: id})
		if err != nil {
			return fmt.Errorf("start import: %w", err)
		}
		if n == 0 {
			row, err := s.loadImport(ctx, tx, workspaceID, id)
			if err != nil {
				return err
			}
			return statusError(row.Status)
		}
		if _, err := s.queue.InsertTx(ctx, tx, model.ImportArgs{WorkspaceID: workspaceID, ImportID: id}, nil); err != nil {
			return fmt.Errorf("enqueue import: %w", err)
		}
		return nil
	})
	if err != nil {
		return model.Import{}, err
	}
	return s.GetImport(ctx, workspaceID, id)
}

// SkippedRows opens the CSV of rows the import skipped (AD-5 step 6).
func (s *Service) SkippedRows(ctx context.Context, workspaceID, id uuid.UUID) (io.ReadCloser, error) {
	row, err := s.loadImport(ctx, s.pool, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if row.SkippedKey == nil {
		return nil, model.ErrNoSkippedRows
	}
	f, err := s.store.Get(ctx, *row.SkippedKey)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, model.ErrNoSkippedRows
	}
	if err != nil {
		return nil, fmt.Errorf("read skipped rows: %w", err)
	}
	return f, nil
}
