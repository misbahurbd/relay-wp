package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/storage"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

const mb = 1 << 20

// MaxInboundMedia is the largest customer file the inbox stores (AD-11).
const MaxInboundMedia = 100 * mb

// fileType is a file the inbox can store and send, by extension. Which of them a
// channel takes, and how large, comes from its Capabilities (AD-11, ADR 0010).
type fileType struct {
	mime string
	kind string
}

var fileTypes = map[string]fileType{
	".jpg":  {"image/jpeg", model.TypeImage},
	".jpeg": {"image/jpeg", model.TypeImage},
	".png":  {"image/png", model.TypeImage},
	".mp4":  {"video/mp4", model.TypeVideo},
	".3gp":  {"video/3gpp", model.TypeVideo},
	".aac":  {"audio/aac", model.TypeAudio},
	".amr":  {"audio/amr", model.TypeAudio},
	".mp3":  {"audio/mpeg", model.TypeAudio},
	".m4a":  {"audio/mp4", model.TypeAudio},
	".ogg":  {"audio/ogg", model.TypeAudio},
	".pdf":  {"application/pdf", model.TypeDocument},
	".txt":  {"text/plain", model.TypeDocument},
	".doc":  {"application/msword", model.TypeDocument},
	".docx": {"application/vnd.openxmlformats-officedocument.wordprocessingml.document", model.TypeDocument},
	".xls":  {"application/vnd.ms-excel", model.TypeDocument},
	".xlsx": {"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", model.TypeDocument},
	".ppt":  {"application/vnd.ms-powerpoint", model.TypeDocument},
	".pptx": {"application/vnd.openxmlformats-officedocument.presentationml.presentation", model.TypeDocument},
}

// MaxUploadBytes is the largest file POST /media accepts; each channel's own
// limits are checked on top.
const MaxUploadBytes = 100 * mb

// mediaLimit is the largest file of this kind and type a channel takes, or false
// when it takes none.
func mediaLimit(caps model.Capabilities, kind, mime string) (int64, bool) {
	for _, m := range caps.Media {
		if m.Kind == kind && (len(m.MimeTypes) == 0 || slices.Contains(m.MimeTypes, mime)) {
			return m.MaxBytes, true
		}
	}
	return 0, false
}

// uploadLimit is the largest file of this kind and type any registered channel
// takes. An upload has no conversation yet, so it is checked against them all,
// and checkMedia checks it again against the conversation's channel at send.
func (s *Service) uploadLimit(kind, mime string) (int64, bool) {
	var limit int64
	found := false
	for _, ch := range s.deps.Channels {
		if ch == nil {
			continue
		}
		if n, ok := mediaLimit(ch.Capabilities(), kind, mime); ok {
			limit, found = max(limit, n), true
		}
	}
	return limit, found
}

// checkMedia checks an upload against one channel's media kinds and limits.
func checkMedia(caps model.Capabilities, up model.Upload) error {
	limit, ok := mediaLimit(caps, up.Kind, up.Mime)
	if !ok {
		return model.ErrMediaTypeNotAllowed.WithDetails(apperr.Detail{
			Field: "body.media_key", Message: "This channel can't send this type of file",
		})
	}
	if up.Size > limit {
		return model.ErrMediaTooLarge.WithDetails(apperr.Detail{
			Field: "body.media_key", Message: fmt.Sprintf("A %s can be at most %s on this channel", up.Kind, sizeLabel(limit)),
		})
	}
	return nil
}

// sizeLabel shows a byte limit in MB, or in KB below 1 MB.
func sizeLabel(n int64) string {
	if n >= mb {
		return fmt.Sprintf("%d MB", n/mb)
	}
	return fmt.Sprintf("%d KB", n>>10)
}

// extensionOf gives the stored file of an inbound message an extension for its MIME type.
func extensionOf(mime string) string {
	mime, _, _ = strings.Cut(mime, ";")
	mime = strings.TrimSpace(strings.ToLower(mime))
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "audio/ogg":
		return ".ogg"
	}
	for ext, t := range fileTypes {
		if t.mime == mime && ext != ".jpeg" {
			return ext
		}
	}
	return ""
}

// safeName keeps a file name's letters, digits, dots, dashes and underscores.
func safeName(name string) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	s := strings.Trim(b.String(), "._")
	if len(s) > 100 {
		ext := path.Ext(s)
		s = s[:100-len(ext)] + ext
	}
	if s == "" || strings.HasPrefix(s, ".") {
		s = "file" + s
	}
	return s
}

// uploadKey is where an outbound upload is stored. It carries the workspace (so
// a key from another workspace is refused), the size and the name, so a send
// needs nothing but the key.
func uploadKey(ws, id uuid.UUID, size int64, name string) string {
	return fmt.Sprintf("inbox/%s/uploads/%s/%d/%s", ws, id, size, name)
}

// parseUploadKey checks that key is an upload of workspace ws and reads it back.
func parseUploadKey(ws uuid.UUID, key string) (model.Upload, bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 6 || parts[0] != "inbox" || parts[1] != ws.String() || parts[2] != "uploads" {
		return model.Upload{}, false
	}
	if _, err := uuid.Parse(parts[3]); err != nil {
		return model.Upload{}, false
	}
	size, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil || size <= 0 || parts[5] != safeName(parts[5]) {
		return model.Upload{}, false
	}
	t, ok := fileTypes[strings.ToLower(path.Ext(parts[5]))]
	if !ok || size > MaxUploadBytes {
		return model.Upload{}, false
	}
	return model.Upload{Key: key, Mime: t.mime, Size: size, Filename: parts[5], Kind: t.kind}, true
}

// Upload stores a file an agent will send (AD-5, AD-11), after checking its
// type and size against the registered channels' limits (the largest that any
// channel takes; the send checks the conversation's own channel).
func (s *Service) Upload(ctx context.Context, a authctx.Access, filename string, size int64, data io.Reader) (model.Upload, error) {
	name := safeName(filename)
	t, ok := fileTypes[strings.ToLower(path.Ext(name))]
	limit := int64(0)
	if ok {
		limit, ok = s.uploadLimit(t.kind, t.mime)
	}
	if !ok {
		return model.Upload{}, model.ErrMediaTypeNotAllowed.WithDetails(apperr.Detail{
			Field: "file", Message: "Send a JPEG or PNG image, an MP4 or 3GP video, an audio file, or a PDF or Office document",
		})
	}
	if size <= 0 {
		return model.Upload{}, invalid("file", "The file is empty")
	}
	if size > limit {
		return model.Upload{}, model.ErrMediaTooLarge.WithDetails(apperr.Detail{
			Field: "file", Message: fmt.Sprintf("A %s can be at most %s", t.kind, sizeLabel(limit)),
		})
	}
	id, err := newID()
	if err != nil {
		return model.Upload{}, err
	}
	key := uploadKey(a.WorkspaceID, id, size, name)
	if err := s.store.Put(ctx, key, io.LimitReader(data, size), size, t.mime); err != nil {
		return model.Upload{}, fmt.Errorf("store upload: %w", err)
	}
	return model.Upload{Key: key, Mime: t.mime, Size: size, Filename: name, Kind: t.kind}, nil
}

// OpenMedia streams a message's file after the scope check (AD-11). The caller closes it.
func (s *Service) OpenMedia(ctx context.Context, a authctx.Access, conversationID, messageID uuid.UUID) (io.ReadCloser, model.MediaFile, error) {
	if _, err := s.Get(ctx, a, conversationID); err != nil {
		return nil, model.MediaFile{}, err
	}
	m, err := repository.NewRepository(s.pool).GetMessage(ctx, gen.GetMessageParams{WorkspaceID: a.WorkspaceID, ID: messageID})
	if repository.IsNoRows(err) || (err == nil && m.ConversationID != conversationID) {
		return nil, model.MediaFile{}, model.ErrMessageNotFound
	}
	if err != nil {
		return nil, model.MediaFile{}, fmt.Errorf("get message: %w", err)
	}
	c, err := decodeContent(m.Content)
	if err != nil {
		return nil, model.MediaFile{}, err
	}
	key, info := "", model.MediaFile{}
	switch {
	case c.Media != nil:
		key, info = c.Media.StorageKey, model.MediaFile{Mime: c.Media.Mime, Filename: c.Media.Filename, Size: c.Media.Size}
	case c.Template != nil && c.Template.HeaderMediaKey != "":
		if up, ok := parseUploadKey(a.WorkspaceID, c.Template.HeaderMediaKey); ok {
			key, info = up.Key, model.MediaFile{Mime: up.Mime, Filename: up.Filename, Size: up.Size}
		}
	case m.Kind == model.KindMessage:
	default:
		return nil, model.MediaFile{}, model.ErrMessageNotFound
	}
	if key == "" {
		return nil, model.MediaFile{}, model.ErrMediaNotReady
	}
	f, err := s.store.Get(ctx, key)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, model.MediaFile{}, model.ErrMediaNotReady
	}
	if err != nil {
		return nil, model.MediaFile{}, fmt.Errorf("open media: %w", err)
	}
	if info.Mime == "" {
		info.Mime = "application/octet-stream"
	}
	return f, info, nil
}

// FetchMedia downloads an inbound message's file from its channel into storage
// and records its key, type, size and hash (AD-11). An error makes River retry.
func (s *Service) FetchMedia(ctx context.Context, ws, messageID uuid.UUID) error {
	m, err := repository.NewRepository(s.pool).GetMessageToSend(ctx, gen.GetMessageToSendParams{WorkspaceID: ws, ID: messageID})
	if repository.IsNoRows(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load message: %w", err)
	}
	c, err := decodeContent(m.Content)
	if err != nil {
		return err
	}
	if c.Media == nil || c.Media.ExternalID == "" || c.Media.StorageKey != "" {
		return nil
	}
	ch, ok := s.channel(m.ChannelType)
	if !ok {
		s.log.Warn("inbox: inbound media on an unregistered channel, not stored", "workspace_id", ws, "message_id", messageID)
		return nil
	}
	body, mime, size, err := ch.FetchMedia(ctx, ws, m.ChannelID, c.Media.ExternalID)
	if err != nil {
		return fmt.Errorf("fetch media: %w", err)
	}
	defer func() { _ = body.Close() }()
	if size > MaxInboundMedia {
		s.log.Warn("inbox: inbound media too large, not stored", "workspace_id", ws, "message_id", messageID, "size", size)
		return nil
	}
	if mime == "" {
		mime = c.Media.Mime
	}
	key := fmt.Sprintf("inbox/%s/%s/%s%s", ws, m.ConversationID, messageID, extensionOf(mime))
	h := sha256.New()
	if err := s.store.Put(ctx, key, io.TeeReader(io.LimitReader(body, MaxInboundMedia), h), size, mime); err != nil {
		return fmt.Errorf("store media: %w", err)
	}
	media, err := json.Marshal(model.Media{StorageKey: key, Mime: mime, Size: size, SHA256: hex.EncodeToString(h.Sum(nil))})
	if err != nil {
		return fmt.Errorf("encode media: %w", err)
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		rows, err := repository.NewRepository(tx).SetMessageMedia(ctx, gen.SetMessageMediaParams{Media: media, WorkspaceID: ws, ID: messageID})
		if err != nil {
			return fmt.Errorf("record media: %w", err)
		}
		for _, c := range rows {
			if err := signal(ctx, tx, ws, model.SignalMessage, c.ConversationID, c.TeamID, c.AssigneeID, c.ChannelType, c.ChannelID); err != nil {
				return err
			}
		}
		return nil
	})
}
