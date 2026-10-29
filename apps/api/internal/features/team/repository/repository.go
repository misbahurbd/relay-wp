// Package repository is team's data access: the sqlc queries in this folder and small helpers
// around them.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// Repository is the sqlc query set for team tables. It works on a pool or a transaction.
type Repository struct {
	*gen.Queries
	db database.DBTX
}

// NewRepository wraps the sqlc queries for db, a pool or a transaction.
func NewRepository(db database.DBTX) Repository {
	return Repository{Queries: gen.New(db), db: db}
}

// EnsureLogPartitions creates the access log's monthly partitions from this
// month through `ahead` months, and drops those older than `keep` months.
// Partition DDL can't go through sqlc.
func (r Repository) EnsureLogPartitions(ctx context.Context, now time.Time, ahead, keep int) error {
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i <= ahead; i++ {
		from := start.AddDate(0, i, 0)
		to := from.AddDate(0, 1, 0)
		sql := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS access_log_events_%s PARTITION OF access_log_events
			FOR VALUES FROM ('%s') TO ('%s')`, from.Format("2006_01"), from.Format(time.DateOnly), to.Format(time.DateOnly))
		if _, err := r.db.Exec(ctx, sql); err != nil {
			return fmt.Errorf("create log partition %s: %w", from.Format("2006-01"), err)
		}
	}
	cutoff := start.AddDate(0, -keep, 0)
	rows, err := r.db.Query(ctx, `SELECT c.relname FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid JOIN pg_class p ON p.oid = i.inhparent
		WHERE p.relname = 'access_log_events' AND c.relname ~ '^access_log_events_[0-9]{4}_[0-9]{2}$'`)
	if err != nil {
		return fmt.Errorf("list log partitions: %w", err)
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return fmt.Errorf("list log partitions: %w", err)
	}
	for _, name := range names {
		month, err := time.Parse("2006_01", name[len("access_log_events_"):])
		if err != nil || !month.Before(cutoff) {
			continue
		}
		if _, err := r.db.Exec(ctx, "DROP TABLE "+pgx.Identifier{name}.Sanitize()); err != nil {
			return fmt.Errorf("drop log partition %s: %w", name, err)
		}
	}
	return nil
}

// IsNoRows reports whether err means the query matched no row.
func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// IsUnique reports a unique-constraint violation on the named index.
func IsUnique(err error, index string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == index
}

// IsForeignKey reports a foreign-key violation (an ID from another workspace, or gone).
func IsForeignKey(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
