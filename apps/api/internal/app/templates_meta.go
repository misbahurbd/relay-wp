package app

import (
	"context"
	"io"

	"github.com/google/uuid"

	templatesmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/model"
	templatesservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/service"
	whatsappmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
	whatsappservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/service"
)

// This file is the only place that knows both templates and whatsapp
// (docs/PHASE-4-CONTACTS-TEMPLATES.md AD-9).

// metaTemplates gives templates' MetaTemplates on top of the whatsapp service,
// converting our template shape to whatsapp's spec and back.
type metaTemplates struct {
	wa *whatsappservice.Service
}

var _ templatesservice.MetaTemplates = metaTemplates{}

// The templates service hears status webhooks directly: its method already has
// the sink's signature, so that direction needs no conversion.
var _ whatsappservice.TemplateStatusSink = (*templatesservice.Service)(nil)

func (m metaTemplates) Submit(ctx context.Context, workspaceID uuid.UUID, t templatesmodel.RemoteTemplate) (string, string, error) {
	return m.wa.SubmitTemplate(ctx, workspaceID, t.MetaID, toTemplateSpec(t))
}

func (m metaTemplates) Delete(ctx context.Context, workspaceID uuid.UUID, metaID, name string) error {
	return m.wa.DeleteTemplate(ctx, workspaceID, metaID, name)
}

func (m metaTemplates) List(ctx context.Context, workspaceID uuid.UUID) ([]templatesmodel.RemoteTemplate, error) {
	list, err := m.wa.ListTemplates(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]templatesmodel.RemoteTemplate, len(list))
	for i, t := range list {
		out[i] = fromMessageTemplate(t)
	}
	return out, nil
}

func (m metaTemplates) UploadSample(ctx context.Context, workspaceID uuid.UUID, contentType string, data io.Reader, size int64) (string, error) {
	return m.wa.UploadTemplateSample(ctx, workspaceID, contentType, data, size)
}

func toTemplateSpec(t templatesmodel.RemoteTemplate) whatsappmodel.TemplateSpec {
	c := t.Components
	spec := whatsappmodel.TemplateSpec{
		Name: t.Name, Language: t.Language, Category: t.Category,
		Header: whatsappmodel.TemplateHeader{
			Format: c.Header.Type, Text: c.Header.Text, Example: c.Header.Example, Handle: t.HeaderHandle,
		},
		Body:    whatsappmodel.TemplateBody{Text: c.Body.Text, Examples: c.Body.Examples},
		Footer:  c.Footer.Text,
		Buttons: make([]whatsappmodel.TemplateButton, len(c.Buttons)),
	}
	for i, b := range c.Buttons {
		spec.Buttons[i] = whatsappmodel.TemplateButton{Type: b.Type, Text: b.Text, Value: b.Value}
	}
	return spec
}

func fromMessageTemplate(t whatsappmodel.MessageTemplate) templatesmodel.RemoteTemplate {
	s := t.Spec
	c := templatesmodel.Components{
		Header:  templatesmodel.Header{Type: s.Header.Format, Text: s.Header.Text, Example: s.Header.Example},
		Body:    templatesmodel.Body{Text: s.Body.Text, Examples: s.Body.Examples},
		Footer:  templatesmodel.Footer{Text: s.Footer},
		Buttons: make([]templatesmodel.Button, len(s.Buttons)),
	}
	for i, b := range s.Buttons {
		c.Buttons[i] = templatesmodel.Button{Type: b.Type, Text: b.Text, Value: b.Value}
	}
	return templatesmodel.RemoteTemplate{
		MetaID: t.ID, Name: s.Name, Language: s.Language, Category: s.Category,
		Status: t.Status, RejectedReason: t.RejectedReason, Components: c,
	}
}
