// Package dbtest gives tests a migrated Postgres database.
//
// It uses TEST_DATABASE_URL when set (CI), otherwise starts a throwaway
// Postgres container with testcontainers (local, needs Docker).
package dbtest

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

const image = "postgres:18.6-alpine"

var (
	once   sync.Once
	dbURL  string
	setErr error
)

// New returns a pool connected to a migrated test database. It skips the test
// under -short. The database is shared by every test in the package run.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database test in -short mode")
	}

	once.Do(func() { dbURL, setErr = setup() })
	if setErr != nil {
		t.Fatalf("set up test database: %v", setErr)
	}

	ctx := context.Background()
	pool, err := database.New(ctx, dbURL, 5)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// URL returns the test database URL. Call New first.
func URL() string { return dbURL }

func setup() (string, error) {
	ctx := context.Background()
	url := os.Getenv("TEST_DATABASE_URL")

	if url == "" {
		// The container lives until the test binary exits; Ryuk reaps it.
		c, err := postgres.Run(ctx, image,
			postgres.WithDatabase("relay_test"),
			postgres.WithUsername("relay"),
			postgres.WithPassword("relay"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).WithStartupTimeout(60*time.Second)),
		)
		if err != nil {
			return "", fmt.Errorf("start postgres container: %w", err)
		}
		if url, err = c.ConnectionString(ctx, "sslmode=disable"); err != nil {
			return "", fmt.Errorf("postgres container url: %w", err)
		}
	}

	pool, err := database.New(ctx, url, 2)
	if err != nil {
		return "", err
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool, "up", io.Discard); err != nil {
		return "", err
	}
	return url, nil
}
