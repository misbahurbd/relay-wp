package handler

import (
	"context"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/logger"
)

func (h *Handler) listMessages(ctx context.Context, in *dto.ListMessagesInput) (*dto.InboxMessagesOutput, error) {
	var before *uuid.UUID
	if in.Before != "" {
		id, err := uuid.Parse(in.Before)
		if err != nil {
			return nil, model.ErrInvalidCursor
		}
		before = &id
	}
	list, next, err := h.svc.Thread(ctx, actor(ctx), in.ConversationID, before)
	if err != nil {
		return nil, err
	}
	out := &dto.InboxMessagesOutput{}
	out.Body.Items = make([]dto.InboxMessageDTO, len(list))
	for i, m := range list {
		out.Body.Items[i] = dto.ToMessageDTO(m)
	}
	out.Body.NextBefore = next
	return out, nil
}

func message(m model.Message, err error) (*dto.InboxMessageOutput, error) {
	if err != nil {
		return nil, err
	}
	return &dto.InboxMessageOutput{Body: dto.ToMessageDTO(m)}, nil
}

func (h *Handler) sendMessage(ctx context.Context, in *dto.SendMessageInput) (*dto.InboxMessageOutput, error) {
	return message(h.svc.Send(ctx, actor(ctx), in.ConversationID, in.ToModel()))
}

func (h *Handler) addNote(ctx context.Context, in *dto.AddNoteInput) (*dto.InboxMessageOutput, error) {
	return message(h.svc.AddNote(ctx, actor(ctx), in.ConversationID, in.Body.Text, in.Body.Mentions))
}

func (h *Handler) react(ctx context.Context, in *dto.ReactInput) (*struct{}, error) {
	return nil, h.svc.React(ctx, actor(ctx), in.ConversationID, in.MessageID, in.Body.Emoji)
}

func (h *Handler) uploadMedia(ctx context.Context, in *dto.UploadMediaInput) (*dto.InboxUploadOutput, error) {
	f := in.RawBody.Data().File
	defer func() { _ = f.Close() }()
	u, err := h.svc.Upload(ctx, actor(ctx), f.Filename, f.Size, f)
	if err != nil {
		return nil, err
	}
	return dto.ToUploadOutput(u), nil
}

// inlineTypes are shown in the browser; anything else downloads.
var inlineTypes = []string{"image/", "video/", "audio/"}

func (h *Handler) media(ctx context.Context, in *dto.MessagePath) (*huma.StreamResponse, error) {
	f, info, err := h.svc.OpenMedia(ctx, actor(ctx), in.ConversationID, in.MessageID)
	if err != nil {
		return nil, err
	}
	disposition := "attachment"
	for _, p := range inlineTypes {
		if strings.HasPrefix(info.Mime, p) {
			disposition = "inline"
		}
	}
	if info.Filename != "" {
		disposition = mime.FormatMediaType(disposition, map[string]string{"filename": info.Filename})
	}
	return &huma.StreamResponse{Body: func(hctx huma.Context) {
		defer func() { _ = f.Close() }()
		hctx.SetHeader("Content-Type", info.Mime)
		hctx.SetHeader("Content-Disposition", disposition)
		hctx.SetHeader("Cache-Control", "private, max-age=3600")
		hctx.SetHeader("X-Content-Type-Options", "nosniff")
		if info.Size > 0 {
			hctx.SetHeader("Content-Length", strconv.FormatInt(info.Size, 10))
		}
		hctx.SetStatus(http.StatusOK)
		if _, err := io.Copy(hctx.BodyWriter(), f); err != nil {
			logger.FromContext(ctx).Warn("inbox media stream cut off", "message_id", in.MessageID, "err", err)
		}
	}}, nil
}
