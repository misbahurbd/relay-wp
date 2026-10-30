package service

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
)

func TestFileType(t *testing.T) {
	for name, want := range map[string]string{"a.csv": model.FileCSV, "B.XLSX": model.FileXLSX} {
		if got, err := fileType(name); err != nil || got != want {
			t.Errorf("fileType(%q) = %q, %v", name, got, err)
		}
	}
	for _, name := range []string{"old.xls", "notes.txt", "noext"} {
		if _, err := fileType(name); !errors.Is(err, model.ErrUnsupportedFile) {
			t.Errorf("fileType(%q) err = %v, want unsupported_file", name, err)
		}
	}
}

func TestParseCSV(t *testing.T) {
	data := "\xef\xbb\xbfName;Phone;;Phone\n" +
		"Daniel;+44 7700 900412;x;1\n" +
		";;;\n" + // blank rows are dropped
		"Ana;447700900413\n" // short rows are padded
	tb, err := parseFile(model.FileCSV, []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Name", "Phone", "Column 3", "Phone (2)"}; strings.Join(tb.header, "|") != strings.Join(want, "|") {
		t.Fatalf("header = %q", tb.header)
	}
	if len(tb.rows) != 2 || tb.rows[1][0] != "Ana" || len(tb.rows[1]) != 4 {
		t.Fatalf("rows = %q", tb.rows)
	}
}

func TestParseRejectsEmptyAndHugeFiles(t *testing.T) {
	if _, err := parseFile(model.FileCSV, []byte("name,phone\n")); !errors.Is(err, model.ErrEmptyFile) {
		t.Errorf("header only: %v", err)
	}
	var b strings.Builder
	b.WriteString("phone\n")
	for i := range model.MaxImportRows + 1 {
		fmt.Fprintf(&b, "+4477009%05d\n", i)
	}
	if _, err := parseFile(model.FileCSV, []byte(b.String())); !errors.Is(err, model.ErrTooManyRows) {
		t.Errorf("50,001 rows: %v", err)
	}
	if _, err := parseFile(model.FileXLSX, []byte("not a zip")); !isCode(err, "unreadable_file") {
		t.Errorf("bad xlsx: %v", err)
	}
}

func xlsxFile(t *testing.T, rows [][]any) []byte {
	t.Helper()
	f := excelize.NewFile()
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow("Sheet1", cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParseXLSXKeepsLongNumbers(t *testing.T) {
	data := xlsxFile(t, [][]any{
		{"Full name", "WhatsApp number", "Email", "City"},
		{"Daniel", 447700900412, "dan@example.com", "Leeds"},
	})
	tb, err := parseFile(model.FileXLSX, data)
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.rows) != 1 {
		t.Fatalf("rows = %q", tb.rows)
	}
	if phone, ok := NormalizePhone(tb.rows[0][1]); !ok || phone != "+447700900412" {
		t.Fatalf("phone cell %q -> %q, %v", tb.rows[0][1], phone, ok)
	}
}

func TestGuessColumns(t *testing.T) {
	tb := table{
		header: []string{"Full name", "Phone number", "E-mail", "Tags", "City", "Mobile", "Column 7"},
		rows: [][]string{
			{"Daniel", "+447700900412", "d@example.com", "VIP", "Leeds", "", ""},
			{"Ana", "+447700900413", "", "", "York", "", ""},
		},
	}
	got := guessColumns(tb)
	want := []string{model.FieldName, model.FieldPhone, model.FieldEmail, model.FieldTag, model.FieldAttribute, model.FieldSkip, model.FieldSkip}
	for i, c := range got {
		if c.Guess != want[i] {
			t.Errorf("column %q guessed %q, want %q", c.Name, c.Guess, want[i])
		}
	}
	if len(got[4].Samples) != 2 || got[4].Samples[1] != "York" {
		t.Errorf("samples = %q", got[4].Samples)
	}
}
