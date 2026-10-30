package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// part is one slice of a member's scope, listed by its own query (ADR 0002:
// one query per team, merged, rather than team_id = ANY).
type part struct {
	teamID, assigneeID *uuid.UUID
	outsideTeams       []uuid.UUID
}

// partsOf splits a member's scope into disjoint slices.
func partsOf(a authctx.Access) []part {
	if a.Owner || a.Scope == authctx.ScopeAll {
		return []part{{}}
	}
	me := a.UserID
	if a.Scope != authctx.ScopeTeam {
		return []part{{assigneeID: &me}}
	}
	parts := make([]part, 0, len(a.Teams)+1)
	for _, t := range a.Teams {
		parts = append(parts, part{teamID: &t})
	}
	// Chats assigned to the member outside their teams.
	return append(parts, part{assigneeID: &me, outsideTeams: a.Teams})
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// phoneDigits returns the digits of q when q looks like (part of) a phone number.
func phoneDigits(q string) (string, bool) {
	var b strings.Builder
	for _, r := range q {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case strings.ContainsRune(" +-().", r):
		default:
			return "", false
		}
	}
	return b.String(), b.Len() >= 3
}

// List returns one page of the member's conversations with the tab counts (F2).
// Search is ILIKE on the last message, the contact's name and phone (AD-4: the
// trigram index is left out until pg_trgm is installed; at a page of 50 behind
// the scope's index it is fast enough).
func (s *Service) List(ctx context.Context, a authctx.Access, f model.ListFilter) (model.Page, error) {
	if f.Limit <= 0 || f.Limit > model.MaxPageLimit {
		f.Limit = model.DefaultPageLimit
	}
	var status *string
	notClosed := false
	parts := partsOf(a)
	switch f.Tab {
	case "", model.TabOpen:
		status = ptr(model.StatusOpen)
	case model.TabPending, model.TabClosed:
		status = &f.Tab
	case model.TabMine:
		notClosed, parts = true, []part{{assigneeID: &a.UserID}}
	default:
		return model.Page{}, model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "tab", Message: "Unknown tab"})
	}
	var query, phoneQuery *string
	if q := strings.TrimSpace(f.Query); q != "" {
		query = ptr(likeEscaper.Replace(q))
		if d, ok := phoneDigits(q); ok {
			phoneQuery = &d
		}
	}
	waiting := f.Sort == model.SortWaiting
	numbers := numbersOf(a)
	r := repository.NewRepository(s.pool)

	var rows []model.Conversation
	for _, p := range parts {
		got, err := s.listPart(ctx, r, a.WorkspaceID, p, status, notClosed, numbers, query, phoneQuery, waiting, f.Cursor, f.Limit+1)
		if err != nil {
			return model.Page{}, err
		}
		rows = append(rows, got...)
	}
	if len(parts) > 1 {
		slices.SortFunc(rows, func(x, y model.Conversation) int { return compareRows(x, y, waiting) })
	}
	page := model.Page{Items: rows}
	if len(rows) > int(f.Limit) {
		page.Items = rows[:f.Limit]
		last := page.Items[len(page.Items)-1]
		page.Next = &model.Cursor{ID: last.ID, At: &last.LastMessageAt}
		if waiting {
			page.Next.At = last.WaitingSince
		}
	}
	if page.Items == nil {
		page.Items = []model.Conversation{}
	}
	if err := s.nameChannels(ctx, a.WorkspaceID, page.Items); err != nil {
		return model.Page{}, err
	}
	counts, err := s.counts(ctx, r, a, numbers)
	if err != nil {
		return model.Page{}, err
	}
	page.Counts = counts
	return page, nil
}

func (s *Service) listPart(ctx context.Context, r repository.Repository, ws uuid.UUID, p part, status *string, notClosed bool,
	numbers []uuid.UUID, query, phoneQuery *string, waiting bool, cursor *model.Cursor, limit int32,
) ([]model.Conversation, error) {
	var afterAt *time.Time
	var afterID uuid.UUID
	if cursor != nil {
		afterAt, afterID = cursor.At, cursor.ID
	}
	if !waiting {
		if cursor != nil && afterAt == nil {
			return nil, model.ErrInvalidCursor
		}
		rows, err := r.ListConversationsNewest(ctx, gen.ListConversationsNewestParams{
			WorkspaceID: ws, Status: status, NotClosed: notClosed, TeamID: p.teamID, AssigneeID: p.assigneeID,
			OutsideTeams: p.outsideTeams, NumberIds: numbers, Query: query, PhoneQuery: phoneQuery,
			AfterAt: afterAt, AfterID: afterID, PageSize: limit,
		})
		if err != nil {
			return nil, fmt.Errorf("list conversations: %w", err)
		}
		out := make([]model.Conversation, len(rows))
		for i, row := range rows {
			out[i] = toConversation(gen.ListConversationsWaitingRow(row))
		}
		return out, nil
	}
	rows, err := r.ListConversationsWaiting(ctx, gen.ListConversationsWaitingParams{
		WorkspaceID: ws, Status: status, NotClosed: notClosed, TeamID: p.teamID, AssigneeID: p.assigneeID,
		OutsideTeams: p.outsideTeams, NumberIds: numbers, Query: query, PhoneQuery: phoneQuery,
		HasCursor: cursor != nil, AfterAt: afterAt, AfterID: afterID, PageSize: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	out := make([]model.Conversation, len(rows))
	for i, row := range rows {
		out[i] = toConversation(row)
	}
	return out, nil
}

// compareRows orders merged rows as the queries do.
func compareRows(x, y model.Conversation, waiting bool) int {
	if !waiting {
		if c := y.LastMessageAt.Compare(x.LastMessageAt); c != 0 {
			return c
		}
		return strings.Compare(y.ID.String(), x.ID.String())
	}
	switch {
	case x.WaitingSince != nil && y.WaitingSince == nil:
		return -1
	case x.WaitingSince == nil && y.WaitingSince != nil:
		return 1
	case x.WaitingSince != nil:
		if c := x.WaitingSince.Compare(*y.WaitingSince); c != 0 {
			return c
		}
	}
	return strings.Compare(x.ID.String(), y.ID.String())
}

func (s *Service) counts(ctx context.Context, r repository.Repository, a authctx.Access, numbers []uuid.UUID) (model.Counts, error) {
	var c model.Counts
	for _, p := range partsOf(a) {
		row, err := r.CountConversations(ctx, gen.CountConversationsParams{
			UserID: a.UserID, WorkspaceID: a.WorkspaceID, TeamID: p.teamID, AssigneeID: p.assigneeID,
			OutsideTeams: p.outsideTeams, NumberIds: numbers,
		})
		if err != nil {
			return c, fmt.Errorf("count conversations: %w", err)
		}
		c.Open += row.OpenCount
		c.Pending += row.PendingCount
		c.Closed += row.ClosedCount
		c.Mine += row.MineCount
	}
	return c, nil
}

func toConversation(row gen.ListConversationsWaitingRow) model.Conversation {
	return model.Conversation{
		ID:      row.ID,
		Contact: model.ContactRef{ID: row.ContactID, Name: row.ContactName, Phone: row.ContactPhone},
		Channel: model.ChannelRef{Type: row.ChannelType, ID: row.ChannelID},
		Status:  row.Status, Source: row.Source, TeamID: row.TeamID,
		Assignee:      person(row.AssigneeID, row.AssigneeDisplayName, row.AssigneeFirstName, row.AssigneeLastName, row.AssigneeEmail),
		LastMessageAt: row.LastMessageAt, LastMessagePreview: row.LastMessagePreview,
		LastMessageDirection: row.LastMessageDirection, WindowExpiresAt: row.WindowExpiresAt,
		UnreadCount: row.UnreadCount, WaitingSince: row.WaitingSince, SnoozedUntil: row.SnoozedUntil,
	}
}

// nameChannels fills each conversation's channel name and handle from its
// adapter, one lookup per channel type. A channel that isn't registered, or an
// account that is gone, keeps empty names: the chat stays readable.
func (s *Service) nameChannels(ctx context.Context, ws uuid.UUID, convs []model.Conversation) error {
	accounts := map[string]map[uuid.UUID]model.ChannelAccount{}
	for i := range convs {
		ref := &convs[i].Channel
		names, seen := accounts[ref.Type]
		if !seen {
			if ch, ok := s.channel(ref.Type); ok {
				var err error
				if names, err = ch.Accounts(ctx, ws); err != nil {
					return fmt.Errorf("%s accounts: %w", ref.Type, err)
				}
			}
			accounts[ref.Type] = names
		}
		if acc, ok := names[ref.ID]; ok {
			ref.Name, ref.Handle = acc.Name, acc.Handle
		}
	}
	return nil
}

// Get returns one conversation in the member's scope; out of scope is not found (AD-10).
func (s *Service) Get(ctx context.Context, a authctx.Access, id uuid.UUID) (model.ConversationDetail, error) {
	return s.get(ctx, s.pool, a, id)
}

func (s *Service) get(ctx context.Context, db database.DBTX, a authctx.Access, id uuid.UUID) (model.ConversationDetail, error) {
	row, err := repository.NewRepository(db).GetConversation(ctx, gen.GetConversationParams{WorkspaceID: a.WorkspaceID, ID: id})
	if repository.IsNoRows(err) {
		return model.ConversationDetail{}, model.ErrConversationNotFound
	}
	if err != nil {
		return model.ConversationDetail{}, fmt.Errorf("get conversation: %w", err)
	}
	if !canSee(a, row.TeamID, row.AssigneeID, row.ChannelType, row.ChannelID) {
		return model.ConversationDetail{}, model.ErrConversationNotFound
	}
	d := model.ConversationDetail{
		Conversation: model.Conversation{
			ID:      row.ID,
			Contact: model.ContactRef{ID: row.ContactID, Name: row.ContactName, Phone: row.ContactPhone},
			Channel: model.ChannelRef{Type: row.ChannelType, ID: row.ChannelID},
			Status:  row.Status, Source: row.Source, TeamID: row.TeamID,
			Assignee:      person(row.AssigneeID, row.AssigneeDisplayName, row.AssigneeFirstName, row.AssigneeLastName, row.AssigneeEmail),
			LastMessageAt: row.LastMessageAt, LastMessagePreview: row.LastMessagePreview,
			LastMessageDirection: row.LastMessageDirection, WindowExpiresAt: row.WindowExpiresAt,
			UnreadCount: row.UnreadCount, WaitingSince: row.WaitingSince, SnoozedUntil: row.SnoozedUntil,
		},
		ContactOptIn: row.ContactOptIn, TeamName: row.TeamName,
		LastInboundAt: row.LastInboundAt, ClosedAt: row.ClosedAt, CreatedAt: row.CreatedAt,
		TemplateSinceInbound: row.TemplateSinceInbound,
	}
	if ch, ok := s.channel(row.ChannelType); ok {
		d.Capabilities = ch.Capabilities()
	}
	one := []model.Conversation{d.Conversation}
	if err := s.nameChannels(ctx, a.WorkspaceID, one); err != nil {
		return model.ConversationDetail{}, err
	}
	d.Channel = one[0].Channel
	return d, nil
}

// lock locks a conversation in tx for a write, checking the member can see it.
func lock(ctx context.Context, r repository.Repository, a authctx.Access, id uuid.UUID) (gen.LockConversationRow, error) {
	c, err := r.LockConversation(ctx, gen.LockConversationParams{WorkspaceID: a.WorkspaceID, ID: id})
	if repository.IsNoRows(err) {
		return c, model.ErrConversationNotFound
	}
	if err != nil {
		return c, fmt.Errorf("lock conversation: %w", err)
	}
	if !canSee(a, c.TeamID, c.AssigneeID, c.ChannelType, c.ChannelID) {
		return c, model.ErrConversationNotFound
	}
	return c, nil
}

func denied(p authctx.Permission) error {
	return model.ErrPermissionDenied.WithDetails(apperr.Detail{Field: "permission", Message: p.Key()})
}

// event writes a system event into the thread (F7).
func (s *Service) event(ctx context.Context, r repository.Repository, ws, conversationID uuid.UUID, e model.EventContent) error {
	id, err := newID()
	if err != nil {
		return err
	}
	content, err := encodeContent(model.Content{Event: &e})
	if err != nil {
		return err
	}
	if _, err := r.InsertMessage(ctx, gen.InsertMessageParams{
		ID: id, WorkspaceID: ws, ConversationID: conversationID, Direction: model.DirectionOut, Kind: model.KindEvent,
		Type: model.TypeEvent, Content: content, AuthorID: e.ActorID, Mentions: []uuid.UUID{}, CreatedAt: s.now(),
	}); err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

// Assign gives the chat to userID, or to nobody when nil. It needs
// inbox.close_reassign, except that anyone may take an unassigned chat (AD-10).
func (s *Service) Assign(ctx context.Context, a authctx.Access, id uuid.UUID, userID *uuid.UUID) (model.ConversationDetail, error) {
	var out model.ConversationDetail
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		c, err := lock(ctx, r, a, id)
		if err != nil {
			return err
		}
		selfTake := userID != nil && *userID == a.UserID && c.AssigneeID == nil
		if !a.Can(authctx.PermInboxCloseReassign) && !selfTake {
			return denied(authctx.PermInboxCloseReassign)
		}
		if userID != nil && *userID != a.UserID {
			target, suspended, ok, err := s.deps.Members.ResolveAccess(ctx, a.WorkspaceID, *userID)
			if err != nil {
				return fmt.Errorf("check assignee: %w", err)
			}
			if !ok || suspended || !hasInboxRole(target) || !onNumbers(numbersOf(target), c.ChannelType, c.ChannelID) {
				return model.ErrInvalidAssignee
			}
		}
		if equalIDs(userID, c.AssigneeID) {
			out, err = s.get(ctx, tx, a, id)
			return err
		}
		row, err := r.SetAssignee(ctx, gen.SetAssigneeParams{AssigneeID: userID, WorkspaceID: a.WorkspaceID, ID: id})
		if err != nil {
			return fmt.Errorf("assign: %w", err)
		}
		e := model.EventContent{Action: model.ActionAssigned, ActorID: &a.UserID, TargetID: userID}
		if userID == nil {
			e = model.EventContent{Action: model.ActionUnassigned, ActorID: &a.UserID, TargetID: c.AssigneeID}
		}
		if err := s.event(ctx, r, a.WorkspaceID, id, e); err != nil {
			return err
		}
		if err := signal(ctx, tx, a.WorkspaceID, model.SignalConversation, id, row.TeamID, row.AssigneeID, row.ChannelType, row.ChannelID); err != nil {
			return err
		}
		// The previous assignee hears that the chat left them.
		if c.AssigneeID != nil {
			if err := signal(ctx, tx, a.WorkspaceID, model.SignalConversation, id, row.TeamID, c.AssigneeID, row.ChannelType, row.ChannelID); err != nil {
				return err
			}
		}
		out, err = s.get(ctx, tx, authctx.Access{WorkspaceID: a.WorkspaceID, Owner: true}, id)
		return err
	})
	return out, err
}

func equalIDs(x, y *uuid.UUID) bool {
	if x == nil || y == nil {
		return x == y
	}
	return *x == *y
}

// SetStatus opens or closes a chat, or snoozes it (pending until snoozeUntil).
// It needs inbox.close_reassign, except that a member may close a chat assigned to them (AD-10).
func (s *Service) SetStatus(ctx context.Context, a authctx.Access, id uuid.UUID, status string, snoozeUntil *time.Time) (model.ConversationDetail, error) {
	now := s.now()
	switch {
	case snoozeUntil != nil:
		if !snoozeUntil.After(now) || snoozeUntil.Sub(now) > model.MaxSnooze {
			return model.ConversationDetail{}, model.ErrInvalidInput.WithDetails(apperr.Detail{
				Field: "body.snooze_until", Message: "Snooze until a time in the next 90 days",
			})
		}
		status = model.StatusPending
	case status != model.StatusOpen && status != model.StatusClosed:
		return model.ConversationDetail{}, model.ErrInvalidInput.WithDetails(apperr.Detail{
			Field: "body.status", Message: "Set status to open or closed, or give snooze_until",
		})
	}
	var out model.ConversationDetail
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		c, err := lock(ctx, r, a, id)
		if err != nil {
			return err
		}
		ownClose := status == model.StatusClosed && c.AssigneeID != nil && *c.AssigneeID == a.UserID
		if !a.Can(authctx.PermInboxCloseReassign) && !ownClose {
			return denied(authctx.PermInboxCloseReassign)
		}
		if snoozeUntil == nil && c.Status == status {
			out, err = s.get(ctx, tx, a, id)
			return err
		}
		var until *time.Time
		if snoozeUntil != nil {
			until = ptr(snoozeUntil.UTC())
		}
		row, err := r.SetConversationStatus(ctx, gen.SetConversationStatusParams{
			Status: status, SnoozedUntil: until, WorkspaceID: a.WorkspaceID, ID: id,
		})
		if err != nil {
			return fmt.Errorf("set conversation status: %w", err)
		}
		action := model.ActionReopened
		switch {
		case until != nil:
			action = model.ActionSnoozed
		case status == model.StatusClosed:
			action = model.ActionClosed
		}
		if err := s.event(ctx, r, a.WorkspaceID, id, model.EventContent{Action: action, ActorID: &a.UserID, Until: until}); err != nil {
			return err
		}
		if err := signal(ctx, tx, a.WorkspaceID, model.SignalConversation, id, row.TeamID, row.AssigneeID, row.ChannelType, row.ChannelID); err != nil {
			return err
		}
		out, err = s.get(ctx, tx, a, id)
		return err
	})
	return out, err
}

// MarkRead clears the chat's unread count for everyone (F9). A member who can
// reply also sends the customer a read receipt for their last message, on a
// channel with read receipts.
func (s *Service) MarkRead(ctx context.Context, a authctx.Access, id uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		c, err := lock(ctx, r, a, id)
		if err != nil {
			return err
		}
		if c.UnreadCount == 0 {
			return nil
		}
		row, err := r.ClearUnread(ctx, gen.ClearUnreadParams{WorkspaceID: a.WorkspaceID, ID: id})
		if err != nil {
			return fmt.Errorf("clear unread: %w", err)
		}
		if ch, ok := s.channel(c.ChannelType); ok && ch.Capabilities().ReadReceipts && a.Can(authctx.PermInboxReply) {
			last, err := r.LastInboundExternalID(ctx, gen.LastInboundExternalIDParams{WorkspaceID: a.WorkspaceID, ConversationID: id})
			switch {
			case err == nil && last != nil:
				if _, err := s.queue.InsertTx(ctx, tx, model.MarkReadArgs{
					WorkspaceID: a.WorkspaceID, ChannelType: c.ChannelType, ChannelID: c.ChannelID, ExternalID: *last,
				}, nil); err != nil {
					return fmt.Errorf("enqueue read receipt: %w", err)
				}
			case err != nil && !repository.IsNoRows(err):
				return fmt.Errorf("last inbound message: %w", err)
			}
		}
		return signal(ctx, tx, a.WorkspaceID, model.SignalConversation, id, row.TeamID, row.AssigneeID, row.ChannelType, row.ChannelID)
	})
}

// SendReadReceipt marks the customer's message read on its channel. It is best
// effort: a failure is logged and dropped (AD-5).
func (s *Service) SendReadReceipt(ctx context.Context, args model.MarkReadArgs) {
	ch, ok := s.channel(args.ChannelType)
	if !ok || !ch.Capabilities().ReadReceipts {
		return
	}
	if err := ch.MarkRead(ctx, args.WorkspaceID, args.ChannelID, args.ExternalID); err != nil {
		s.log.Warn("inbox: read receipt failed", "workspace_id", args.WorkspaceID, "external_id", args.ExternalID, "err", err)
	}
}

// unsnoozeBatch is how many chats one unsnooze transaction reopens.
const unsnoozeBatch = 200

// Unsnooze reopens every chat whose snooze has ended and returns how many.
func (s *Service) Unsnooze(ctx context.Context) (int, error) {
	total := 0
	for {
		n := 0
		err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
			r := repository.NewRepository(tx)
			rows, err := r.ReopenSnoozed(ctx, gen.ReopenSnoozedParams{Now: s.now(), BatchSize: unsnoozeBatch})
			if err != nil {
				return fmt.Errorf("reopen snoozed: %w", err)
			}
			for _, c := range rows {
				if err := s.event(ctx, r, c.WorkspaceID, c.ID, model.EventContent{Action: model.ActionUnsnoozed}); err != nil {
					return err
				}
				if err := signal(ctx, tx, c.WorkspaceID, model.SignalConversation, c.ID, c.TeamID, c.AssigneeID, c.ChannelType, c.ChannelID); err != nil {
					return err
				}
			}
			n = len(rows)
			return nil
		})
		if err != nil {
			return total, err
		}
		total += n
		if n < unsnoozeBatch {
			return total, nil
		}
	}
}
