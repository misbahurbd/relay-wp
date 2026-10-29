package service

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
)

// table is a parsed import file: unique column names and the non-blank data
// rows, each padded or cut to the header's width.
type table struct {
	header []string
	rows   [][]string
}

// fileType returns the import file type from its name, or an error for anything else.
func fileType(name string) (string, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".csv":
		return model.FileCSV, nil
	case ".xlsx":
		return model.FileXLSX, nil
	default: // .xls (legacy Excel) included
		return "", model.ErrUnsupportedFile
	}
}

// parseFile reads a whole import file. It stops with ErrTooManyRows past the limit.
func parseFile(kind string, data []byte) (table, error) {
	var (
		t   table
		err error
	)
	switch kind {
	case model.FileCSV:
		t, err = parseCSV(data)
	case model.FileXLSX:
		t, err = parseXLSX(data)
	default:
		return table{}, model.ErrUnsupportedFile
	}
	if err != nil {
		return table{}, err
	}
	if len(t.header) == 0 || len(t.rows) == 0 {
		return table{}, model.ErrEmptyFile
	}
	return t, nil
}

// collector builds a table one record at a time.
type collector struct {
	t       table
	started bool
}

// add takes the next record; the first non-blank one is the header.
func (c *collector) add(rec []string) error {
	blank := true
	for i, v := range rec {
		rec[i] = strings.TrimSpace(strings.ToValidUTF8(v, ""))
		if rec[i] != "" {
			blank = false
		}
	}
	if blank {
		return nil
	}
	if !c.started {
		c.started = true
		c.t.header = headerNames(rec)
		return nil
	}
	if len(c.t.rows) >= model.MaxImportRows {
		return model.ErrTooManyRows
	}
	row := make([]string, len(c.t.header))
	copy(row, rec)
	c.t.rows = append(c.t.rows, row)
	return nil
}

func parseCSV(data []byte) (table, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")) // UTF-8 BOM, as Excel writes it
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = sniffDelimiter(data)
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	var c collector
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			return c.t, nil
		}
		if err != nil {
			return table{}, model.ErrUnreadableFile.Wrap(err)
		}
		if err := c.add(rec); err != nil {
			return table{}, err
		}
	}
}

// sniffDelimiter picks ",", ";" or tab, whichever the first line has most of.
// Spreadsheets in many locales save "CSV" with semicolons.
func sniffDelimiter(data []byte) rune {
	line, _, _ := bytes.Cut(data, []byte("\n"))
	best, count := ',', bytes.Count(line, []byte(","))
	for _, d := range []rune{';', '\t'} {
		if n := bytes.Count(line, []byte(string(d))); n > count {
			best, count = d, n
		}
	}
	return best
}

func parseXLSX(data []byte) (table, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return table{}, model.ErrUnreadableFile.Wrap(err)
	}
	defer func() { _ = f.Close() }()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return table{}, model.ErrEmptyFile
	}
	rows, err := f.Rows(sheets[0])
	if err != nil {
		return table{}, model.ErrUnreadableFile.Wrap(err)
	}
	defer func() { _ = rows.Close() }()
	var c collector
	for rows.Next() {
		rec, err := rows.Columns()
		if err != nil {
			return table{}, model.ErrUnreadableFile.Wrap(err)
		}
		if err := c.add(rec); err != nil {
			return table{}, err
		}
	}
	if err := rows.Error(); err != nil {
		return table{}, model.ErrUnreadableFile.Wrap(err)
	}
	return c.t, nil
}

// headerNames names blank headers "Column N" and makes repeated names unique,
// since the mapping refers to columns by name.
func headerNames(rec []string) []string {
	out := make([]string, len(rec))
	seen := map[string]int{}
	for i, name := range rec {
		if name == "" {
			name = "Column " + strconv.Itoa(i+1)
		}
		key := strings.ToLower(name)
		if n := seen[key]; n > 0 {
			name = fmt.Sprintf("%s (%d)", name, n+1)
		}
		seen[key]++
		out[i] = name
	}
	return out
}

// Header words that suggest what a column holds.
var (
	nameHeaders  = []string{"name", "full name", "fullname", "contact name", "customer name", "contact", "customer"}
	phoneHeaders = []string{"phone", "mobile", "whatsapp", "number", "tel", "cell", "msisdn"}
	emailHeaders = []string{"email", "e-mail", "mail"}
	tagHeaders   = []string{"tag", "tags", "label", "labels", "group", "groups", "segment"}
)

// guessColumns suggests a field for each column from its header. Name, phone
// and email are guessed once each; a column with no values is skipped.
func guessColumns(t table) []model.UploadedColumn {
	out := make([]model.UploadedColumn, len(t.header))
	taken := map[string]bool{}
	for i, name := range t.header {
		col := model.UploadedColumn{Name: name, Samples: []string{}}
		for _, row := range t.rows {
			if v := row[i]; v != "" {
				col.Samples = append(col.Samples, v)
				if len(col.Samples) == model.ColumnSamples {
					break
				}
			}
		}
		col.Guess = guessField(name)
		if (col.Guess == model.FieldName || col.Guess == model.FieldPhone || col.Guess == model.FieldEmail) && taken[col.Guess] {
			col.Guess = model.FieldAttribute
		}
		if len(col.Samples) == 0 {
			col.Guess = model.FieldSkip
		}
		taken[col.Guess] = true
		out[i] = col
	}
	return out
}

func guessField(header string) string {
	h := strings.ToLower(strings.TrimSpace(header))
	has := func(words []string) bool {
		for _, w := range words {
			if h == w || strings.HasPrefix(h, w+" ") || strings.HasSuffix(h, " "+w) || strings.Contains(h, " "+w+" ") {
				return true
			}
		}
		return false
	}
	switch {
	case has(emailHeaders):
		return model.FieldEmail
	case has(phoneHeaders):
		return model.FieldPhone
	case has(tagHeaders):
		return model.FieldTag
	case has(nameHeaders), h == "first name":
		return model.FieldName
	case strings.HasPrefix(h, "column "):
		return model.FieldSkip
	default:
		return model.FieldAttribute
	}
}
