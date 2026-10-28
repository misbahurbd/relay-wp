// Command relay is the single binary for the backend.
//
//	relay api      start the HTTP server
//	relay openapi  print the OpenAPI spec to stdout
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/misbahurbd/relay-wp/apps/api/internal/app"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cmd := "api"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "api":
		return app.New().Run(ctx)
	case "openapi":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(app.New().OpenAPI()); err != nil {
			return fmt.Errorf("encode openapi: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q (want: api, openapi)", cmd)
	}
}
