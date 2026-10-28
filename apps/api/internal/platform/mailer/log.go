package mailer

import (
	"context"
	"log/slog"
)

// logMailer writes emails to the log instead of sending them. For tests and local runs without SMTP.
type logMailer struct {
	log  *slog.Logger
	from string
}

func (m *logMailer) Send(ctx context.Context, msg Message) error {
	m.log.InfoContext(ctx, "email (log driver, not sent)",
		"from", m.from, "to", msg.To, "subject", msg.Subject, "text", msg.Text)
	return nil
}
