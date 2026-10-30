package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

var shortcutRe = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

// QuickReplies lists the workspace's quick replies by shortcut (F8).
func (s *Service) QuickReplies(ctx context.Context, ws uuid.UUID) ([]model.QuickReply, error) {
	rows, err := repository.NewRepository(s.pool).ListQuickReplies(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("list quick replies: %w", err)
	}
	out := make([]model.QuickReply, len(rows))
	for i, r := range rows {
		out[i] = model.QuickReply(r)
	}
	return out, nil
}

// checkQuickReply trims and validates the given fields; create needs all of them.
func checkQuickReply(in model.QuickReplyInput, create bool) (model.QuickReplyInput, error) {
	var details []apperr.Detail
	check := func(v **string, field string, ok func(string) bool, msg string) {
		if *v == nil {
			if create {
				details = append(details, apperr.Detail{Field: "body." + field, Message: msg})
			}
			return
		}
		t := strings.TrimSpace(**v)
		if field == "shortcut" {
			t = strings.ToLower(strings.TrimPrefix(t, "/"))
		}
		if !ok(t) {
			details = append(details, apperr.Detail{Field: "body." + field, Message: msg})
		}
		*v = &t
	}
	check(&in.Shortcut, "shortcut", shortcutRe.MatchString, "Use 1 to 32 lowercase letters, digits, - or _")
	check(&in.Title, "title", func(t string) bool { return t != "" && utf8.RuneCountInString(t) <= model.MaxQuickReplyTitleLen },
		fmt.Sprintf("Write 1 to %d characters", model.MaxQuickReplyTitleLen))
	check(&in.Body, "body", func(t string) bool { return t != "" && utf8.RuneCountInString(t) <= model.MaxQuickReplyBodyLen },
		fmt.Sprintf("Write 1 to %d characters", model.MaxQuickReplyBodyLen))
	if len(details) > 0 {
		return in, model.ErrInvalidInput.WithDetails(details...)
	}
	return in, nil
}

// CreateQuickReply adds a quick reply.
func (s *Service) CreateQuickReply(ctx context.Context, a authctx.Access, in model.QuickReplyInput) (model.QuickReply, error) {
	in, err := checkQuickReply(in, true)
	if err != nil {
		return model.QuickReply{}, err
	}
	id, err := newID()
	if err != nil {
		return model.QuickReply{}, err
	}
	row, err := repository.NewRepository(s.pool).CreateQuickReply(ctx, gen.CreateQuickReplyParams{
		ID: id, WorkspaceID: a.WorkspaceID, Shortcut: *in.Shortcut, Title: *in.Title, Body: *in.Body, CreatedBy: &a.UserID,
	})
	if repository.IsUnique(err, repository.QuickReplyShortcutConstraint) {
		return model.QuickReply{}, model.ErrQuickReplyExists
	}
	if err != nil {
		return model.QuickReply{}, fmt.Errorf("create quick reply: %w", err)
	}
	return model.QuickReply(row), nil
}

// UpdateQuickReply changes the given fields of a quick reply.
func (s *Service) UpdateQuickReply(ctx context.Context, ws, id uuid.UUID, in model.QuickReplyInput) (model.QuickReply, error) {
	in, err := checkQuickReply(in, false)
	if err != nil {
		return model.QuickReply{}, err
	}
	row, err := repository.NewRepository(s.pool).UpdateQuickReply(ctx, gen.UpdateQuickReplyParams{
		Shortcut: in.Shortcut, Title: in.Title, Body: in.Body, WorkspaceID: ws, ID: id,
	})
	switch {
	case repository.IsNoRows(err):
		return model.QuickReply{}, model.ErrQuickReplyNotFound
	case repository.IsUnique(err, repository.QuickReplyShortcutConstraint):
		return model.QuickReply{}, model.ErrQuickReplyExists
	case err != nil:
		return model.QuickReply{}, fmt.Errorf("update quick reply: %w", err)
	}
	return model.QuickReply(row), nil
}

// DeleteQuickReply removes a quick reply.
func (s *Service) DeleteQuickReply(ctx context.Context, ws, id uuid.UUID) error {
	n, err := repository.NewRepository(s.pool).DeleteQuickReply(ctx, gen.DeleteQuickReplyParams{WorkspaceID: ws, ID: id})
	if err != nil {
		return fmt.Errorf("delete quick reply: %w", err)
	}
	if n == 0 {
		return model.ErrQuickReplyNotFound
	}
	return nil
}

// Templates lists the approved templates with the variables a send must fill,
// for a channel type (WhatsApp when empty). A channel without templates, or one
// not registered, answers ErrTemplatesUnsupported (ADR 0010).
func (s *Service) Templates(ctx context.Context, ws uuid.UUID, channelType string) ([]model.TemplateOption, error) {
	if channelType == "" {
		channelType = model.ChannelWhatsApp
	}
	if ch, ok := s.channel(channelType); !ok || !ch.Capabilities().Templates {
		return nil, model.ErrTemplatesUnsupported
	}
	list, err := s.deps.Templates.ApprovedTemplates(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("approved templates: %w", err)
	}
	out := make([]model.TemplateOption, len(list))
	for i, t := range list {
		out[i] = option(t)
	}
	return out, nil
}
