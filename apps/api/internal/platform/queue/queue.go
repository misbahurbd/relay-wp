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

// Bulk is the queue for jobs that come in thousands at once, such as a
// broadcast's sends, so they never wait in front of the default queue's work
// (ADR 0013). Insert on it with &river.InsertOpts{Queue: queue.Bulk}.
const Bulk = "bulk"

// New builds a River client on the direct (non-pooled) pool, which River needs for LISTEN/NOTIFY.
// With workers == nil the client is insert-only, as used by the API process.
// maxWorkers and bulkWorkers size the default and Bulk queues.
func New(directPool *pgxpool.Pool, workers *river.Workers, maxWorkers, bulkWorkers int, log *slog.Logger) (*Client, error) {
	cfg := &river.Config{Logger: log}
	if workers != nil {
		cfg.Workers = workers
		cfg.Queues = map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: maxWorkers},
			Bulk:               {MaxWorkers: bulkWorkers},
		}
	}

	c, err := river.NewClient(riverpgxv5.New(directPool), cfg)
	if err != nil {
		return nil, fmt.Errorf("create river client: %w", err)
	}
	return c, nil
}
