// Package mailer sends email. The implementation is chosen by MAILER_DRIVER.
package mailer

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

// Message is one email. Text is required; HTML is optional.
type Message struct {
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
	HTML    string   `json:"html,omitempty"`
	// Attachments are never encoded, so file bytes can't land in a queued
	// SendArgs job: a message with attachments is sent with Send directly.
	Attachments []Attachment `json:"-"`
}

// Attachment is one file attached to a message.
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// Mailer sends a message.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

// New returns the Mailer for cfg.Driver: "smtp" or "log".
func New(cfg config.Mailer, log *slog.Logger) (Mailer, error) {
	switch cfg.Driver {
	case "smtp":
		return newSMTP(cfg)
	case "log":
		return &logMailer{log: log, from: cfg.From}, nil
	default:
		return nil, fmt.Errorf("unknown MAILER_DRIVER %q (want smtp or log)", cfg.Driver)
	}
}
