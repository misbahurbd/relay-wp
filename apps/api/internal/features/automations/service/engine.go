package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

const tickPage = 500

// OnInbound queues the reaction to a customer's message in tx, so a message
// stored is a message reacted to, and ingest never waits for it.
func (s *Service) OnInbound(ctx context.Context, tx pgx.Tx, in model.Inbound) error {
	if _, err := s.queue.InsertTx(ctx, tx, model.InboundArgs{Inbound: in}, nil); err != nil {
		return fmt.Errorf("enqueue automations inbound: %w", err)
	}
	return nil
}

// HandleInbound honors STOP, whatever the plan or toggles say, then picks at
// most one reply. Every other candidate leaves a skipped run with its reason.
func (s *Service) HandleInbound(ctx context.Context, in model.Inbound) error {
	if isStop(in.Text) {
		return s.optOut(ctx, in)
	}
	allowed, err := s.deps.Plans.HasAccess(ctx, in.WorkspaceID, model.PlanResource)
	if err != nil || !allowed {
		return err
	}
	items, err := s.enabled(ctx, in.WorkspaceID)
	if err != nil {
		return err
	}
	now := s.now()
	found := candidates(items, in, now)
	if len(found) == 0 {
		return nil
	}
	conv, err := s.deps.Inbox.Conversation(ctx, in.WorkspaceID, in.ConversationID)
	if err != nil {
		return fmt.Errorf("automation conversation: %w", err)
	}
	blocked := ""
	switch {
	case conv.OptIn == "out":
		blocked = model.ReasonOptedOut
	case conv.LastAgentAt != nil && now.Sub(*conv.LastAgentAt) < agentQuietPeriod:
		blocked = model.ReasonAgentActive
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if blocked == "" {
			recent, err := r.CountRecentReplies(ctx, gen.CountRecentRepliesParams{
				WorkspaceID: in.WorkspaceID, ConversationID: &in.ConversationID, Since: now.Add(-time.Hour),
			})
			if err != nil {
				return fmt.Errorf("count recent replies: %w", err)
			}
			if recent >= replyCap {
				blocked = model.ReasonRateLimited
			}
		}
		for i, c := range found {
			reason := blocked
			if reason == "" && i > 0 {
				reason = model.ReasonLowerPriority
			}
			if err := s.addRun(ctx, tx, r, newRun{
				workspaceID: in.WorkspaceID, kind: c.kind, contactID: &in.ContactID, conversation: &in.ConversationID,
				dedupe: c.dedupe, at: now.Add(c.delay), immediate: c.delay == 0,
				context: model.RunContext{Text: personalize(c.text, conv.ContactName)}, skipped: reason,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// newRun is a run to insert. skipped records one that won't send, and why.
type newRun struct {
	workspaceID  uuid.UUID
	kind         model.Kind
	contactID    *uuid.UUID
	conversation *uuid.UUID
	dedupe       string
	at           time.Time
	immediate    bool
	context      model.RunContext
	skipped      string
}

func (s *Service) addRun(ctx context.Context, tx pgx.Tx, r repository.Repository, n newRun) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("new run id: %w", err)
	}
	raw, err := json.Marshal(n.context)
	if err != nil {
		return fmt.Errorf("encode run context: %w", err)
	}
	p := gen.InsertRunParams{
		ID: id, WorkspaceID: n.workspaceID, Kind: string(n.kind), ContactID: n.contactID,
		ConversationID: n.conversation, DedupeKey: n.dedupe, State: model.StateScheduled, Context: raw,
	}
	if n.skipped != "" {
		p.State, p.Reason = model.StateSkipped, &n.skipped
	} else {
		p.NextAt = &n.at
	}
	ids, err := r.InsertRun(ctx, p)
	if err != nil {
		return fmt.Errorf("insert run: %w", err)
	}
	if len(ids) == 0 || n.skipped != "" || !n.immediate {
		return nil
	}
	if _, err := s.queue.InsertTx(ctx, tx, model.ProcessRunArgs{WorkspaceID: n.workspaceID, RunID: id}, nil); err != nil {
		return fmt.Errorf("enqueue run: %w", err)
	}
	return nil
}

func (s *Service) optOut(ctx context.Context, in model.Inbound) error {
	changed, err := s.deps.Contacts.SetOptOut(ctx, in.WorkspaceID, in.ContactID, model.OptOutSource)
	if err != nil {
		return fmt.Errorf("opt out: %w", err)
	}
	reason := model.ReasonOptedOut
	if err := repository.NewRepository(s.pool).CancelContactRuns(ctx, gen.CancelContactRunsParams{
		WorkspaceID: in.WorkspaceID, ContactID: &in.ContactID, Reason: &reason,
	}); err != nil {
		return fmt.Errorf("cancel runs: %w", err)
	}
	if !changed {
		return nil
	}
	return s.deps.Inbox.NoteOptOut(ctx, in.WorkspaceID, in.ConversationID)
}

// enabled returns the kinds that are on, with their settings.
func (s *Service) enabled(ctx context.Context, workspaceID uuid.UUID) (map[model.Kind]model.Settings, error) {
	rows, err := repository.NewRepository(s.pool).ListAutomations(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list automations: %w", err)
	}
	out := make(map[model.Kind]model.Settings, len(rows))
	for _, r := range rows {
		if !r.Enabled {
			continue
		}
		k := model.Kind(r.Kind)
		if out[k], err = model.Decode(k, r.Settings); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ProcessRun fires a due run, checking everything again because the world may
// have moved since it was scheduled. The message and the run's new state commit
// together, so running this twice sends once.
func (s *Service) ProcessRun(ctx context.Context, workspaceID, runID uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		run, err := r.ClaimRun(ctx, gen.ClaimRunParams{WorkspaceID: workspaceID, ID: runID, Now: s.now()})
		if repository.IsNoRows(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("claim run: %w", err)
		}
		state, reason, err := s.fire(ctx, tx, r, workspaceID, run)
		if err != nil {
			return err
		}
		return r.FinishRun(ctx, gen.FinishRunParams{WorkspaceID: workspaceID, ID: runID, State: state, Reason: reason})
	})
}

func (s *Service) fire(ctx context.Context, tx pgx.Tx, r repository.Repository, workspaceID uuid.UUID, run gen.ClaimRunRow) (string, *string, error) {
	end := func(state, reason string) (string, *string, error) { return state, &reason, nil }
	row, err := r.GetAutomation(ctx, gen.GetAutomationParams{WorkspaceID: workspaceID, Kind: run.Kind})
	if err != nil && !repository.IsNoRows(err) {
		return "", nil, fmt.Errorf("get automation: %w", err)
	}
	if err != nil || !row.Enabled {
		return end(model.StateCancelled, model.ReasonPaused)
	}
	allowed, err := s.deps.Plans.HasAccess(ctx, workspaceID, model.PlanResource)
	if err != nil {
		return "", nil, err
	}
	if !allowed {
		return end(model.StateCancelled, model.ReasonPlan)
	}
	if run.ConversationID != nil {
		conv, err := s.deps.Inbox.Conversation(ctx, workspaceID, *run.ConversationID)
		if err != nil {
			return "", nil, fmt.Errorf("automation conversation: %w", err)
		}
		if conv.LastAgentAt != nil && conv.LastAgentAt.After(run.CreatedAt) {
			return end(model.StateSkipped, model.ReasonAgentActive)
		}
	}
	var c model.RunContext
	if err := json.Unmarshal(run.Context, &c); err != nil {
		return "", nil, fmt.Errorf("decode run context: %w", err)
	}
	err = s.deps.Inbox.SendAutomated(ctx, tx, model.AutomatedSend{
		WorkspaceID: workspaceID, ConversationID: run.ConversationID, ContactID: run.ContactID, RunID: run.ID,
		Text: c.Text, TemplateID: c.TemplateID, Params: c.Params,
	})
	switch {
	case errors.Is(err, model.ErrOptedOut):
		return end(model.StateSkipped, model.ReasonOptedOut)
	case errors.Is(err, model.ErrWindowClosed):
		return end(model.StateSkipped, model.ReasonWindowClosed)
	case errors.Is(err, model.ErrTemplateUnavailable):
		return end(model.StateFailed, model.ReasonTemplateUnavailable)
	case errors.Is(err, model.ErrMissingVariable):
		return end(model.StateFailed, model.ReasonMissingVariable)
	case errors.Is(err, model.ErrNoChannel):
		return end(model.StateFailed, model.ReasonNoChannel)
	case err != nil:
		return "", nil, fmt.Errorf("send automated: %w", err)
	}
	return model.StateDone, nil, nil
}

// Tick queues every due run. Creation already queues immediate ones, so this
// finds delayed runs and whatever a crash left behind.
func (s *Service) Tick(ctx context.Context) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		due, err := repository.NewRepository(tx).DueRuns(ctx, gen.DueRunsParams{Now: s.now(), PageSize: tickPage})
		if err != nil {
			return fmt.Errorf("due runs: %w", err)
		}
		for _, d := range due {
			args := model.ProcessRunArgs{WorkspaceID: d.WorkspaceID, RunID: d.ID, Step: d.Step}
			if _, err := s.queue.InsertTx(ctx, tx, args, nil); err != nil {
				return fmt.Errorf("enqueue run: %w", err)
			}
		}
		return nil
	})
}
