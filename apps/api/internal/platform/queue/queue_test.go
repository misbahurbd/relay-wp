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

func (f *fakeMailer) Send(_ context.Context, m mailer.Message) error {
	f.sent <- m
	return nil
}

func TestSendEmailJobRuns(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()

	fm := &fakeMailer{sent: make(chan mailer.Message, 1)}
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

	if _, err := client.Insert(ctx, mailer.SendArgs{Message: mailer.Message{
		To: []string{"dev@relay.test"}, Subject: "Queued", Text: "hi",
	}}, nil); err != nil {
		t.Fatal(err)
	}

	select {
	case m := <-fm.sent:
		if m.Subject != "Queued" {
			t.Errorf("subject = %q", m.Subject)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("send_email job did not run within 15s")
	}
}
