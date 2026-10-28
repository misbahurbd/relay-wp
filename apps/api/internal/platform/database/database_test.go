package database_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database/dbtest"
)

func TestWithTxCommitsAndRollsBack(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS tx_probe (v text PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DROP TABLE tx_probe`) })

	if err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO tx_probe VALUES ('committed')`)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	boom := errors.New("boom")
	err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO tx_probe VALUES ('rolled_back')`); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("WithTx error = %v, want boom", err)
	}

	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tx_probe`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("rows = %d, want 1 (only the committed one)", n)
	}
}

func TestMigrationsApplied(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()

	var out strings.Builder
	if err := database.Migrate(ctx, pool, "status", &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "applied") || !strings.Contains(out.String(), "00001_init.sql") {
		t.Errorf("status output:\n%s", out.String())
	}

	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('river_job') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("river_job table missing; River migrations did not run")
	}

	if err := database.Migrate(ctx, pool, "sideways", io.Discard); err == nil {
		t.Error("expected error for unknown command")
	}
}
