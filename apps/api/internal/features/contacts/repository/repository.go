// Package repository is contacts' data access: the sqlc queries in this folder and small helpers
// around them.
package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// PhoneConstraint is the unique constraint on a contact's phone in its workspace.
const PhoneConstraint = "contacts_workspace_phone_key"

// Repository is the sqlc query set for contacts tables. It works on a pool or a transaction.
type Repository struct {
	*gen.Queries
	db database.DBTX
}

// NewRepository wraps the sqlc queries for db, a pool or a transaction.
func NewRepository(db database.DBTX) Repository {
	return Repository{Queries: gen.New(db), db: db}
}

// SegmentNameConstraint is the unique constraint on a segment's name in its workspace.
const SegmentNameConstraint = "contact_segments_workspace_name_key"

// IsNoRows reports whether err means the query matched no row.
func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// IsUnique reports a unique-constraint violation on the named constraint.
func IsUnique(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}
