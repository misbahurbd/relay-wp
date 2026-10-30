package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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

// outbound is a checked send, ready to insert.
type outbound struct {
	msgType  string
	body     string
	content  model.Content
	template *model.Template
}

// Send queues an agent's message (AD-5, AD-6): it checks the permission for the
// type, what the conversation's channel allows (window, templates, media; ADR
// 0010) and the template, then in one transaction inserts the message as
// pending, queues inbox_send and signals. The job sends it.
func (s *Service) Send(ctx context.Context, a authctx.Access, conversationID uuid.UUID, in model.SendInput) (model.Message, error) {
	need := authctx.PermInboxReply
	if in.Type == model.SendTemplate {
		need = authctx.PermInboxSendTemplate
	}
	if !a.Can(need) {
		return model.Message{}, denied(need)
	}
	conv, err := s.Get(ctx, a, conversationID)
	if err != nil {
		return model.Message{}, err
	}
	ch, ok := s.channel(conv.Channel.Type)
	if !ok {
		return model.Message{}, model.ErrChannelUnavailable
	}
	caps := ch.Capabilities()
	if in.Type == model.SendTemplate && !caps.Templates {
		return model.Message{}, model.ErrTemplatesUnsupported
	}
	out, err := s.checkSend(ctx, a, caps, in)
	if err != nil {
		return model.Message{}, err
	}
	if out.template != nil && out.template.Category == model.CategoryMarketing && conv.ContactOptIn == "out" {
		return model.Message{}, model.ErrContactOptedOut
	}
	id, err := newID()
	if err != nil {
		return model.Message{}, err
	}
	raw, err := encodeContent(out.content)
	if err != nil {
		return model.Message{}, err
	}
	now := s.now()
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		c, err := lock(ctx, r, a, conversationID)
		if err != nil {
			return err
		}
		// Free-form messages need the channel's window, if it has one; the server
		// decides, not the client (AD-6).
		if out.template == nil && !windowOpen(caps, c.WindowExpiresAt, now) {
			return model.ErrWindowClosed
		}
		if in.ReplyToID != nil {
			q, err := r.GetMessage(ctx, gen.GetMessageParams{WorkspaceID: a.WorkspaceID, ID: *in.ReplyToID})
			if err != nil || q.ConversationID != conversationID || q.Kind != model.KindMessage {
				if err != nil && !repository.IsNoRows(err) {
					return fmt.Errorf("quoted message: %w", err)
				}
				return model.ErrInvalidReplyTo
			}
		}
		if _, err := r.InsertMessage(ctx, gen.InsertMessageParams{
			ID: id, WorkspaceID: a.WorkspaceID, ConversationID: conversationID, Direction: model.DirectionOut,
			Kind: model.KindMessage, Type: out.msgType, Body: out.body, Content: raw, ReplyToID: in.ReplyToID,
			Status: ptr(model.MessagePending), AuthorID: &a.UserID, Mentions: []uuid.UUID{}, CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("insert message: %w", err)
		}
		row, err := r.TouchOutbound(ctx, gen.TouchOutboundParams{
			At: now, Preview: preview(out.msgType, out.body), ToPending: out.template != nil,
			WorkspaceID: a.WorkspaceID, ID: conversationID,
		})
		if err != nil {
			return fmt.Errorf("touch conversation: %w", err)
		}
		if err := s.deps.Contacts.SetLastMessage(ctx, tx, a.WorkspaceID, c.ContactID, now); err != nil {
			return err
		}
		if out.template != nil && c.Status == model.StatusClosed {
			if err := s.event(ctx, r, a.WorkspaceID, conversationID, model.EventContent{
				Action: model.ActionTemplatePending, ActorID: &a.UserID,
			}); err != nil {
				return err
			}
		}
		if _, err := s.queue.InsertTx(ctx, tx, model.SendArgs{WorkspaceID: a.WorkspaceID, MessageID: id}, nil); err != nil {
			return fmt.Errorf("enqueue send: %w", err)
		}
		return signal(ctx, tx, a.WorkspaceID, model.SignalMessage, conversationID, row.TeamID, row.AssigneeID, row.ChannelType, row.ChannelID)
	})
	if err != nil {
		return model.Message{}, err
	}
	return s.message(ctx, a.WorkspaceID, conversationID, id)
}

// windowOpen reports whether free-form messages are allowed now: always on a
// channel without a window, else until the window expires.
func windowOpen(caps model.Capabilities, expires *time.Time, now time.Time) bool {
	return caps.Window <= 0 || (expires != nil && now.Before(*expires))
}

func invalid(field, msg string) error {
	return model.ErrInvalidInput.WithDetails(apperr.Detail{Field: field, Message: msg})
}

// checkSend validates a send by type against the channel's capabilities and
// builds what is stored.
func (s *Service) checkSend(ctx context.Context, a authctx.Access, caps model.Capabilities, in model.SendInput) (outbound, error) {
	switch in.Type {
	case model.SendText:
		limit := caps.MaxTextLen
		if limit <= 0 {
			limit = model.MaxTextLen
		}
		text := strings.TrimSpace(in.Text)
		if text == "" || utf8.RuneCountInString(text) > limit {
			return outbound{}, invalid("body.text", fmt.Sprintf("Write 1 to %d characters", limit))
		}
		return outbound{msgType: model.TypeText, body: text}, nil
	case model.SendMedia:
		up, ok := parseUploadKey(a.WorkspaceID, in.MediaKey)
		if !ok {
			return outbound{}, invalid("body.media_key", "Upload the file first, then send its key")
		}
		caption := strings.TrimSpace(in.Caption)
		if utf8.RuneCountInString(caption) > model.MaxCaptionLen {
			return outbound{}, invalid("body.caption", fmt.Sprintf("At most %d characters", model.MaxCaptionLen))
		}
		if caption != "" && up.Kind == model.TypeAudio {
			return outbound{}, invalid("body.caption", "Audio can't have a caption")
		}
		if err := checkMedia(caps, up); err != nil {
			return outbound{}, err
		}
		if err := s.checkStored(ctx, up.Key); err != nil {
			return outbound{}, err
		}
		return outbound{msgType: up.Kind, body: caption, content: model.Content{Media: &model.Media{
			StorageKey: up.Key, Mime: up.Mime, Size: up.Size, Filename: up.Filename,
		}}}, nil
	case model.SendTemplate:
		return s.checkTemplate(ctx, a, caps, in)
	default:
		return outbound{}, invalid("body.type", "Send text, media or template")
	}
}

// checkStored makes sure an uploaded file is still there.
func (s *Service) checkStored(ctx context.Context, key string) error {
	f, err := s.store.Get(ctx, key)
	if errors.Is(err, storage.ErrNotFound) {
		return invalid("body.media_key", "That upload has expired. Upload the file again.")
	}
	if err != nil {
		return fmt.Errorf("check upload: %w", err)
	}
	_ = f.Close()
	return nil
}

var variableRe = regexp.MustCompile(`\{\{\s*(\d+)\s*\}\}`)

// countVariables is the highest {{n}} in text.
func countVariables(text string) int {
	n := 0
	for _, m := range variableRe.FindAllStringSubmatch(text, -1) {
		if v, err := strconv.Atoi(m[1]); err == nil && v > n {
			n = v
		}
	}
	return n
}

// render fills {{n}} with params[n-1].
func render(text string, params []string) string {
	return variableRe.ReplaceAllStringFunc(text, func(m string) string {
		v, err := strconv.Atoi(variableRe.FindStringSubmatch(m)[1])
		if err != nil || v < 1 || v > len(params) {
			return m
		}
		return params[v-1]
	})
}

// option describes the variables a template needs.
func option(t model.Template) model.TemplateOption {
	o := model.TemplateOption{Template: t, BodyVariables: countVariables(t.Body), URLButtons: []int{}}
	switch t.HeaderType {
	case "text":
		o.HeaderVariable = countVariables(t.HeaderText) > 0
	case "image", "video", "document":
		o.HeaderMedia = true
	}
	for i, b := range t.Buttons {
		if b.Type == "url" && countVariables(b.Value) > 0 {
			o.URLButtons = append(o.URLButtons, i)
		}
	}
	return o
}

// checkTemplate requires an approved template with every variable filled (AD-6).
func (s *Service) checkTemplate(ctx context.Context, a authctx.Access, caps model.Capabilities, in model.SendInput) (outbound, error) {
	if in.TemplateID == nil {
		return outbound{}, invalid("body.template_id", "Choose a template")
	}
	t, err := s.deps.Templates.ApprovedTemplate(ctx, a.WorkspaceID, *in.TemplateID)
	if err != nil {
		return outbound{}, err
	}
	o := option(t)
	p := model.TemplateParams{HeaderText: strings.TrimSpace(in.Params.HeaderText)}
	var missing []apperr.Detail
	for i := range o.BodyVariables {
		v := ""
		if i < len(in.Params.Body) {
			v = strings.TrimSpace(in.Params.Body[i])
		}
		if v == "" {
			missing = append(missing, apperr.Detail{Field: fmt.Sprintf("body.params.body[%d]", i), Message: fmt.Sprintf("Fill in {{%d}}", i+1)})
		}
		p.Body = append(p.Body, v)
	}
	if o.HeaderVariable && p.HeaderText == "" {
		missing = append(missing, apperr.Detail{Field: "body.params.header_text", Message: "Fill in the header's {{1}}"})
	}
	if !o.HeaderVariable {
		p.HeaderText = ""
	}
	given := map[int]string{}
	for _, b := range in.Params.URLButtons {
		given[b.Index] = strings.TrimSpace(b.Value)
	}
	for _, i := range o.URLButtons {
		if given[i] == "" {
			missing = append(missing, apperr.Detail{Field: "body.params.url_buttons", Message: fmt.Sprintf("Fill in the link of button %d", i+1)})
			continue
		}
		p.URLButtons = append(p.URLButtons, model.URLButtonParam{Index: i, Value: given[i]})
	}
	content := model.TemplateContent{
		ID: t.ID, Name: t.Name, Language: t.Language, Category: t.Category, HeaderType: t.HeaderType, Params: p,
	}
	if o.HeaderMedia {
		up, ok := parseUploadKey(a.WorkspaceID, in.MediaKey)
		switch {
		case !ok:
			missing = append(missing, apperr.Detail{Field: "body.media_key", Message: "Upload the header's " + t.HeaderType})
		case up.Kind != t.HeaderType:
			missing = append(missing, apperr.Detail{Field: "body.media_key", Message: "The header needs a " + t.HeaderType})
		default:
			if err := checkMedia(caps, up); err != nil {
				return outbound{}, err
			}
			if err := s.checkStored(ctx, up.Key); err != nil {
				return outbound{}, err
			}
			content.HeaderMediaKey = up.Key
		}
	}
	if len(missing) > 0 {
		return outbound{}, model.ErrTemplateParamsMissing.WithDetails(missing...)
	}
	return outbound{
		msgType: model.TypeTemplate, body: render(t.Body, p.Body), template: &t,
		content: model.Content{Template: &content},
	}, nil
}

// React puts the agent's emoji on a message, or removes it with an empty emoji,
// and queues it for the channel. The channel must have reactions, and like a
// reply it needs the window (AD-6, ADR 0010).
func (s *Service) React(ctx context.Context, a authctx.Access, conversationID, messageID uuid.UUID, emoji string) error {
	if !a.Can(authctx.PermInboxReply) {
		return denied(authctx.PermInboxReply)
	}
	emoji = strings.TrimSpace(emoji)
	if len(emoji) > model.MaxEmojiLen {
		return invalid("body.emoji", "Choose one emoji")
	}
	now := s.now()
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		c, err := lock(ctx, r, a, conversationID)
		if err != nil {
			return err
		}
		ch, ok := s.channel(c.ChannelType)
		if !ok {
			return model.ErrChannelUnavailable
		}
		caps := ch.Capabilities()
		if !caps.Reactions {
			return model.ErrReactionsUnsupported
		}
		if !windowOpen(caps, c.WindowExpiresAt, now) {
			return model.ErrWindowClosed
		}
		m, err := r.GetMessage(ctx, gen.GetMessageParams{WorkspaceID: a.WorkspaceID, ID: messageID})
		if repository.IsNoRows(err) || (err == nil && (m.ConversationID != conversationID || m.Kind != model.KindMessage)) {
			return model.ErrMessageNotFound
		}
		if err != nil {
			return fmt.Errorf("get message: %w", err)
		}
		if m.ExternalID == nil {
			return model.ErrMessageNotSent
		}
		rows, err := r.SetReaction(ctx, gen.SetReactionParams{Emoji: emoji, Side: model.DirectionOut, WorkspaceID: a.WorkspaceID, ID: messageID})
		if err != nil || len(rows) == 0 {
			return err
		}
		if _, err := s.queue.InsertTx(ctx, tx, model.ReactArgs{WorkspaceID: a.WorkspaceID, MessageID: messageID, Emoji: emoji}, nil); err != nil {
			return fmt.Errorf("enqueue reaction: %w", err)
		}
		return signal(ctx, tx, a.WorkspaceID, model.SignalMessage, conversationID, rows[0].TeamID, rows[0].AssigneeID, rows[0].ChannelType, rows[0].ChannelID)
	})
}

// ---------- Jobs ----------

// failure codes for sends that never reached the channel.
const (
	codeSendFailed         = "send_failed"
	codeMediaMissing       = "media_missing"
	codeChannelUnavailable = "channel_unavailable"
)

// Deliver sends one pending message through its conversation's channel (AD-5).
// The channel's permanent refusals, and a channel type with no adapter, fail the
// message at once; other errors are returned for River to retry, and on the last
// attempt (final) the message fails.
func (s *Service) Deliver(ctx context.Context, ws, messageID uuid.UUID, final bool) error {
	m, err := repository.NewRepository(s.pool).GetMessageToSend(ctx, gen.GetMessageToSendParams{WorkspaceID: ws, ID: messageID})
	if repository.IsNoRows(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load message: %w", err)
	}
	if m.Status == nil || *m.Status != model.MessagePending {
		return nil
	}
	content, err := decodeContent(m.Content)
	if err != nil {
		return err
	}
	ch, ok := s.channel(m.ChannelType)
	if !ok {
		return s.finish(ctx, ws, messageID, "", &model.SendFailure{
			Code: codeChannelUnavailable, Title: "This chat's channel isn't available", Permanent: true,
		})
	}
	externalID, err := s.sendOne(ctx, ws, ch, m, content)
	if err == nil {
		return s.finish(ctx, ws, messageID, externalID, nil)
	}
	var f *model.SendFailure
	switch {
	case errors.As(err, &f) && (f.Permanent || final):
	case final:
		f = &model.SendFailure{Code: codeSendFailed, Title: "Couldn't reach the channel. Try sending again."}
	default:
		s.log.Warn("inbox: send failed, will retry", "workspace_id", ws, "message_id", messageID, "err", err)
		return err
	}
	s.log.Warn("inbox: send failed", "workspace_id", ws, "message_id", messageID, "code", f.Code)
	return s.finish(ctx, ws, messageID, "", f)
}

func (s *Service) sendOne(ctx context.Context, ws uuid.UUID, ch Channel, m gen.GetMessageToSendRow, c model.Content) (string, error) {
	reply := deref(m.ReplyExternalID)
	switch {
	case m.Type == model.TypeText:
		return ch.SendText(ctx, ws, m.ChannelID, m.Recipient, m.Body, reply)
	case m.Type == model.TypeTemplate && c.Template != nil && !ch.Capabilities().Templates:
		return "", &model.SendFailure{Code: model.ErrTemplatesUnsupported.Code, Title: model.ErrTemplatesUnsupported.Message, Permanent: true}
	case m.Type == model.TypeTemplate && c.Template != nil:
		t := c.Template
		out := model.OutboundTemplate{
			Name: t.Name, Language: t.Language, HeaderText: t.Params.HeaderText, BodyParams: t.Params.Body,
			URLButtonParams: map[int]string{},
		}
		for _, b := range t.Params.URLButtons {
			out.URLButtonParams[b.Index] = b.Value
		}
		if t.HeaderMediaKey != "" {
			id, err := s.uploadToChannel(ctx, ws, ch, m.ChannelID, t.HeaderMediaKey)
			if err != nil {
				return "", err
			}
			out.HeaderKind, out.HeaderMediaID = t.HeaderType, id
		}
		return ch.SendTemplate(ctx, ws, m.ChannelID, m.Recipient, out)
	case c.Media != nil && c.Media.StorageKey != "":
		id, err := s.uploadToChannel(ctx, ws, ch, m.ChannelID, c.Media.StorageKey)
		if err != nil {
			return "", err
		}
		return ch.SendMedia(ctx, ws, m.ChannelID, m.Recipient, model.OutboundMedia{
			Kind: m.Type, MediaID: id, Caption: m.Body, Filename: c.Media.Filename,
		}, reply)
	default:
		return "", &model.SendFailure{Code: codeSendFailed, Title: "This message can't be sent", Permanent: true}
	}
}

// uploadToChannel streams an uploaded file from storage to the channel and returns its media ID.
func (s *Service) uploadToChannel(ctx context.Context, ws uuid.UUID, ch Channel, channelID uuid.UUID, key string) (string, error) {
	up, ok := parseUploadKey(ws, key)
	if !ok {
		return "", &model.SendFailure{Code: codeMediaMissing, Title: "The file is missing. Upload it again.", Permanent: true}
	}
	f, err := s.store.Get(ctx, key)
	if errors.Is(err, storage.ErrNotFound) {
		return "", &model.SendFailure{Code: codeMediaMissing, Title: "The file is missing. Upload it again.", Permanent: true}
	}
	if err != nil {
		return "", fmt.Errorf("open upload: %w", err)
	}
	defer func() { _ = f.Close() }()
	return ch.UploadMedia(ctx, ws, channelID, up.Mime, up.Filename, io.LimitReader(f, up.Size), up.Size)
}

// finish records a send's outcome: sent with the channel's message ID, or failed.
func (s *Service) finish(ctx context.Context, ws, messageID uuid.UUID, externalID string, f *model.SendFailure) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		var rows []gen.MarkMessageSentRow
		var err error
		if f == nil {
			rows, err = r.MarkMessageSent(ctx, gen.MarkMessageSentParams{ExternalID: externalID, At: s.now(), WorkspaceID: ws, ID: messageID})
		} else {
			var failed []gen.MarkMessageFailedRow
			failed, err = r.MarkMessageFailed(ctx, gen.MarkMessageFailedParams{
				At: s.now(), ErrorCode: f.Code, ErrorTitle: truncate(f.Title, 200), WorkspaceID: ws, ID: messageID,
			})
			for _, row := range failed {
				rows = append(rows, gen.MarkMessageSentRow(row))
			}
		}
		if err != nil {
			return fmt.Errorf("record send: %w", err)
		}
		for _, c := range rows {
			if err := signal(ctx, tx, ws, model.SignalStatus, c.ConversationID, c.TeamID, c.AssigneeID, c.ChannelType, c.ChannelID); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeliverReaction sends the agent's reaction. It is retried a few times and then
// dropped, since a reaction has no status to fail.
func (s *Service) DeliverReaction(ctx context.Context, args model.ReactArgs, final bool) error {
	m, err := repository.NewRepository(s.pool).GetMessageToSend(ctx, gen.GetMessageToSendParams{WorkspaceID: args.WorkspaceID, ID: args.MessageID})
	if repository.IsNoRows(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load message: %w", err)
	}
	ch, ok := s.channel(m.ChannelType)
	if m.ExternalID == nil || !ok || !ch.Capabilities().Reactions {
		return nil
	}
	_, err = ch.SendReaction(ctx, args.WorkspaceID, m.ChannelID, m.Recipient, *m.ExternalID, args.Emoji)
	var f *model.SendFailure
	if err != nil && (final || (errors.As(err, &f) && f.Permanent)) {
		s.log.Warn("inbox: reaction dropped", "workspace_id", args.WorkspaceID, "message_id", args.MessageID, "err", err)
		return nil
	}
	return err
}
