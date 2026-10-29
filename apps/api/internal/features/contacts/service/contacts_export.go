package service

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/repository"
)

// exportPage is how many contacts the export reads at a time.
const exportPage = 1000

// exportColumns are the fixed columns of the export; one column per custom field follows.
var exportColumns = []string{"name", "phone", "email", "opt_in", "tags", "source", "language", "created_at"}

// Export prepares a CSV of the contacts matching f (AD-4). The returned function
// writes it; it is called once the response has started, so errors before that
// (a database that is down) still get a proper status.
func (s *Service) Export(ctx context.Context, workspaceID uuid.UUID, f model.Filter) (func(w io.Writer) error, error) {
	p := s.toFilter(f)
	r := repository.NewRepository(s.pool)
	keys, err := r.ContactAttributeKeys(ctx, gen.ContactAttributeKeysParams{
		WorkspaceID: workspaceID, OptIn: p.optIn, TagID: p.tagID, Query: p.query, PhoneQuery: p.phoneQuery,
		MessageAfter: p.messageAfter, MessageBefore: p.messageBefore,
	})
	if err != nil {
		return nil, fmt.Errorf("export attribute keys: %w", err)
	}
	return func(w io.Writer) error {
		cw := csv.NewWriter(w)
		header := append(append([]string{}, exportColumns...), keys...)
		for i := len(exportColumns); i < len(header); i++ {
			header[i] = csvSafe(header[i])
		}
		if err := cw.Write(header); err != nil {
			return fmt.Errorf("export header: %w", err)
		}
		var after *uuid.UUID
		for {
			rows, err := r.ExportContacts(ctx, gen.ExportContactsParams{
				WorkspaceID: workspaceID, OptIn: p.optIn, TagID: p.tagID, Query: p.query, PhoneQuery: p.phoneQuery,
				MessageAfter: p.messageAfter, MessageBefore: p.messageBefore, AfterID: after, PageSize: exportPage,
			})
			if err != nil {
				return fmt.Errorf("export contacts: %w", err)
			}
			for _, c := range rows {
				attrs := map[string]string{}
				if err := json.Unmarshal(c.Attributes, &attrs); err != nil {
					return fmt.Errorf("export attributes: %w", err)
				}
				rec := []string{
					csvSafe(c.Name), c.Phone, csvSafe(deref(c.Email)), c.OptIn, csvSafe(c.Tags), c.Source,
					csvSafe(deref(c.Language)), c.CreatedAt.UTC().Format(time.RFC3339),
				}
				for _, k := range keys {
					rec = append(rec, csvSafe(attrs[k]))
				}
				if err := cw.Write(rec); err != nil {
					return fmt.Errorf("export row: %w", err)
				}
			}
			cw.Flush()
			if err := cw.Error(); err != nil {
				return fmt.Errorf("export flush: %w", err)
			}
			if len(rows) < exportPage {
				return nil
			}
			last := rows[len(rows)-1].ID
			after = &last
		}
	}, nil
}

// csvSafe stops a spreadsheet from running a cell as a formula. Phones are
// written as they are: E.164 always starts with "+" and is only digits.
func csvSafe(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}
