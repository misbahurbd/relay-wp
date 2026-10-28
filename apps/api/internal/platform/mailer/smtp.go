package mailer

import (
	"context"
	"errors"
	"fmt"

	"github.com/wneessen/go-mail"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

type smtpMailer struct {
	client   *mail.Client
	from     string
	fromName string
}

func newSMTP(cfg config.Mailer) (*smtpMailer, error) {
	opts := []mail.Option{mail.WithPort(cfg.SMTPPort)}

	switch cfg.SMTPTLS {
	case "none":
		opts = append(opts, mail.WithTLSPolicy(mail.NoTLS))
	case "starttls":
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	case "tls":
		opts = append(opts, mail.WithSSL())
	default:
		return nil, fmt.Errorf("unknown SMTP_TLS %q (want none, starttls or tls)", cfg.SMTPTLS)
	}
	if cfg.SMTPUser != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(cfg.SMTPUser),
			mail.WithPassword(cfg.SMTPPass))
	}

	c, err := mail.NewClient(cfg.SMTPHost, opts...)
	if err != nil {
		return nil, fmt.Errorf("create smtp client: %w", err)
	}
	return &smtpMailer{client: c, from: cfg.From, fromName: cfg.FromName}, nil
}

func (m *smtpMailer) Send(ctx context.Context, msg Message) error {
	if len(msg.To) == 0 {
		return errors.New("mailer: message has no recipients")
	}

	em := mail.NewMsg()
	if err := em.FromFormat(m.fromName, m.from); err != nil {
		return fmt.Errorf("mailer: from: %w", err)
	}
	if err := em.To(msg.To...); err != nil {
		return fmt.Errorf("mailer: to: %w", err)
	}
	em.Subject(msg.Subject)
	em.SetBodyString(mail.TypeTextPlain, msg.Text)
	if msg.HTML != "" {
		em.AddAlternativeString(mail.TypeTextHTML, msg.HTML)
	}

	if err := m.client.DialAndSendWithContext(ctx, em); err != nil {
		return fmt.Errorf("mailer: send: %w", err)
	}
	return nil
}
