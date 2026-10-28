package database

import (
	"context"
	"fmt"
	"io"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/misbahurbd/relay-wp/apps/api/db"
)

// Migrate runs "up", "down" or "status" for the app's goose migrations.
// "up" also applies River's own migrations so the job tables exist.
// pool must use the direct (non-pooled) connection.
func Migrate(ctx context.Context, pool *pgxpool.Pool, command string, out io.Writer) error {
	migrations, err := fs.Sub(db.Migrations, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()

	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}

	switch command {
	case "up":
		if err := migrateRiver(ctx, pool, out); err != nil {
			return err
		}
		results, err := provider.Up(ctx)
		for _, r := range results {
			_, _ = fmt.Fprintln(out, r)
		}
		if err != nil {
			return fmt.Errorf("migrate up: %w", err)
		}
	case "down":
		r, err := provider.Down(ctx)
		if r != nil {
			_, _ = fmt.Fprintln(out, r)
		}
		if err != nil {
			return fmt.Errorf("migrate down: %w", err)
		}
	case "status":
		statuses, err := provider.Status(ctx)
		if err != nil {
			return fmt.Errorf("migrate status: %w", err)
		}
		for _, s := range statuses {
			_, _ = fmt.Fprintf(out, "%-10s %s\n", s.State, s.Source.Path)
		}
	default:
		return fmt.Errorf("unknown migrate command %q (want up, down, status)", command)
	}
	return nil
}

func migrateRiver(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("create river migrator: %w", err)
	}
	res, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("migrate river: %w", err)
	}
	for _, v := range res.Versions {
		_, _ = fmt.Fprintf(out, "river migration %03d applied\n", v.Version)
	}
	return nil
}
