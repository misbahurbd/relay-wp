package service

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

var fileColumns = []string{"Name", "Phone", "Email", "Tags", "City"}

func goodMapping() model.Mapping {
	return model.Mapping{OptInConfirmed: true, Columns: []model.ColumnMapping{
		{Name: "Name", Field: model.FieldName},
		{Name: "Phone", Field: model.FieldPhone},
		{Name: "Email", Field: model.FieldEmail},
		{Name: "Tags", Field: model.FieldTag},
		{Name: "City", Field: model.FieldAttribute},
	}}
}

func TestCheckMapping(t *testing.T) {
	m, err := checkMapping(fileColumns, goodMapping())
	if err != nil {
		t.Fatal(err)
	}
	if m.Columns[4].AttributeKey != "City" {
		t.Errorf("attribute key defaults to the column name, got %q", m.Columns[4].AttributeKey)
	}

	noOptIn := goodMapping()
	noOptIn.OptInConfirmed = false
	if _, err := checkMapping(fileColumns, noOptIn); !errors.Is(err, model.ErrOptInRequired) {
		t.Errorf("unconfirmed opt-in: %v", err)
	}

	for name, change := range map[string]func(*model.Mapping){
		"no phone":       func(m *model.Mapping) { m.Columns[1].Field = model.FieldSkip },
		"two phones":     func(m *model.Mapping) { m.Columns[2].Field = model.FieldPhone },
		"unknown column": func(m *model.Mapping) { m.Columns[0].Name = "Nope" },
		"column twice":   func(m *model.Mapping) { m.Columns[3].Name = "Name" },
		"same attribute": func(m *model.Mapping) {
			m.Columns[0] = model.ColumnMapping{Name: "Name", Field: model.FieldAttribute, AttributeKey: "city"}
		},
	} {
		m := goodMapping()
		m.Columns = append([]model.ColumnMapping(nil), m.Columns...)
		change(&m)
		if _, err := checkMapping(fileColumns, m); !isCode(err, "invalid_mapping") {
			t.Errorf("%s: err = %v, want invalid_mapping", name, err)
		}
	}
}

func isCode(err error, code string) bool {
	ae, ok := apperr.As(err)
	return ok && ae.Code == code
}

func TestPlanRows(t *testing.T) {
	tb := table{header: fileColumns, rows: [][]string{
		{"Daniel", "+44 7700 900412", "dan@example.com", "VIP, Leeds", "Leeds"},
		{"Dan again", "447700900412", "", "", ""}, // same number, other spelling
		{"No country", "07700 900413", "", "", ""},
		{"Bad email", "+447700900414", "not-an-email", "", ""},
		{"No phone", "", "", "", ""},
		{"", "+447700900415", "", "", ""},
	}}
	m, err := checkMapping(fileColumns, goodMapping())
	if err != nil {
		t.Fatal(err)
	}
	plan := planRows(tb, m.Columns)
	want := []string{"", reasonDuplicate, reasonBadPhone, reasonBadEmail, reasonNoPhone, ""}
	for i, p := range plan {
		if p.reason != want[i] {
			t.Errorf("row %d (%s): reason %q, want %q", p.line, p.name, p.reason, want[i])
		}
	}
	first := plan[0]
	if first.phone != "+447700900412" || first.attrs["City"] != "Leeds" || len(first.tags) != 2 || first.tags[1] != "Leeds" {
		t.Errorf("first row = %+v", first)
	}
	if plan[5].reason != "" || plan[5].name != "" {
		t.Errorf("a row without a name still imports: %+v", plan[5])
	}
}

func TestBulkPermissionDependsOnAction(t *testing.T) {
	s := New(nil, nil, nil, nil) // the permission is checked before anything else
	editor := authctx.Access{WorkspaceID: uuid.New()}
	_, err := s.Bulk(t.Context(), editor, model.Bulk{Action: model.BulkDelete, ContactIDs: []uuid.UUID{uuid.New()}})
	if !isCode(err, "permission_denied") {
		t.Fatalf("delete without contacts.delete: %v", err)
	}
	ae, _ := apperr.As(err)
	if len(ae.Details) != 1 || ae.Details[0].Message != "contacts.delete" {
		t.Errorf("details = %+v", ae.Details)
	}
	if BulkPermission(model.BulkAddTag) != authctx.PermContactsEdit || BulkPermission(model.BulkAssign) != authctx.PermContactsEdit {
		t.Error("tagging and assigning need contacts.edit")
	}
}
