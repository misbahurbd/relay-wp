// Command relay is the single binary for the backend.
//
//	relay api                    serve HTTP (enqueues jobs, doesn't work them)
//	relay worker                 work background jobs
//	relay all                    both, in one process (default)
//	relay migrate up|down|status run database migrations
//	relay openapi                print the OpenAPI spec to stdout
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/misbahurbd/relay-wp/apps/api/internal/app"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/logger"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cmd := "all"
	if len(args) > 0 {
		cmd = args[0]
	}

	// openapi needs no configuration or connections.
	if cmd == "openapi" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(app.OpenAPI()); err != nil {
			return fmt.Errorf("encode openapi: %w", err)
		}
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log, err := logger.New(os.Stderr, cfg.LogFormat, cfg.LogLevel)
	if err != nil {
		return err
	}
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "api", "worker", "all":
		a, err := app.New(ctx, cfg, log, app.Mode(cmd))
		if err != nil {
			return err
		}
		defer a.Close()
		return a.Run(ctx)
	case "migrate":
		if len(args) < 2 {
			return errors.New("usage: relay migrate up|down|status")
		}
		pool, err := database.New(ctx, cfg.Database.DirectURL, 2)
		if err != nil {
			return err
		}
		defer pool.Close()
		return database.Migrate(ctx, pool, args[1], os.Stdout)
	default:
		return fmt.Errorf("unknown command %q (want api, worker, all, migrate, openapi)", cmd)
	}
}
