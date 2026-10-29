package team

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// MaintenanceArgs expires temporary permissions and keeps the access log's
// monthly partitions: two months ahead, twelve kept. Access checks already
// ignore expired permissions, so this only records the expiry and tidies up.
type MaintenanceArgs struct{}

// Kind is the job's stable name in the river_job table.
func (MaintenanceArgs) Kind() string { return "team_maintenance" }

// MaintenanceWorker works MaintenanceArgs.
type MaintenanceWorker struct {
	river.WorkerDefaults[MaintenanceArgs]
	svc *Service
}

// NewMaintenanceWorker builds the worker.
func NewMaintenanceWorker(svc *Service) *MaintenanceWorker { return &MaintenanceWorker{svc: svc} }

// Work runs one maintenance pass.
func (w *MaintenanceWorker) Work(ctx context.Context, _ *river.Job[MaintenanceArgs]) error {
	if err := w.svc.ExpireOverrides(ctx); err != nil {
		return err
	}
	return w.svc.EnsureLogPartitions(ctx)
}

// MaintenancePeriodicJob runs maintenance every 15 minutes and at startup.
func MaintenancePeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(15*time.Minute),
		func() (river.JobArgs, *river.InsertOpts) { return MaintenanceArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true})
}

// ExpireOverrides records temporary permissions that have reached their end date.
func (s *Service) ExpireOverrides(ctx context.Context) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		rows, err := r.ExpireOverrides(ctx)
		if err != nil {
			return fmt.Errorf("expire overrides: %w", err)
		}
		for _, o := range rows {
			if err := r.BumpAccessVersions(ctx, gen.BumpAccessVersionsParams{WorkspaceID: o.WorkspaceID, UserIds: []uuid.UUID{o.UserID}}); err != nil {
				return fmt.Errorf("bump access: %w", err)
			}
			if err := r.log(ctx, event{
				workspaceID: o.WorkspaceID, subject: &o.UserID, action: ActionPermissionExpired, target: o.PermissionKey,
				before: map[string]any{"effect": o.Effect},
			}, Meta{}); err != nil {
				return err
			}
		}
		return nil
	})
}

// EnsureLogPartitions keeps the access log's partitions two months ahead and drops those older than a year.
func (s *Service) EnsureLogPartitions(ctx context.Context) error {
	return newRepository(s.pool).ensureLogPartitions(ctx, s.now(), 2, 12)
}

// ListAccessLog returns one page of the access log, newest first, and the cursor for the next page.
func (s *Service) ListAccessLog(ctx context.Context, workspaceID uuid.UUID, f LogFilter) ([]LogEntry, *LogCursor, error) {
	if f.PageSize <= 0 || f.PageSize > 200 {
		f.PageSize = 50
	}
	if oldest := s.now().AddDate(-1, 0, 0); f.Since.Before(oldest) {
		f.Since = oldest
	}
	p := gen.ListAccessLogParams{
		WorkspaceID: workspaceID, Since: f.Since, Action: f.Action, SubjectUserID: f.SubjectID, PageSize: f.PageSize + 1,
	}
	if f.Before != nil {
		p.BeforeCreatedAt, p.BeforeID = &f.Before.CreatedAt, &f.Before.ID
	}
	rows, err := newRepository(s.pool).ListAccessLog(ctx, p)
	if err != nil {
		return nil, nil, fmt.Errorf("list access log: %w", err)
	}
	var next *LogCursor
	if len(rows) > int(f.PageSize) {
		rows = rows[:f.PageSize]
		last := rows[len(rows)-1]
		next = &LogCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	out := make([]LogEntry, len(rows))
	for i, r := range rows {
		ip := ""
		if r.Ip != nil {
			ip = r.Ip.String()
		}
		out[i] = LogEntry{
			ID: r.ID, CreatedAt: r.CreatedAt, ActorID: r.ActorUserID, ActorName: r.ActorName, SubjectID: r.SubjectUserID,
			SubjectName: r.SubjectName, SubjectEmail: r.SubjectEmail, Action: r.Action, Target: r.Target,
			Before: r.BeforeValue, After: r.AfterValue, Note: r.Note, IP: ip, UserAgent: r.UserAgent,
		}
	}
	return out, next, nil
}

// AccessLogSummary is the last 30 days at a glance: changes, permissions ending
// within a week and members with repeated failed sign-ins.
func (s *Service) AccessLogSummary(ctx context.Context, workspaceID uuid.UUID) (LogSummary, error) {
	r := newRepository(s.pool)
	now := s.now()
	since := now.AddDate(0, 0, -30)
	counts, err := r.AccessLogSummary(ctx, gen.AccessLogSummaryParams{WorkspaceID: workspaceID, Since: since})
	if err != nil {
		return LogSummary{}, fmt.Errorf("log summary: %w", err)
	}
	out := LogSummary{Events: counts.Events, PermissionChanges: counts.PermissionChanges}
	until := now.AddDate(0, 0, 7)
	expiring, err := r.ExpiringOverrides(ctx, gen.ExpiringOverridesParams{WorkspaceID: workspaceID, Until: &until})
	if err != nil {
		return LogSummary{}, fmt.Errorf("expiring overrides: %w", err)
	}
	for _, e := range expiring {
		if e.ExpiresAt != nil {
			out.Expiring = append(out.Expiring, Expiring{UserID: e.UserID, Name: e.Name, PermissionKey: e.PermissionKey, ExpiresAt: *e.ExpiresAt})
		}
	}
	alerts, err := r.FailedLoginAlerts(ctx, gen.FailedLoginAlertsParams{WorkspaceID: workspaceID, Since: since})
	if err != nil {
		return LogSummary{}, fmt.Errorf("login alerts: %w", err)
	}
	for _, a := range alerts {
		if a.UserID != nil {
			out.Alerts = append(out.Alerts, LoginAlert{UserID: *a.UserID, Name: a.Name, Failures: a.Failures, LastAt: a.LastAt})
		}
	}
	return out, nil
}
