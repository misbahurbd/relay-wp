package mailer_test

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/mailer"
)

func TestNewRejectsUnknownDriver(t *testing.T) {
	if _, err := mailer.New(config.Mailer{Driver: "pigeon"}, slog.Default()); err == nil {
		t.Error("expected error for unknown driver")
	}
	if _, err := mailer.New(config.Mailer{Driver: "smtp", SMTPHost: "localhost", SMTPPort: 25, SMTPTLS: "maybe"}, slog.Default()); err == nil {
		t.Error("expected error for unknown SMTP_TLS")
	}
}

func TestLogDriverLogsMessage(t *testing.T) {
	var buf bytes.Buffer
	m, err := mailer.New(config.Mailer{Driver: "log"}, slog.New(slog.NewTextHandler(&buf, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Send(context.Background(), mailer.Message{To: []string{"a@b.test"}, Subject: "Hi", Text: "x"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "subject=Hi") {
		t.Errorf("log output = %s", buf.String())
	}
}

// TestSMTPSend talks to a real SMTP server (Mailpit locally). Set SMTP_TEST_PORT to run it.
func TestSMTPSend(t *testing.T) {
	port, _ := strconv.Atoi(os.Getenv("SMTP_TEST_PORT"))
	if port == 0 {
		t.Skip("SMTP_TEST_PORT not set")
	}
	m, err := mailer.New(config.Mailer{
		Driver: "smtp", From: "no-reply@relay.test", FromName: "Relay",
		SMTPHost: "localhost", SMTPPort: port, SMTPTLS: "none",
	}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	err = m.Send(context.Background(), mailer.Message{
		To: []string{"dev@relay.test"}, Subject: "Relay SMTP test", Text: "Plain body", HTML: "<p>HTML body</p>",
	})
	if err != nil {
		t.Fatal(err)
	}
}
