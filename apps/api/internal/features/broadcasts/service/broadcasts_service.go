package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/broadcasts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/broadcasts/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// Limits on what a person can enter.
const (
	MaxPageSize     = 100
	maxNameLength   = 120
	maxTags         = 20
	maxSegments     = 10
	maxVariableText = 1024
	// maxScheduleAhead is how far ahead a broadcast can be scheduled.
	maxScheduleAhead = 90 * 24 * time.Hour
)

// List returns the workspace's broadcasts, newest first, and the cursor for the next page.
func (s *Service) List(ctx context.Context, workspaceID uuid.UUID, f model.ListFilter) ([]model.Broadcast, *uuid.UUID, error) {
	limit := f.Limit
	if limit < 1 || limit > MaxPageSize {
		limit = 25
	}
	var status *string
	if f.Status != nil {
		st := string(*f.Status)
		status = &st
	}
	rows, err := repository.NewRepository(s.pool).ListBroadcasts(ctx, gen.ListBroadcastsParams{
		WorkspaceID: workspaceID, Status: status, Query: escapeLike(f.Query), BeforeID: f.BeforeID, Lim: int32(limit + 1), //nolint:gosec // capped above
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list broadcasts: %w", err)
	}
	var next *uuid.UUID
	if len(rows) > limit {
		rows = rows[:limit]
		next = &rows[limit-1].ID
	}
	out := make([]model.Broadcast, len(rows))
	for i, r := range rows {
		out[i] = model.Broadcast{
			ID: r.ID, Name: r.Name, TemplateID: r.TemplateID, Status: model.Status(r.Status),
			ScheduledAt: r.ScheduledAt, StartedAt: r.StartedAt, CompletedAt: r.CompletedAt,
			RecipientsTotal: int(r.RecipientsTotal), CreatedAt: r.CreatedAt,
			Stats: model.Stats{Sent: int(r.Sent), Delivered: int(r.Delivered), Read: int(r.Read), Failed: int(r.Failed)},
		}
		if r.TemplateName != nil {
			out[i].Template = &model.TemplateRef{Name: *r.TemplateName}
		}
	}
	return out, next, nil
}

// Get returns one broadcast with its delivery counts and why recipients missed it.
func (s *Service) Get(ctx context.Context, workspaceID, id uuid.UUID) (model.Broadcast, error) {
	repo := repository.NewRepository(s.pool)
	b, err := s.get(ctx, repo, workspaceID, id)
	if err != nil {
		return model.Broadcast{}, err
	}
	st, err := repo.BroadcastStats(ctx, gen.BroadcastStatsParams{WorkspaceID: workspaceID, BroadcastID: id})
	if err != nil {
		return model.Broadcast{}, fmt.Errorf("broadcast stats: %w", err)
	}
	b.Stats = toStats(st)
	reasons, err := repo.FailureReasons(ctx, gen.FailureReasonsParams{WorkspaceID: workspaceID, BroadcastID: id})
	if err != nil {
		return model.Broadcast{}, fmt.Errorf("failure reasons: %w", err)
	}
	b.FailureBreakdown = make([]model.FailureCount, len(reasons))
	for i, r := range reasons {
		b.FailureBreakdown[i] = model.FailureCount{Reason: r.Reason, Recipients: int(r.Recipients)}
	}
	return b, nil
}

func (s *Service) get(ctx context.Context, repo repository.Repository, workspaceID, id uuid.UUID) (model.Broadcast, error) {
	r, err := repo.GetBroadcast(ctx, gen.GetBroadcastParams{WorkspaceID: workspaceID, ID: id})
	if repository.IsNoRows(err) {
		return model.Broadcast{}, model.ErrNotFound
	}
	if err != nil {
		return model.Broadcast{}, fmt.Errorf("get broadcast: %w", err)
	}
	b := model.Broadcast{
		ID: r.ID, Name: r.Name, NumberID: r.NumberID, TemplateID: r.TemplateID, Status: model.Status(r.Status),
		ScheduledAt: r.ScheduledAt, StartedAt: r.StartedAt, CompletedAt: r.CompletedAt,
		RecipientsTotal: int(r.RecipientsTotal), FailureReason: r.FailureReason, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if r.TemplateName != nil {
		b.Template = &model.TemplateRef{Name: *r.TemplateName, Language: deref(r.TemplateLanguage), Category: deref(r.TemplateCategory)}
	}
	if b.Audience, b.Variables, err = decode(r.Audience, r.Variables); err != nil {
		return model.Broadcast{}, err
	}
	return b, nil
}

// Create saves a new draft.
func (s *Service) Create(ctx context.Context, workspaceID, userID uuid.UUID, in model.Input) (model.Broadcast, error) {
	in, err := clean(in)
	if err != nil {
		return model.Broadcast{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return model.Broadcast{}, fmt.Errorf("new id: %w", err)
	}
	audience, variables, err := encode(in)
	if err != nil {
		return model.Broadcast{}, err
	}
	var createdBy *uuid.UUID
	if userID != uuid.Nil {
		createdBy = &userID
	}
	repo := repository.NewRepository(s.pool)
	if err := repo.InsertBroadcast(ctx, gen.InsertBroadcastParams{
		ID: id, WorkspaceID: workspaceID, Name: in.Name, NumberID: in.NumberID, TemplateID: in.TemplateID,
		Audience: audience, Variables: variables, CreatedBy: createdBy,
	}); err != nil {
		return model.Broadcast{}, fmt.Errorf("insert broadcast: %w", err)
	}
	return s.get(ctx, repo, workspaceID, id)
}

// Update replaces a draft or scheduled broadcast's content.
func (s *Service) Update(ctx context.Context, workspaceID, id uuid.UUID, in model.Input) (model.Broadcast, error) {
	in, err := clean(in)
	if err != nil {
		return model.Broadcast{}, err
	}
	audience, variables, err := encode(in)
	if err != nil {
		return model.Broadcast{}, err
	}
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		repo := repository.NewRepository(tx)
		b, err := lock(ctx, repo, workspaceID, id)
		if err != nil {
			return err
		}
		if !model.Status(b.Status).Editable() {
			return model.ErrNotEditable
		}
		if err := repo.UpdateBroadcast(ctx, gen.UpdateBroadcastParams{
			WorkspaceID: workspaceID, ID: id, Name: in.Name, NumberID: in.NumberID, TemplateID: in.TemplateID,
			Audience: audience, Variables: variables,
		}); err != nil {
			return fmt.Errorf("update broadcast: %w", err)
		}
		return nil
	})
	if err != nil {
		return model.Broadcast{}, err
	}
	return s.get(ctx, repository.NewRepository(s.pool), workspaceID, id)
}

// Delete removes a draft. A broadcast that was sent or scheduled stays for its report.
func (s *Service) Delete(ctx context.Context, workspaceID, id uuid.UUID) error {
	repo := repository.NewRepository(s.pool)
	n, err := repo.DeleteDraft(ctx, gen.DeleteDraftParams{WorkspaceID: workspaceID, ID: id})
	if err != nil {
		return fmt.Errorf("delete broadcast: %w", err)
	}
	if n == 0 {
		if _, err := s.get(ctx, repo, workspaceID, id); err != nil {
			return err
		}
		return model.ErrNotEditable
	}
	return nil
}

// Duplicate copies a broadcast's content into a new draft.
func (s *Service) Duplicate(ctx context.Context, workspaceID, userID, id uuid.UUID) (model.Broadcast, error) {
	b, err := s.get(ctx, repository.NewRepository(s.pool), workspaceID, id)
	if err != nil {
		return model.Broadcast{}, err
	}
	name := b.Name + " (copy)"
	if len([]rune(name)) > maxNameLength {
		name = string([]rune(name)[:maxNameLength])
	}
	return s.Create(ctx, workspaceID, userID, model.Input{
		Name: name, NumberID: b.NumberID, TemplateID: b.TemplateID, Audience: b.Audience, Variables: b.Variables,
	})
}

// PreviewAudience counts who an audience reaches, and the number's daily tier.
func (s *Service) PreviewAudience(ctx context.Context, workspaceID uuid.UUID, a model.Audience, numberID *uuid.UUID) (model.AudienceCount, error) {
	a.TagIDs, a.SegmentIDs = dedupe(a.TagIDs), dedupe(a.SegmentIDs)
	if len(a.SegmentIDs) > maxSegments {
		return model.AudienceCount{}, model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "segment_ids", Message: fmt.Sprintf("Choose at most %d segments", maxSegments)})
	}
	if len(a.TagIDs) > maxTags {
		return model.AudienceCount{}, model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "tag_ids", Message: fmt.Sprintf("Choose at most %d tags", maxTags)})
	}
	c, err := s.countAudience(ctx, workspaceID, a)
	if err != nil {
		return model.AudienceCount{}, err
	}
	if numberID != nil {
		n, err := s.deps.Numbers.Number(ctx, workspaceID, *numberID)
		if err != nil && !errors.Is(err, model.ErrNumberNotConnected) {
			return model.AudienceCount{}, err
		}
		c.TierLimit = n.TierLimit
	}
	return c, nil
}

func (s *Service) countAudience(ctx context.Context, workspaceID uuid.UUID, a model.Audience) (model.AudienceCount, error) {
	if a.Empty() {
		return model.AudienceCount{}, nil
	}
	members, err := s.segmentMembers(ctx, workspaceID, a)
	if err != nil {
		return model.AudienceCount{}, err
	}
	r, err := repository.NewRepository(s.pool).AudienceCount(ctx, gen.AudienceCountParams{
		WorkspaceID: workspaceID, Everyone: a.All, TagIds: tagIDs(a), ContactIds: members,
		RecentSince: s.now().Add(-model.RecentMarketingWindow),
	})
	if err != nil {
		return model.AudienceCount{}, fmt.Errorf("count audience: %w", err)
	}
	c := model.AudienceCount{Included: int(r.OptedIn), LeftOut: int(r.Matched - r.OptedIn), Recent: int(r.Recent)}
	if a.SkipRecentMarketing {
		c.Included -= c.Recent
	}
	return c, nil
}

// Send starts a draft now, or at scheduledAt. Called on a scheduled broadcast it
// moves the time, or sends it now, without using another of the plan's broadcasts.
func (s *Service) Send(ctx context.Context, workspaceID, id uuid.UUID, scheduledAt *time.Time) (model.Broadcast, error) {
	now := s.now()
	if scheduledAt != nil {
		at := scheduledAt.UTC()
		switch {
		case at.Before(now.Add(-time.Minute)):
			return model.Broadcast{}, model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "scheduled_at", Message: "Pick a time in the future"})
		case at.After(now.Add(maxScheduleAhead)):
			return model.Broadcast{}, model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "scheduled_at", Message: "Schedule at most 90 days ahead"})
		case !at.After(now):
			scheduledAt = nil // "now" or a moment ago: send it now
		default:
			scheduledAt = &at
		}
	}
	repo := repository.NewRepository(s.pool)
	b, err := s.get(ctx, repo, workspaceID, id)
	if err != nil {
		return model.Broadcast{}, err
	}
	if !b.Status.Editable() {
		return model.Broadcast{}, model.ErrNotEditable
	}
	if err := s.checkReady(ctx, workspaceID, b); err != nil {
		return model.Broadcast{}, err
	}

	// A draft uses one of the plan's broadcasts this month, before the
	// transaction; it is given back if the transaction fails (ADR 0008).
	var period *time.Time
	if b.Status == model.StatusDraft {
		p, ok, err := s.deps.Usage.Consume(ctx, workspaceID, model.PlanResource, 1)
		if err != nil {
			return model.Broadcast{}, err
		}
		if !ok {
			return model.Broadcast{}, model.ErrPlanLimit
		}
		period = &p
	}
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error { return s.send(ctx, tx, workspaceID, id, b.Status, scheduledAt, period) })
	if err != nil {
		if period != nil {
			if rerr := s.deps.Usage.Release(ctx, workspaceID, model.PlanResource, *period, 1); rerr != nil {
				s.log.ErrorContext(ctx, "release broadcast usage", "workspace_id", workspaceID, "broadcast_id", id, "error", rerr)
			}
		}
		return model.Broadcast{}, err
	}
	return s.get(ctx, repo, workspaceID, id)
}

func (s *Service) send(ctx context.Context, tx pgx.Tx, workspaceID, id uuid.UUID, was model.Status, scheduledAt, period *time.Time) error {
	repo := repository.NewRepository(tx)
	b, err := lock(ctx, repo, workspaceID, id)
	if err != nil {
		return err
	}
	if model.Status(b.Status) != was {
		return model.ErrNotEditable // someone else sent or cancelled it meanwhile
	}
	usage := b.UsagePeriod
	if period != nil {
		usage = pgtype.Date{Time: *period, Valid: true}
	}
	start := model.StartArgs{WorkspaceID: workspaceID, BroadcastID: id}
	if scheduledAt != nil {
		if err := repo.ScheduleBroadcast(ctx, gen.ScheduleBroadcastParams{
			WorkspaceID: workspaceID, ID: id, ScheduledAt: *scheduledAt, UsagePeriod: usage,
		}); err != nil {
			return fmt.Errorf("schedule broadcast: %w", err)
		}
		_, err = s.queue.InsertTx(ctx, tx, start, &river.InsertOpts{ScheduledAt: *scheduledAt})
	} else {
		if err := repo.SendBroadcastNow(ctx, gen.SendBroadcastNowParams{
			WorkspaceID: workspaceID, ID: id, Now: s.now(), UsagePeriod: usage,
		}); err != nil {
			return fmt.Errorf("send broadcast: %w", err)
		}
		_, err = s.queue.InsertTx(ctx, tx, start, nil)
	}
	if err != nil {
		return fmt.Errorf("enqueue broadcast start: %w", err)
	}
	return nil
}

// checkReady says what a broadcast still needs before it can go.
func (s *Service) checkReady(ctx context.Context, workspaceID uuid.UUID, b model.Broadcast) error {
	var missing []apperr.Detail
	add := func(field, msg string) { missing = append(missing, apperr.Detail{Field: field, Message: msg}) }
	if b.NumberID == nil {
		add("number_id", "Choose the WhatsApp number to send from")
	} else if n, err := s.deps.Numbers.Number(ctx, workspaceID, *b.NumberID); err != nil {
		return err
	} else if !n.Connected {
		return model.ErrNumberNotConnected
	}
	if b.TemplateID == nil {
		add("template_id", "Choose a template")
	} else {
		t, err := s.deps.Templates.ApprovedTemplate(ctx, workspaceID, *b.TemplateID)
		if err != nil {
			return err
		}
		missing = append(missing, checkVariables(t, b.Variables)...)
	}
	if len(missing) > 0 {
		return model.ErrNotSendable.WithDetails(missing...)
	}
	c, err := s.countAudience(ctx, workspaceID, b.Audience)
	if err != nil {
		return err
	}
	if c.Included == 0 {
		return model.ErrEmptyAudience
	}
	return nil
}

// checkVariables says which of the template's placeholders have no value.
// A contact field needs a fallback, since not every contact has it.
func checkVariables(t model.Template, vs model.Variables) []apperr.Detail {
	var missing []apperr.Detail
	check := func(field string, v *model.Variable) {
		switch {
		case v == nil || (v.Source == model.SourceText && strings.TrimSpace(v.Value) == ""):
			missing = append(missing, apperr.Detail{Field: field, Message: "Fill in this variable"})
		case v.Source != model.SourceText && v.Source != model.SourcePhone && strings.TrimSpace(v.Fallback) == "":
			missing = append(missing, apperr.Detail{Field: field + ".fallback", Message: "Add a fallback for contacts without this field"})
		}
	}
	if t.HeaderVariable {
		check("variables.header", vs.Header)
	}
	for i := range t.BodyVariables {
		var v *model.Variable
		if i < len(vs.Body) {
			v = &vs.Body[i]
		}
		check(fmt.Sprintf("variables.body[%d]", i), v)
	}
	return missing
}

// Cancel stops a scheduled or sending broadcast. Messages already queued still
// go. A broadcast that queued nothing gives its plan unit back.
func (s *Service) Cancel(ctx context.Context, workspaceID, id uuid.UUID) (model.Broadcast, error) {
	var period pgtype.Date
	var queued int32
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		repo := repository.NewRepository(tx)
		if _, err := lock(ctx, repo, workspaceID, id); err != nil {
			return err
		}
		p, err := repo.CancelBroadcast(ctx, gen.CancelBroadcastParams{WorkspaceID: workspaceID, ID: id, Now: s.now()})
		if repository.IsNoRows(err) {
			return model.ErrNotCancellable
		}
		if err != nil {
			return fmt.Errorf("cancel broadcast: %w", err)
		}
		period = p
		reason := model.ReasonCancelled
		if err := repo.SkipPending(ctx, gen.SkipPendingParams{WorkspaceID: workspaceID, BroadcastID: id, Reason: &reason}); err != nil {
			return fmt.Errorf("skip pending recipients: %w", err)
		}
		if queued, err = repo.CountQueued(ctx, gen.CountQueuedParams{WorkspaceID: workspaceID, BroadcastID: id}); err != nil {
			return fmt.Errorf("count queued: %w", err)
		}
		return nil
	})
	if err != nil {
		return model.Broadcast{}, err
	}
	if queued == 0 && period.Valid {
		if err := s.deps.Usage.Release(ctx, workspaceID, model.PlanResource, period.Time, 1); err != nil {
			return model.Broadcast{}, err
		}
	}
	return s.Get(ctx, workspaceID, id)
}

// Recipients lists who a broadcast went to, by contact, with the next page's cursor.
func (s *Service) Recipients(ctx context.Context, workspaceID, id uuid.UUID, filter *string, after *uuid.UUID, limit int) ([]model.Recipient, *uuid.UUID, error) {
	repo := repository.NewRepository(s.pool)
	if _, err := s.get(ctx, repo, workspaceID, id); err != nil {
		return nil, nil, err
	}
	if limit < 1 || limit > MaxPageSize {
		limit = 50
	}
	rows, err := repo.ListRecipients(ctx, gen.ListRecipientsParams{
		WorkspaceID: workspaceID, BroadcastID: id, AfterID: after, Filter: filter, Lim: int32(limit + 1), //nolint:gosec // capped above
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list recipients: %w", err)
	}
	var next *uuid.UUID
	if len(rows) > limit {
		rows = rows[:limit]
		next = &rows[limit-1].ContactID
	}
	out := make([]model.Recipient, len(rows))
	for i, r := range rows {
		out[i] = model.Recipient{
			ContactID: r.ContactID, Name: r.Name, Phone: r.Phone, State: r.State, Reason: r.Reason, QueuedAt: r.QueuedAt,
			MessageStatus: r.MessageStatus, StatusAt: r.StatusAt, Error: r.ErrorTitle, Replied: r.Replied,
		}
	}
	return out, next, nil
}

// clean trims the input and refuses what could never be valid. A draft may be
// incomplete; Send checks the rest.
func clean(in model.Input) (model.Input, error) {
	var bad []apperr.Detail
	add := func(field, msg string) { bad = append(bad, apperr.Detail{Field: field, Message: msg}) }
	in.Name = strings.TrimSpace(in.Name)
	switch n := len([]rune(in.Name)); {
	case n == 0:
		add("name", "Name the broadcast")
	case n > maxNameLength:
		add("name", fmt.Sprintf("Use at most %d characters", maxNameLength))
	}
	if len(in.Audience.TagIDs) > maxTags {
		add("audience.tag_ids", fmt.Sprintf("Choose at most %d tags", maxTags))
	}
	if len(in.Audience.SegmentIDs) > maxSegments {
		add("audience.segment_ids", fmt.Sprintf("Choose at most %d segments", maxSegments))
	}
	in.Audience.TagIDs = dedupe(in.Audience.TagIDs)
	in.Audience.SegmentIDs = dedupe(in.Audience.SegmentIDs)
	checkVar := func(field string, v *model.Variable) {
		v.Value, v.Fallback = strings.TrimSpace(v.Value), strings.TrimSpace(v.Fallback)
		switch v.Source {
		case model.SourceText, model.SourceName, model.SourcePhone:
		case model.SourceAttribute:
			if v.Value == "" {
				add(field+".value", "Choose the contact field")
			}
		default:
			add(field+".source", "Use text, name, phone or attribute")
		}
		if len(v.Value) > maxVariableText || len(v.Fallback) > maxVariableText {
			add(field, fmt.Sprintf("Use at most %d characters", maxVariableText))
		}
	}
	if in.Variables.Header != nil {
		checkVar("variables.header", in.Variables.Header)
	}
	for i := range in.Variables.Body {
		checkVar(fmt.Sprintf("variables.body[%d]", i), &in.Variables.Body[i])
	}
	if len(bad) > 0 {
		return in, model.ErrInvalidInput.WithDetails(bad...)
	}
	return in, nil
}

func lock(ctx context.Context, repo repository.Repository, workspaceID, id uuid.UUID) (gen.LockBroadcastRow, error) {
	b, err := repo.LockBroadcast(ctx, gen.LockBroadcastParams{WorkspaceID: workspaceID, ID: id})
	if repository.IsNoRows(err) {
		return b, model.ErrNotFound
	}
	if err != nil {
		return b, fmt.Errorf("lock broadcast: %w", err)
	}
	return b, nil
}

func encode(in model.Input) (audience, variables []byte, err error) {
	if in.Audience.TagIDs == nil {
		in.Audience.TagIDs = []uuid.UUID{}
	}
	if in.Variables.Body == nil {
		in.Variables.Body = []model.Variable{}
	}
	if audience, err = json.Marshal(in.Audience); err != nil {
		return nil, nil, fmt.Errorf("encode audience: %w", err)
	}
	if variables, err = json.Marshal(in.Variables); err != nil {
		return nil, nil, fmt.Errorf("encode variables: %w", err)
	}
	return audience, variables, nil
}

func decode(audience, variables []byte) (model.Audience, model.Variables, error) {
	var a model.Audience
	var v model.Variables
	if err := json.Unmarshal(audience, &a); err != nil {
		return a, v, fmt.Errorf("decode audience: %w", err)
	}
	if err := json.Unmarshal(variables, &v); err != nil {
		return a, v, fmt.Errorf("decode variables: %w", err)
	}
	if a.TagIDs == nil {
		a.TagIDs = []uuid.UUID{}
	}
	if v.Body == nil {
		v.Body = []model.Variable{}
	}
	return a, v, nil
}

func toStats(r gen.BroadcastStatsRow) model.Stats {
	return model.Stats{
		Pending: int(r.Pending), Queued: int(r.Queued), Skipped: int(r.Skipped), Failed: int(r.Failed),
		Sent: int(r.Sent), Delivered: int(r.Delivered), Read: int(r.Read), Replied: int(r.Replied),
	}
}

// segmentMembers is everyone in the audience's segments; none when it sends to all.
func (s *Service) segmentMembers(ctx context.Context, workspaceID uuid.UUID, a model.Audience) ([]uuid.UUID, error) {
	if a.All || len(a.SegmentIDs) == 0 || s.deps.Segments == nil {
		return []uuid.UUID{}, nil
	}
	return s.deps.Segments.Members(ctx, workspaceID, a.SegmentIDs)
}

func tagIDs(a model.Audience) []uuid.UUID {
	if a.TagIDs == nil {
		return []uuid.UUID{}
	}
	return a.TagIDs
}

func dedupe(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// escapeLike makes q match literally inside ILIKE '%…%'.
func escapeLike(q *string) *string {
	if q == nil || strings.TrimSpace(*q) == "" {
		return nil
	}
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.TrimSpace(*q))
	return &r
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
