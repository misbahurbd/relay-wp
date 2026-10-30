package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/app"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

// runDev holds local-development helpers. They refuse to run outside development.
//
//	relay dev seed-inbox <workspace-id>   sample WhatsApp chats for the inbox
func runDev(ctx context.Context, cfg config.Config, log *slog.Logger, args []string, out io.Writer) error {
	if !cfg.IsDevelopment() {
		return errors.New("dev commands run only with APP_ENV=development")
	}
	if len(args) != 2 || args[0] != "seed-inbox" {
		return errors.New("usage: relay dev seed-inbox <workspace-id>")
	}
	workspaceID, err := uuid.Parse(args[1])
	if err != nil {
		return fmt.Errorf("workspace id: %w", err)
	}
	a, err := app.New(ctx, cfg, log, app.ModeWorker)
	if err != nil {
		return err
	}
	defer a.Close()
	n, err := a.SeedInbox(ctx, workspaceID)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Seeded %d sample messages. Set WHATSAPP_FAKE=true so replies get delivered ticks.\n", n); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}
