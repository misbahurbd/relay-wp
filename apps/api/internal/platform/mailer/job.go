package mailer

import (
	"context"

	"github.com/riverqueue/river"
)

// SendArgs is the River job that sends one email. Features enqueue it in the
// same transaction as their data change (outbox pattern) instead of calling Send directly.
type SendArgs struct {
	Message Message `json:"message"`
}

// Kind is the job's stable name in the river_job table.
func (SendArgs) Kind() string { return "send_email" }

// InsertOpts retries a failed send up to 10 times with River's default backoff.
func (SendArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{MaxAttempts: 10} }

// SendWorker works SendArgs jobs.
type SendWorker struct {
	river.WorkerDefaults[SendArgs]
	mailer Mailer
}

// NewSendWorker returns a worker that sends through m.
func NewSendWorker(m Mailer) *SendWorker { return &SendWorker{mailer: m} }

// Work sends the email; returning an error makes River retry.
func (w *SendWorker) Work(ctx context.Context, job *river.Job[SendArgs]) error {
	return w.mailer.Send(ctx, job.Args.Message)
}
