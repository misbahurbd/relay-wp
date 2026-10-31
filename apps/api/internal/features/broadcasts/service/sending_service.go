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
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/broadcasts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/broadcasts/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// Timings of the tick (AD-7).
const (
	// startGrace is how long a send waits for its start job before the tick queues another.
	startGrace = time.Minute
	// stallAfter fails a broadcast still sending this long after it started.
	stallAfter = 24 * time.Hour
)

// Start takes a due broadcast's audience and queues its first batch. It runs
// once per broadcast: a second start, or a cancelled broadcast, does nothing.
func (s *Service) Start(ctx context.Context, workspaceID, id uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		repo := repository.NewRepository(tx)
		now := s.now()
		raw, err := repo.StartBroadcast(ctx, gen.StartBroadcastParams{WorkspaceID: workspaceID, ID: id, Now: now})
		if repository.IsNoRows(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("start broadcast: %w", err)
		}
		var a model.Audience
		if err := json.Unmarshal(raw, &a); err != nil {
			return fmt.Errorf("decode audience: %w", err)
		}
		members, err := s.segmentMembers(ctx, workspaceID, a)
		if errors.Is(err, model.ErrSegmentNotFound) {
			// A segment was deleted while this waited: it can't be sent as planned.
			reason := model.ReasonSegmentDeleted
			_, ferr := repo.FailBroadcast(ctx, gen.FailBroadcastParams{WorkspaceID: workspaceID, ID: id, Reason: &reason, Now: now})
			return ferr
		}
		if err != nil {
			return err
		}
		var n int64
		if !a.Empty() {
			n, err = repo.SnapshotRecipients(ctx, gen.SnapshotRecipientsParams{
				WorkspaceID: workspaceID, BroadcastID: id, Everyone: a.All, TagIds: tagIDs(a), ContactIds: members,
				SkipRecent: a.SkipRecentMarketing, RecentSince: now.Add(-model.RecentMarketingWindow),
			})
		}
		if err != nil {
			return fmt.Errorf("snapshot recipients: %w", err)
		}
		if err := repo.SetSnapshot(ctx, gen.SetSnapshotParams{
			WorkspaceID: workspaceID, ID: id, RecipientsTotal: int32(min(n, 1<<31-1)), Now: now, //nolint:gosec // capped
		}); err != nil {
			return fmt.Errorf("set snapshot: %w", err)
		}
		s.log.InfoContext(ctx, "broadcast started", "workspace_id", workspaceID, "broadcast_id", id, "recipients", n)
		if n == 0 {
			return nil // the tick completes it
		}
		if _, err := s.queue.InsertTx(ctx, tx, model.BatchArgs{WorkspaceID: workspaceID, BroadcastID: id}, nil); err != nil {
			return fmt.Errorf("enqueue first batch: %w", err)
		}
		return nil
	})
}

// Batch queues the next model.BatchSize pending recipients' messages and, if
// there may be more, the next batch. Each recipient's message and its new
// state commit together, so a retried batch never sends twice (AD-2).
func (s *Service) Batch(ctx context.Context, workspaceID, id uuid.UUID, seq int) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		repo := repository.NewRepository(tx)
		b, err := repo.BroadcastStatus(ctx, gen.BroadcastStatusParams{WorkspaceID: workspaceID, ID: id})
		if repository.IsNoRows(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("broadcast status: %w", err)
		}
		if model.Status(b.Status) != model.StatusSending || b.NumberID == nil || b.TemplateID == nil {
			return nil // cancelled, failed or finished meanwhile
		}
		var vars model.Variables
		if err := json.Unmarshal(b.Variables, &vars); err != nil {
			return fmt.Errorf("decode variables: %w", err)
		}
		rows, err := repo.ClaimRecipients(ctx, gen.ClaimRecipientsParams{WorkspaceID: workspaceID, BroadcastID: id, Lim: model.BatchSize})
		if err != nil {
			return fmt.Errorf("claim recipients: %w", err)
		}
		var queued, skipped, failed int
		for _, r := range rows {
			attrs := map[string]string{}
			if len(r.Attributes) > 0 {
				if err := json.Unmarshal(r.Attributes, &attrs); err != nil {
					return fmt.Errorf("decode contact attributes: %w", err)
				}
			}
			header, body := vars.Render(model.Contact{Name: r.Name, Phone: r.Phone, Attributes: attrs})
			msgID, err := s.deps.Inbox.SendBroadcast(ctx, tx, model.Send{
				WorkspaceID: workspaceID, ContactID: r.ContactID, NumberID: *b.NumberID, TemplateID: *b.TemplateID,
				HeaderText: header, Body: body,
			})
			state, reason := outcome(err)
			switch state {
			case "":
				return err // anything else: retry the whole batch
			case model.RecipientQueued:
				queued++
				err = repo.SetRecipientQueued(ctx, gen.SetRecipientQueuedParams{
					WorkspaceID: workspaceID, BroadcastID: id, ContactID: r.ContactID, MessageID: &msgID, Now: s.now(),
				})
			default:
				if state == model.RecipientSkipped {
					skipped++
				} else {
					failed++
				}
				err = repo.SetRecipientOutcome(ctx, gen.SetRecipientOutcomeParams{
					WorkspaceID: workspaceID, BroadcastID: id, ContactID: r.ContactID, State: state, Reason: &reason,
				})
			}
			if err != nil {
				return fmt.Errorf("record recipient: %w", err)
			}
		}
		s.log.InfoContext(ctx, "broadcast batch", "workspace_id", workspaceID, "broadcast_id", id, "seq", seq,
			"queued", queued, "skipped", skipped, "failed", failed)
		if len(rows) < model.BatchSize {
			return nil
		}
		if _, err := s.queue.InsertTx(ctx, tx, model.BatchArgs{WorkspaceID: workspaceID, BroadcastID: id, Seq: seq + 1}, nil); err != nil {
			return fmt.Errorf("enqueue next batch: %w", err)
		}
		return nil
	})
}

// outcome maps the inbox's answer to a recipient state; "" means retry.
func outcome(err error) (state, reason string) {
	switch {
	case err == nil:
		return model.RecipientQueued, ""
	case errors.Is(err, model.ErrSendOptedOut):
		return model.RecipientSkipped, model.ReasonOptedOut
	case errors.Is(err, model.ErrSendTemplate):
		return model.RecipientFailed, model.ReasonTemplateUnusable
	case errors.Is(err, model.ErrSendMissingVariable):
		return model.RecipientFailed, model.ReasonMissingVariable
	case errors.Is(err, model.ErrSendNoChannel):
		return model.RecipientFailed, model.ReasonNoChannel
	}
	return "", ""
}

// Tick starts due broadcasts, completes the ones whose messages have all gone
// and fails the ones stuck for a day. It runs every minute, on the leader.
func (s *Service) Tick(ctx context.Context) error {
	repo := repository.NewRepository(s.pool)
	now := s.now()
	due, err := repo.DueToStart(ctx, gen.DueToStartParams{Now: now, Stale: now.Add(-startGrace)})
	if err != nil {
		return fmt.Errorf("due broadcasts: %w", err)
	}
	for _, d := range due {
		if err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
			_, err := s.queue.InsertTx(ctx, tx, model.StartArgs{WorkspaceID: d.WorkspaceID, BroadcastID: d.ID}, nil)
			return err
		}); err != nil {
			return fmt.Errorf("enqueue broadcast start: %w", err)
		}
	}
	done, err := repo.FinishedSending(ctx)
	if err != nil {
		return fmt.Errorf("finished broadcasts: %w", err)
	}
	for _, d := range done {
		if err := s.complete(ctx, d.WorkspaceID, d.ID); err != nil {
			return err
		}
	}
	stalled, err := repo.StalledSending(ctx, now.Add(-stallAfter))
	if err != nil {
		return fmt.Errorf("stalled broadcasts: %w", err)
	}
	for _, d := range stalled {
		if err := s.fail(ctx, d.WorkspaceID, d.ID, model.ReasonStalled); err != nil {
			return err
		}
	}
	return nil
}

// complete marks a broadcast completed and tells integrations, in one
// transaction; one that reached a customer activates the workspace.
func (s *Service) complete(ctx context.Context, workspaceID, id uuid.UUID) error {
	var reached bool
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		repo := repository.NewRepository(tx)
		now := s.now()
		b, err := repo.CompleteBroadcast(ctx, gen.CompleteBroadcastParams{WorkspaceID: workspaceID, ID: id, Now: now})
		if repository.IsNoRows(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("complete broadcast: %w", err)
		}
		st, err := repo.BroadcastStats(ctx, gen.BroadcastStatsParams{WorkspaceID: workspaceID, BroadcastID: id})
		if err != nil {
			return fmt.Errorf("broadcast stats: %w", err)
		}
		reached = st.Sent > 0
		s.log.InfoContext(ctx, "broadcast completed", "workspace_id", workspaceID, "broadcast_id", id,
			"recipients", b.RecipientsTotal, "sent", st.Sent, "failed", st.Failed, "skipped", st.Skipped)
		if s.deps.Events == nil {
			return nil
		}
		return s.deps.Events.Publish(ctx, tx, workspaceID, model.EventCompleted, model.Completed{
			BroadcastID: id, Name: b.Name, Recipients: int(b.RecipientsTotal), Sent: int(st.Sent), Delivered: int(st.Delivered),
			Read: int(st.Read), Failed: int(st.Failed), Skipped: int(st.Skipped), CompletedAt: now,
		})
	})
	if err != nil {
		return err
	}
	if reached && s.deps.Activation != nil {
		if _, err := s.deps.Activation.MarkActivated(ctx, workspaceID); err != nil {
			s.log.ErrorContext(ctx, "mark workspace activated", "workspace_id", workspaceID, "error", err)
		}
	}
	return nil
}

func (s *Service) fail(ctx context.Context, workspaceID, id uuid.UUID, reason string) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		repo := repository.NewRepository(tx)
		n, err := repo.FailBroadcast(ctx, gen.FailBroadcastParams{WorkspaceID: workspaceID, ID: id, Reason: &reason, Now: s.now()})
		if err != nil {
			return fmt.Errorf("fail broadcast: %w", err)
		}
		if n == 0 {
			return nil
		}
		s.log.WarnContext(ctx, "broadcast failed", "workspace_id", workspaceID, "broadcast_id", id, "reason", reason)
		if err := repo.SkipPending(ctx, gen.SkipPendingParams{WorkspaceID: workspaceID, BroadcastID: id, Reason: &reason}); err != nil {
			return fmt.Errorf("skip pending recipients: %w", err)
		}
		return nil
	})
}
