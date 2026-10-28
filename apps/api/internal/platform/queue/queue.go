// Package queue builds the River job client. Jobs live in Postgres, so no extra service is needed.
package queue

import (
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// Client inserts jobs (and works them, when built with workers).
type Client = river.Client[pgx.Tx]

// New builds a River client on the direct (non-pooled) pool, which River needs for LISTEN/NOTIFY.
// With workers == nil the client is insert-only, as used by the API process.
func New(directPool *pgxpool.Pool, workers *river.Workers, maxWorkers int, log *slog.Logger) (*Client, error) {
	cfg := &river.Config{Logger: log}
	if workers != nil {
		cfg.Workers = workers
		cfg.Queues = map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: maxWorkers}}
	}

	c, err := river.NewClient(riverpgxv5.New(directPool), cfg)
	if err != nil {
		return nil, fmt.Errorf("create river client: %w", err)
	}
	return c, nil
}
