package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/logger"
)

// CleanupArgs deletes auth rows nobody can use any more: expired sign-in
// attempts, email links and codes, refresh tokens, sessions ended over 30 days
// ago and security events past their 180 days. Runs hourly (CleanupPeriodicJob).
type CleanupArgs struct{}

// Kind is the job's stable name in the river_job table.
func (CleanupArgs) Kind() string { return "auth_cleanup" }

// CleanupPeriodicJob schedules CleanupArgs every hour, and once at start.
func CleanupPeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return CleanupArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true})
}

// CleanupWorker works CleanupArgs. It needs only the database, so worker
// processes (which don't build the auth service) can run it.
type CleanupWorker struct {
	river.WorkerDefaults[CleanupArgs]
	pool *pgxpool.Pool
}

// NewCleanupWorker builds the worker.
func NewCleanupWorker(pool *pgxpool.Pool) *CleanupWorker { return &CleanupWorker{pool: pool} }

// Work deletes each kind of stale row; every delete is independent and safe to repeat.
func (w *CleanupWorker) Work(ctx context.Context, _ *river.Job[CleanupArgs]) error {
	r := newRepository(w.pool)
	steps := []struct {
		name string
		run  func(context.Context) (int64, error)
	}{
		{"oauth_flows", r.DeleteExpiredOAuthFlows},
		{"email_tokens", r.DeleteExpiredEmailTokens},
		{"email_codes", r.DeleteExpiredEmailCodes},
		{"refresh_tokens", r.DeleteExpiredRefreshTokens},
		{"auth_sessions", r.DeleteEndedSessions},
		{"auth_events", r.DeleteOldAuthEvents},
	}
	deleted := map[string]int64{}
	for _, s := range steps {
		n, err := s.run(ctx)
		if err != nil {
			return fmt.Errorf("clean %s: %w", s.name, err)
		}
		deleted[s.name] = n
	}
	logger.FromContext(ctx).Info("auth cleanup", "deleted", deleted)
	return nil
}
