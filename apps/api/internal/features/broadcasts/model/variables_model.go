package model

import "strings"

// Variable sources: fixed text, or a field of the contact receiving it.
const (
	SourceText      = "text"
	SourceName      = "name"
	SourcePhone     = "phone"
	SourceAttribute = "attribute"
)

// Variable fills one template placeholder. Value is the text, or the
// attribute's key; Fallback is used when the contact's field is empty.
type Variable struct {
	Source   string `json:"source"`
	Value    string `json:"value"`
	Fallback string `json:"fallback,omitempty"`
}

// Variables fill a template: Header for the header's {{1}}, Body[i] for {{i+1}}.
type Variables struct {
	Header *Variable  `json:"header,omitempty"`
	Body   []Variable `json:"body"`
}

// Contact is what a variable can read from the contact it is filled for.
type Contact struct {
	Name, Phone string
	Attributes  map[string]string
}

// Render is v's text for c.
func (v Variable) Render(c Contact) string {
	var s string
	switch v.Source {
	case SourceText:
		return v.Value
	case SourceName:
		s = c.Name
	case SourcePhone:
		s = c.Phone
	case SourceAttribute:
		s = c.Attributes[v.Value]
	}
	if strings.TrimSpace(s) == "" {
		return v.Fallback
	}
	return s
}

// Render fills the header and body for c.
func (vs Variables) Render(c Contact) (header string, body []string) {
	if vs.Header != nil {
		header = vs.Header.Render(c)
	}
	body = make([]string, len(vs.Body))
	for i, v := range vs.Body {
		body[i] = v.Render(c)
	}
	return header, body
}
