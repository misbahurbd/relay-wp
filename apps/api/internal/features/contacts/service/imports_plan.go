package service

import (
	"fmt"
	"net/mail"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// Why a row is skipped. Written to the skipped-rows CSV.
const (
	reasonNoPhone   = "missing phone"
	reasonBadPhone  = "invalid phone (include the country code)"
	reasonBadEmail  = "invalid email"
	reasonDuplicate = "duplicate phone in file"
)

// plannedRow is one data row after the mapping. reason is set when it can't be imported.
type plannedRow struct {
	line   int // 1-based data row
	cells  []string
	name   string
	phone  string // E.164 when valid, else as written
	email  string
	attrs  map[string]string
	tags   []string
	reason string
}

// checkMapping validates the mapping step against the file's columns and returns
// the mapping with every attribute key filled in.
func checkMapping(columns []string, m model.Mapping) (model.Mapping, error) {
	if !m.OptInConfirmed {
		return m, model.ErrOptInRequired
	}
	var details []apperr.Detail
	bad := func(i int, msg string) {
		details = append(details, apperr.Detail{Field: fmt.Sprintf("body.columns[%d]", i), Message: msg})
	}
	seen, counts, keys := map[string]bool{}, map[string]int{}, map[string]bool{}
	out := m
	out.Columns = make([]model.ColumnMapping, len(m.Columns))
	for i, c := range m.Columns {
		c.AttributeKey = strings.TrimSpace(c.AttributeKey)
		switch {
		case !slices.Contains(columns, c.Name):
			bad(i, "The file has no column "+c.Name)
		case seen[c.Name]:
			bad(i, "Column "+c.Name+" is mapped twice")
		case !slices.Contains(model.Fields, c.Field):
			bad(i, "Unknown field "+c.Field)
		}
		seen[c.Name] = true
		counts[c.Field]++
		if c.Field == model.FieldAttribute {
			if c.AttributeKey == "" {
				c.AttributeKey = c.Name
			}
			if utf8.RuneCountInString(c.AttributeKey) > model.MaxAttributeKey {
				bad(i, fmt.Sprintf("Custom field names are at most %d characters", model.MaxAttributeKey))
			}
			if keys[strings.ToLower(c.AttributeKey)] {
				bad(i, "Two columns go to the custom field "+c.AttributeKey)
			}
			keys[strings.ToLower(c.AttributeKey)] = true
		} else {
			c.AttributeKey = ""
		}
		out.Columns[i] = c
	}
	if counts[model.FieldPhone] != 1 {
		details = append(details, apperr.Detail{Field: "body.columns", Message: "Map exactly one column to phone"})
	}
	for _, f := range []string{model.FieldName, model.FieldEmail} {
		if counts[f] > 1 {
			details = append(details, apperr.Detail{Field: "body.columns", Message: "Map at most one column to " + f})
		}
	}
	if m.TagName != nil {
		name := strings.Join(strings.Fields(*m.TagName), " ")
		switch {
		case name == "":
			out.TagName = nil
		case utf8.RuneCountInString(name) > model.MaxTagLen:
			details = append(details, apperr.Detail{
				Field: "body.tag_name", Message: fmt.Sprintf("At most %d characters", model.MaxTagLen),
			})
		default:
			out.TagName = &name
		}
	}
	if len(details) > 0 {
		return out, model.ErrInvalidMapping.WithDetails(details...)
	}
	return out, nil
}

// planRows applies the mapping to every row. Preview and the worker both use
// it, so the preview's counts match the result. The second row with a phone
// already seen in the file is skipped.
func planRows(t table, columns []model.ColumnMapping) []plannedRow {
	index := make(map[string]int, len(t.header))
	for i, h := range t.header {
		index[h] = i
	}
	seen := make(map[string]bool, len(t.rows))
	out := make([]plannedRow, len(t.rows))
	for n, cells := range t.rows {
		p := plannedRow{line: n + 1, cells: cells, attrs: map[string]string{}}
		for _, c := range columns {
			i, ok := index[c.Name]
			if !ok {
				continue
			}
			v := cells[i]
			switch c.Field {
			case model.FieldName:
				p.name = truncate(v, model.MaxNameLen)
			case model.FieldPhone:
				p.phone = v
			case model.FieldEmail:
				p.email = v
			case model.FieldTag:
				p.tags = append(p.tags, splitTags(v)...)
			case model.FieldAttribute:
				if v != "" {
					p.attrs[c.AttributeKey] = truncate(v, model.MaxAttributeVal)
				}
			}
		}
		switch phone, ok := NormalizePhone(p.phone); {
		case p.phone == "":
			p.reason = reasonNoPhone
		case !ok:
			p.reason = reasonBadPhone
		case p.email != "" && !validEmail(p.email):
			p.phone, p.reason = phone, reasonBadEmail
		case seen[phone]:
			p.phone, p.reason = phone, reasonDuplicate
		default:
			p.phone = phone
			seen[phone] = true
		}
		out[n] = p
	}
	return out
}

func validEmail(e string) bool {
	addr, err := mail.ParseAddress(e)
	return err == nil && addr.Address == e && len(e) <= 254
}

// splitTags reads "VIP, Leeds; 2024" as three tags.
func splitTags(v string) []string {
	var out []string
	for _, t := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' || r == '|' }) {
		if t = strings.Join(strings.Fields(t), " "); t != "" && utf8.RuneCountInString(t) <= model.MaxTagLen {
			out = append(out, t)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
