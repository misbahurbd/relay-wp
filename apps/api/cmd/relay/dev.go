package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/app"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

const devUsage = `usage:
  relay dev seed                                         CRM staff, plans and sample tenants
  relay dev seed-inbox <workspace-id>                    sample WhatsApp chats for the inbox
  relay dev inbound [flags] <workspace-id> <text>        a customer writes to the fake number (WHATSAPP_FAKE=true)
  relay dev webhook [flags] <workspace-id> [<text>]      a signed Meta webhook to the local API (WHATSAPP_ENABLED=true)`

// runDev holds local-development helpers. They refuse to run outside development.
func runDev(ctx context.Context, cfg config.Config, log *slog.Logger, args []string, out io.Writer) error {
	if !cfg.IsDevelopment() {
		return errors.New("dev commands run only with APP_ENV=development")
	}
	if len(args) == 0 {
		return errors.New(devUsage)
	}
	switch args[0] {
	case "seed":
		return withApp(ctx, cfg, log, func(a *app.App) error { return a.Seed(ctx, out) })
	case "seed-inbox":
		if len(args) != 2 {
			return errors.New(devUsage)
		}
		workspaceID, err := uuid.Parse(args[1])
		if err != nil {
			return fmt.Errorf("workspace id: %w", err)
		}
		return withApp(ctx, cfg, log, func(a *app.App) error {
			n, err := a.SeedInbox(ctx, workspaceID)
			if err != nil {
				return err
			}
			return say(out, "Seeded %d sample messages. Set WHATSAPP_FAKE=true so replies get delivered ticks.\n", n)
		})
	case "inbound":
		return devInbound(ctx, cfg, log, args[1:], out)
	case "webhook":
		return devWebhook(ctx, cfg, args[1:], out)
	}
	return errors.New(devUsage)
}

func devInbound(ctx context.Context, cfg config.Config, log *slog.Logger, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("dev inbound", flag.ContinueOnError)
	fs.SetOutput(out)
	from := fs.String("from", "+14155550101", "the customer's phone, E.164")
	name := fs.String("name", "Daniel Brooks", "the customer's WhatsApp profile name")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}
	workspaceID, text, err := workspaceAndText(fs.Args(), true)
	if err != nil {
		return err
	}
	return withApp(ctx, cfg, log, func(a *app.App) error {
		if err := a.SimulateInbound(ctx, workspaceID, *from, *name, text); err != nil {
			return err
		}
		return say(out, "%s (%s) wrote %q. It is in the inbox now.\n", *name, *from, text)
	})
}

// workspaceAndText reads "<workspace-id> <text words…>".
func workspaceAndText(args []string, needText bool) (uuid.UUID, string, error) {
	if len(args) == 0 || (needText && len(args) < 2) {
		return uuid.Nil, "", errors.New(devUsage)
	}
	workspaceID, err := uuid.Parse(args[0])
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("workspace id: %w", err)
	}
	return workspaceID, strings.Join(args[1:], " "), nil
}

func withApp(ctx context.Context, cfg config.Config, log *slog.Logger, run func(*app.App) error) error {
	a, err := app.New(ctx, cfg, log, app.ModeWorker)
	if err != nil {
		return err
	}
	defer a.Close()
	return run(a)
}

func say(out io.Writer, format string, args ...any) error {
	if _, err := fmt.Fprintf(out, format, args...); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}
