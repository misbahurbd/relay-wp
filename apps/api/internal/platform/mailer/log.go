package mailer

import (
	"context"
	"fmt"
	"log/slog"
)

// logMailer writes emails to the log instead of sending them. For tests and local runs without SMTP.
type logMailer struct {
	log  *slog.Logger
	from string
}

func (m *logMailer) Send(ctx context.Context, msg Message) error {
	args := []any{"from", m.from, "to", msg.To, "subject", msg.Subject, "text", msg.Text}
	if len(msg.Attachments) > 0 {
		files := make([]string, len(msg.Attachments))
		for i, a := range msg.Attachments {
			files[i] = fmt.Sprintf("%s (%d bytes)", a.Filename, len(a.Data))
		}
		args = append(args, "attachments", files)
	}
	m.log.InfoContext(ctx, "email (log driver, not sent)", args...)
	return nil
}
