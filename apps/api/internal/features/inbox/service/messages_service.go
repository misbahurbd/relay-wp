package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Thread returns a page of a conversation's messages, newest first, and the id
// to continue before (F3).
func (s *Service) Thread(ctx context.Context, a authctx.Access, conversationID uuid.UUID, before *uuid.UUID) ([]model.Message, *uuid.UUID, error) {
	if _, err := s.Get(ctx, a, conversationID); err != nil {
		return nil, nil, err
	}
	rows, err := repository.NewRepository(s.pool).ListMessages(ctx, gen.ListMessagesParams{
		WorkspaceID: a.WorkspaceID, ConversationID: conversationID, BeforeID: before, PageSize: model.ThreadPageSize + 1,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list messages: %w", err)
	}
	var next *uuid.UUID
	if len(rows) > model.ThreadPageSize {
		rows = rows[:model.ThreadPageSize]
		next = &rows[len(rows)-1].ID
	}
	out := make([]model.Message, len(rows))
	for i, row := range rows {
		if out[i], err = toMessage(gen.GetThreadMessageRow(row)); err != nil {
			return nil, nil, err
		}
	}
	return out, next, nil
}

func (s *Service) message(ctx context.Context, ws, conversationID, id uuid.UUID) (model.Message, error) {
	row, err := repository.NewRepository(s.pool).GetThreadMessage(ctx, gen.GetThreadMessageParams{
		WorkspaceID: ws, ConversationID: conversationID, ID: id,
	})
	if repository.IsNoRows(err) {
		return model.Message{}, model.ErrMessageNotFound
	}
	if err != nil {
		return model.Message{}, fmt.Errorf("get message: %w", err)
	}
	return toMessage(row)
}

func toMessage(row gen.GetThreadMessageRow) (model.Message, error) {
	m := model.Message{
		ID: row.ID, Direction: row.Direction, Kind: row.Kind, Type: row.Type, Body: row.Body, Status: row.Status,
		ErrorCode: row.ErrorCode, ErrorTitle: row.ErrorTitle, Mentions: row.Mentions, CreatedAt: row.CreatedAt,
		StatusAt: row.StatusAt, Automated: row.AutomationRunID != nil,
		Author: person(row.AuthorID, row.AuthorDisplayName, row.AuthorFirstName, row.AuthorLastName, row.AuthorEmail),
	}
	var err error
	if m.Content, err = decodeContent(row.Content); err != nil {
		return m, err
	}
	if len(row.Reactions) > 0 {
		if err := json.Unmarshal(row.Reactions, &m.Reactions); err != nil {
			return m, fmt.Errorf("decode reactions: %w", err)
		}
	}
	if row.ReplyToID != nil {
		m.ReplyTo = &model.ReplyRef{
			ID: *row.ReplyToID, Direction: deref(row.ReplyDirection), Type: deref(row.ReplyType),
			Body: truncate(deref(row.ReplyBody), model.MaxPreview),
		}
	}
	if m.Mentions == nil {
		m.Mentions = []uuid.UUID{}
	}
	return m, nil
}

// AddNote writes a private note, notifying each @mentioned member in the app
// (F6, AD-12). Every mention must be an active member who can see the chat.
func (s *Service) AddNote(ctx context.Context, a authctx.Access, conversationID uuid.UUID, text string, mentions []uuid.UUID) (model.Message, error) {
	if !a.Can(authctx.PermInboxNotes) {
		return model.Message{}, denied(authctx.PermInboxNotes)
	}
	text = strings.TrimSpace(text)
	if text == "" || utf8.RuneCountInString(text) > model.MaxNoteLen {
		return model.Message{}, invalid("body.text", fmt.Sprintf("Write 1 to %d characters", model.MaxNoteLen))
	}
	mentions = uniqueIDs(mentions)
	if len(mentions) > model.MaxMentions {
		return model.Message{}, invalid("body.mentions", fmt.Sprintf("Mention at most %d people", model.MaxMentions))
	}
	conv, err := s.Get(ctx, a, conversationID)
	if err != nil {
		return model.Message{}, err
	}
	for _, uid := range mentions {
		m, suspended, ok, err := s.deps.Members.ResolveAccess(ctx, a.WorkspaceID, uid)
		if err != nil {
			return model.Message{}, fmt.Errorf("check mention: %w", err)
		}
		var assignee *uuid.UUID
		if conv.Assignee != nil {
			assignee = &conv.Assignee.ID
		}
		if !ok || suspended || !hasInboxRole(m) || !canSee(m, conv.TeamID, assignee, conv.Channel.Type, conv.Channel.ID) {
			return model.Message{}, model.ErrInvalidMention.WithDetails(apperr.Detail{Field: "body.mentions", Message: uid.String()})
		}
	}
	id, err := newID()
	if err != nil {
		return model.Message{}, err
	}
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		c, err := lock(ctx, r, a, conversationID)
		if err != nil {
			return err
		}
		if _, err := r.InsertMessage(ctx, gen.InsertMessageParams{
			ID: id, WorkspaceID: a.WorkspaceID, ConversationID: conversationID, Direction: model.DirectionOut,
			Kind: model.KindNote, Type: model.TypeText, Body: text, Content: []byte("{}"), AuthorID: &a.UserID,
			Mentions: mentions, CreatedAt: s.now(),
		}); err != nil {
			return fmt.Errorf("insert note: %w", err)
		}
		if len(mentions) > 0 {
			if err := r.AddMentions(ctx, gen.AddMentionsParams{
				WorkspaceID: a.WorkspaceID, MessageID: id, ConversationID: conversationID, UserIds: mentions,
			}); err != nil {
				return fmt.Errorf("add mentions: %w", err)
			}
		}
		if err := signal(ctx, tx, a.WorkspaceID, model.SignalNote, conversationID, c.TeamID, c.AssigneeID, c.ChannelType, c.ChannelID); err != nil {
			return err
		}
		return s.notifyMentions(ctx, tx, a.WorkspaceID, conversationID, c.ChannelType, c.ChannelID, mentions)
	})
	if err != nil {
		return model.Message{}, err
	}
	return s.message(ctx, a.WorkspaceID, conversationID, id)
}

func (s *Service) notifyMentions(ctx context.Context, tx pgx.Tx, ws, conversationID uuid.UUID, channelType string, channelID uuid.UUID,
	users []uuid.UUID,
) error {
	for _, uid := range users {
		ev := model.SignalMention
		if err := notifyUser(ctx, tx, ws, ev, conversationID, channelType, channelID, uid); err != nil {
			return err
		}
	}
	return nil
}

func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		if !seen[id] && id != uuid.Nil {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// mentionsPage is how many mentions the bell list shows.
const mentionsPage = 50

// Mentions returns the member's newest mentions in chats they can still see,
// and how many of those are unseen.
func (s *Service) Mentions(ctx context.Context, a authctx.Access) ([]model.Mention, int, error) {
	rows, err := repository.NewRepository(s.pool).ListMentions(ctx, gen.ListMentionsParams{
		WorkspaceID: a.WorkspaceID, UserID: a.UserID, PageSize: mentionsPage * 2,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list mentions: %w", err)
	}
	out := make([]model.Mention, 0, len(rows))
	unseen := 0
	for _, row := range rows {
		if !canSee(a, row.TeamID, row.AssigneeID, row.ChannelType, row.ChannelID) {
			continue
		}
		if len(out) == mentionsPage {
			break
		}
		if row.SeenAt == nil {
			unseen++
		}
		out = append(out, model.Mention{
			MessageID: row.MessageID, ConversationID: row.ConversationID,
			Contact: model.ContactRef{Name: row.ContactName, Phone: row.ContactPhone},
			Body:    truncate(row.Body, 280), SeenAt: row.SeenAt, CreatedAt: row.CreatedAt,
			Author: person(row.AuthorID, row.AuthorDisplayName, row.AuthorFirstName, row.AuthorLastName, row.AuthorEmail),
		})
	}
	return out, unseen, nil
}

// MarkMentionsSeen marks the member's mentions seen: all of them, or only in
// the given notes. It returns how many changed.
func (s *Service) MarkMentionsSeen(ctx context.Context, a authctx.Access, messageIDs []uuid.UUID) (int64, error) {
	var ids []uuid.UUID
	if len(messageIDs) > 0 {
		ids = uniqueIDs(messageIDs)
	}
	n, err := repository.NewRepository(s.pool).MarkMentionsSeen(ctx, gen.MarkMentionsSeenParams{
		WorkspaceID: a.WorkspaceID, UserID: a.UserID, MessageIds: ids,
	})
	if err != nil {
		return 0, fmt.Errorf("mark mentions seen: %w", err)
	}
	return n, nil
}
