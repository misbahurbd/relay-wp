// Package repository is automations' data access: the sqlc queries in this folder and small helpers
// around them.
package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// Repository is the sqlc query set for the automation tables. It works on a pool or a transaction.
type Repository struct {
	*gen.Queries
}

// NewRepository wraps the sqlc queries for db, a pool or a transaction.
func NewRepository(db database.DBTX) Repository {
	return Repository{Queries: gen.New(db)}
}

// IsNoRows reports whether err means the query matched no row.
func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
