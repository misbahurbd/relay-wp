package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// TemplateStatusSink hears Meta's review results for message templates.
// Implemented by features/templates.
type TemplateStatusSink interface {
	TemplateStatusChanged(ctx context.Context, workspaceID uuid.UUID, metaTemplateID, status, reason string) error
}

// SetTemplateStatusSink sets who hears template status webhooks. internal/app
// calls it once at startup, after both features are built.
func (s *Service) SetTemplateStatusSink(sink TemplateStatusSink) { s.templates = sink }

// activeAccount returns the workspace's active account, its WABA and token.
// Template calls use it: a workspace has one active account today.
func (s *Service) activeAccount(ctx context.Context, workspaceID uuid.UUID) (accountID uuid.UUID, waba, token string, err error) {
	if !s.cfg.Enabled {
		return uuid.Nil, "", "", model.ErrNoAccount
	}
	accountID, err = repository.NewRepository(s.pool).GetActiveAccountID(ctx, workspaceID)
	if repository.IsNoRows(err) {
		return uuid.Nil, "", "", model.ErrNoAccount
	}
	if err != nil {
		return uuid.Nil, "", "", fmt.Errorf("active account: %w", err)
	}
	waba, token, err = s.token(ctx, workspaceID, accountID)
	if err != nil {
		return uuid.Nil, "", "", err
	}
	return accountID, waba, token, nil
}

// templateErr turns a Graph failure on a template call into an API error. A
// revoked token disconnects the account.
func (s *Service) templateErr(ctx context.Context, workspaceID, accountID uuid.UUID, err error) error {
	if isAuthFailure(err) {
		if derr := s.disconnect(ctx, workspaceID, accountID); derr != nil {
			return derr
		}
		return model.ErrDisconnected
	}
	if g, ok := asGraphError(err); ok && g.Status < 500 {
		msg := g.UserMessage
		if msg == "" {
			msg = g.Message
		}
		return model.ErrTemplateRefused.WithDetails(apperr.Detail{Message: msg}).Wrap(err)
	}
	return model.ErrMetaUnavailable.Wrap(err)
}

// SubmitTemplate sends a template to Meta for review on the workspace's account:
// a create, or an edit of metaID when set. It returns Meta's ID and the status.
func (s *Service) SubmitTemplate(ctx context.Context, workspaceID uuid.UUID, metaID string, spec model.TemplateSpec) (id, status string, err error) {
	accountID, waba, token, err := s.activeAccount(ctx, workspaceID)
	if err != nil {
		return "", "", err
	}
	if metaID != "" {
		if err := s.graph.EditTemplate(ctx, token, metaID, spec); err != nil {
			return "", "", s.templateErr(ctx, workspaceID, accountID, err)
		}
		return metaID, model.TemplatePending, nil
	}
	id, metaStatus, err := s.graph.CreateTemplate(ctx, token, waba, spec)
	if err != nil {
		return "", "", s.templateErr(ctx, workspaceID, accountID, err)
	}
	status = templateStatus(metaStatus)
	if status == "" {
		status = model.TemplatePending
	}
	return id, status, nil
}

// DeleteTemplate deletes a template from the workspace's account.
func (s *Service) DeleteTemplate(ctx context.Context, workspaceID uuid.UUID, metaID, name string) error {
	accountID, waba, token, err := s.activeAccount(ctx, workspaceID)
	if err != nil {
		return err
	}
	if err := s.graph.DeleteTemplate(ctx, token, waba, metaID, name); err != nil {
		if g, ok := asGraphError(err); ok && g.Status == 404 {
			return nil // already gone
		}
		return s.templateErr(ctx, workspaceID, accountID, err)
	}
	return nil
}

// ListTemplates returns every template on the workspace's account.
func (s *Service) ListTemplates(ctx context.Context, workspaceID uuid.UUID) ([]model.MessageTemplate, error) {
	accountID, waba, token, err := s.activeAccount(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out, err := s.graph.ListTemplates(ctx, token, waba)
	if err != nil {
		return nil, s.templateErr(ctx, workspaceID, accountID, err)
	}
	return out, nil
}

// UploadTemplateSample uploads a media header sample and returns its handle.
func (s *Service) UploadTemplateSample(ctx context.Context, workspaceID uuid.UUID, contentType string, data io.Reader, size int64) (string, error) {
	accountID, _, token, err := s.activeAccount(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	h, err := s.graph.UploadSample(ctx, token, contentType, data, size)
	if err != nil {
		return "", s.templateErr(ctx, workspaceID, accountID, err)
	}
	return h, nil
}

// onTemplateStatus passes Meta's template review result to the templates feature.
func (s *Service) onTemplateStatus(ctx context.Context, acc account, value json.RawMessage) error {
	if s.templates == nil {
		return nil
	}
	var v struct {
		Event      string          `json:"event"`
		TemplateID json.RawMessage `json:"message_template_id"` // a number; a string in some samples
		Reason     string          `json:"reason"`
	}
	if err := json.Unmarshal(value, &v); err != nil {
		return fmt.Errorf("decode template status: %w", err)
	}
	status := templateStatus(v.Event)
	id := strings.Trim(string(v.TemplateID), `"`)
	if status == "" || id == "" || id == "null" {
		return nil // an event we don't track
	}
	reason := ""
	if status == model.TemplateRejected {
		reason = templateReason(v.Reason)
	}
	return s.templates.TemplateStatusChanged(ctx, acc.WorkspaceID, id, status, reason)
}
