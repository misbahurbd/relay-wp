package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
)

// Meta's message template wire format (Graph API /message_templates).

type metaTemplate struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Language       string          `json:"language"`
	Status         string          `json:"status"`
	Category       string          `json:"category"`
	RejectedReason string          `json:"rejected_reason"`
	Components     []metaComponent `json:"components"`
}

type metaComponent struct {
	Type    string       `json:"type"`
	Format  string       `json:"format,omitempty"`
	Text    string       `json:"text,omitempty"`
	Example *metaExample `json:"example,omitempty"`
	Buttons []metaButton `json:"buttons,omitempty"`
}

type metaExample struct {
	HeaderText   []string   `json:"header_text,omitempty"`
	HeaderHandle []string   `json:"header_handle,omitempty"`
	BodyText     [][]string `json:"body_text,omitempty"`
}

type metaButton struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	URL         string `json:"url,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
}

// maxTemplatePages bounds a template listing (100 per page).
const maxTemplatePages = 50

// CreateTemplate submits a new template for review. It returns Meta's ID and status.
func (g *Graph) CreateTemplate(ctx context.Context, token, wabaID string, spec model.TemplateSpec) (id, status string, err error) {
	body := map[string]any{
		"name":             spec.Name,
		"language":         spec.Language,
		"category":         strings.ToLower(spec.Category),
		"parameter_format": "positional",
		"components":       metaComponents(spec),
	}
	var out struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := g.call(ctx, http.MethodPost, "/"+url.PathEscape(wabaID)+"/message_templates", token, nil, body, &out); err != nil {
		return "", "", err
	}
	if out.ID == "" {
		return "", "", errors.New("graph: no template id in create response")
	}
	return out.ID, out.Status, nil
}

// EditTemplate replaces a submitted template's components; Meta reviews it again.
func (g *Graph) EditTemplate(ctx context.Context, token, templateID string, spec model.TemplateSpec) error {
	return g.call(ctx, http.MethodPost, "/"+url.PathEscape(templateID), token, nil,
		map[string]any{"components": metaComponents(spec)}, nil)
}

// DeleteTemplate deletes one template (by ID and name) from the WABA.
func (g *Graph) DeleteTemplate(ctx context.Context, token, wabaID, templateID, name string) error {
	return g.call(ctx, http.MethodDelete, "/"+url.PathEscape(wabaID)+"/message_templates", token,
		url.Values{"hsm_id": {templateID}, "name": {name}}, nil, nil)
}

// ListTemplates returns every template on the WABA, following Meta's paging.
func (g *Graph) ListTemplates(ctx context.Context, token, wabaID string) ([]model.MessageTemplate, error) {
	var out []model.MessageTemplate
	after := ""
	for range maxTemplatePages {
		q := url.Values{"fields": {"id,name,language,status,category,components,rejected_reason"}, "limit": {"100"}}
		if after != "" {
			q.Set("after", after)
		}
		var page struct {
			Data   []metaTemplate `json:"data"`
			Paging struct {
				Cursors struct {
					After string `json:"after"`
				} `json:"cursors"`
				Next string `json:"next"`
			} `json:"paging"`
		}
		if err := g.call(ctx, http.MethodGet, "/"+url.PathEscape(wabaID)+"/message_templates", token, q, nil, &page); err != nil {
			return nil, err
		}
		for _, t := range page.Data {
			if mt, ok := fromMetaTemplate(t); ok {
				out = append(out, mt)
			}
		}
		if page.Paging.Next == "" || page.Paging.Cursors.After == "" {
			return out, nil
		}
		after = page.Paging.Cursors.After
	}
	return out, nil
}

// UploadSample sends a header sample through Meta's Resumable Upload API and
// returns the file handle a template's example.header_handle needs.
func (g *Graph) UploadSample(ctx context.Context, token, contentType string, data io.Reader, size int64) (string, error) {
	var session struct {
		ID string `json:"id"`
	}
	q := url.Values{
		"file_name":   {"header-sample" + sampleExt(contentType)},
		"file_length": {strconv.FormatInt(size, 10)},
		"file_type":   {contentType},
	}
	if err := g.call(ctx, http.MethodPost, "/"+url.PathEscape(g.appID)+"/uploads", token, q, nil, &session); err != nil {
		return "", err
	}
	if !strings.HasPrefix(session.ID, "upload:") {
		return "", errors.New("graph: no upload session id")
	}
	// The session ID ("upload:…", sometimes with "?sig=…") is the path as given.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.base+"/"+session.ID, data)
	if err != nil {
		return "", fmt.Errorf("graph upload request: %w", err)
	}
	req.ContentLength = size
	req.Header.Set("Authorization", "OAuth "+token)
	req.Header.Set("file_offset", "0")
	// Large samples need longer than the client's default timeout; the transport is shared.
	client := *g.http
	client.Timeout = 5 * time.Minute
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("graph upload: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	var out struct {
		H string `json:"h"`
	}
	if err := decodeGraph(res, "upload", &out); err != nil {
		return "", err
	}
	if out.H == "" {
		return "", errors.New("graph: no handle in upload response")
	}
	return out.H, nil
}

func sampleExt(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "video/mp4":
		return ".mp4"
	case "application/pdf":
		return ".pdf"
	}
	return ""
}

// metaComponents converts a spec to Meta's components array.
func metaComponents(spec model.TemplateSpec) []metaComponent {
	var out []metaComponent
	switch h := spec.Header; h.Format {
	case "text":
		c := metaComponent{Type: "HEADER", Format: "TEXT", Text: h.Text}
		if strings.Contains(h.Text, "{{1}}") && h.Example != "" {
			c.Example = &metaExample{HeaderText: []string{h.Example}}
		}
		out = append(out, c)
	case "image", "video", "document":
		c := metaComponent{Type: "HEADER", Format: strings.ToUpper(h.Format)}
		if h.Handle != "" {
			c.Example = &metaExample{HeaderHandle: []string{h.Handle}}
		}
		out = append(out, c)
	}
	body := metaComponent{Type: "BODY", Text: spec.Body.Text}
	if len(spec.Body.Examples) > 0 {
		body.Example = &metaExample{BodyText: [][]string{spec.Body.Examples}}
	}
	out = append(out, body)
	if spec.Footer != "" {
		out = append(out, metaComponent{Type: "FOOTER", Text: spec.Footer})
	}
	if len(spec.Buttons) > 0 {
		buttons := make([]metaButton, 0, len(spec.Buttons))
		for _, b := range spec.Buttons {
			switch b.Type {
			case "quick_reply":
				buttons = append(buttons, metaButton{Type: "QUICK_REPLY", Text: b.Text})
			case "url":
				buttons = append(buttons, metaButton{Type: "URL", Text: b.Text, URL: b.Value})
			case "phone":
				buttons = append(buttons, metaButton{Type: "PHONE_NUMBER", Text: b.Text, PhoneNumber: b.Value})
			}
		}
		out = append(out, metaComponent{Type: "BUTTONS", Buttons: buttons})
	}
	return out
}

// fromMetaTemplate converts a listed template. Templates in states we don't
// track (deleted, pending deletion, …) are skipped.
func fromMetaTemplate(t metaTemplate) (model.MessageTemplate, bool) {
	status := templateStatus(t.Status)
	if status == "" || t.ID == "" {
		return model.MessageTemplate{}, false
	}
	spec := model.TemplateSpec{
		Name: t.Name, Language: t.Language, Category: strings.ToLower(t.Category),
		Header: model.TemplateHeader{Format: "none"},
	}
	for _, c := range t.Components {
		switch strings.ToUpper(c.Type) {
		case "HEADER":
			switch f := strings.ToLower(c.Format); f {
			case "text":
				spec.Header = model.TemplateHeader{Format: f, Text: c.Text}
				if c.Example != nil && len(c.Example.HeaderText) > 0 {
					spec.Header.Example = c.Example.HeaderText[0]
				}
			case "image", "video", "document":
				spec.Header = model.TemplateHeader{Format: f}
			}
		case "BODY":
			spec.Body.Text = c.Text
			if c.Example != nil && len(c.Example.BodyText) > 0 {
				spec.Body.Examples = c.Example.BodyText[0]
			}
		case "FOOTER":
			spec.Footer = c.Text
		case "BUTTONS":
			for _, b := range c.Buttons {
				switch strings.ToUpper(b.Type) {
				case "QUICK_REPLY":
					spec.Buttons = append(spec.Buttons, model.TemplateButton{Type: "quick_reply", Text: b.Text})
				case "URL":
					spec.Buttons = append(spec.Buttons, model.TemplateButton{Type: "url", Text: b.Text, Value: b.URL})
				case "PHONE_NUMBER":
					spec.Buttons = append(spec.Buttons, model.TemplateButton{Type: "phone", Text: b.Text, Value: b.PhoneNumber})
				}
			}
		}
	}
	return model.MessageTemplate{ID: t.ID, Status: status, RejectedReason: templateReason(t.RejectedReason), Spec: spec}, true
}

// templateStatus maps Meta's template states onto ours; "" means not tracked.
func templateStatus(meta string) string {
	switch strings.ToUpper(meta) {
	case "APPROVED", "REINSTATED":
		return model.TemplateApproved
	case "REJECTED":
		return model.TemplateRejected
	case "PAUSED":
		return model.TemplatePaused
	case "DISABLED":
		return model.TemplateDisabled
	case "PENDING":
		return model.TemplatePending
	}
	return ""
}

// templateReason drops Meta's "NONE" placeholder.
func templateReason(meta string) string {
	if strings.EqualFold(meta, "NONE") {
		return ""
	}
	return meta
}
