package mailer_test

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestLogDriverLogsAttachmentNamesNotBytes(t *testing.T) {
	var buf bytes.Buffer
	m, err := mailer.New(config.Mailer{Driver: "log"}, slog.New(slog.NewTextHandler(&buf, nil)))
	if err != nil {
		t.Fatal(err)
	}
	err = m.Send(context.Background(), mailer.Message{
		To: []string{"a@b.test"}, Subject: "Receipt", Text: "x",
		Attachments: []mailer.Attachment{{Filename: "RLY-2026-000001.pdf", ContentType: "application/pdf", Data: []byte("%PDF-secret-bytes")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "RLY-2026-000001.pdf (17 bytes)") {
		t.Errorf("log output lacks the attachment name and size: %s", out)
	}
	if strings.Contains(out, "secret-bytes") {
		t.Errorf("log output contains attachment bytes: %s", out)
	}
}

func TestQueuedMessageDropsAttachments(t *testing.T) {
	raw, err := json.Marshal(mailer.SendArgs{Message: mailer.Message{
		To: []string{"a@b.test"}, Subject: "s", Text: "t",
		Attachments: []mailer.Attachment{{Filename: "f.pdf", Data: []byte("%PDF-")}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "f.pdf") {
		t.Errorf("job args carry the attachment: %s", raw)
	}
}
