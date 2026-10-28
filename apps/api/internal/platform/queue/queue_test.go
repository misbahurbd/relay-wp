package queue_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database/dbtest"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/mailer"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/queue"
)

type fakeMailer struct{ sent chan mailer.Message }

// Send never blocks: jobs left by other packages' tests on the shared CI
// database may run here too, and are dropped once the buffer is full.
func (f *fakeMailer) Send(_ context.Context, m mailer.Message) error {
	select {
	case f.sent <- m:
	default:
	}
	return nil
}

func TestSendEmailJobRuns(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()

	fm := &fakeMailer{sent: make(chan mailer.Message, 64)}
	workers := river.NewWorkers()
	river.AddWorker(workers, mailer.NewSendWorker(fm))

	client, err := queue.New(pool, workers, 2, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Stop(context.Background()) })

	subject := "Queued " + t.Name() + " " + time.Now().Format(time.RFC3339Nano)
	if _, err := client.Insert(ctx, mailer.SendArgs{Message: mailer.Message{
		To: []string{"dev@relay.test"}, Subject: subject, Text: "hi",
	}}, nil); err != nil {
		t.Fatal(err)
	}

	// Other tests may share this database, so skip any job that isn't ours.
	timeout := time.After(15 * time.Second)
	for {
		select {
		case m := <-fm.sent:
			if m.Subject == subject {
				return
			}
		case <-timeout:
			t.Fatal("send_email job did not run within 15s")
		}
	}
}
