// Package repository is broadcasts' data access: the sqlc queries in this folder and small helpers
// around them.
//
// Every query on tenant data takes the workspace ID, except the Due…, Finished… and
// Stalled… lookups that only the periodic tick runs across all workspaces.
//
// The audience and report queries read contacts, contact_tag_links, message_templates
// and messages directly, as automations' run stats read messages: one indexed query
// instead of paging thousands of rows through another feature (ADR 0013).
package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// Repository is the sqlc query set for the broadcast tables. It works on a pool or a transaction.
type Repository struct {
	*gen.Queries
}

// NewRepository wraps the sqlc queries for db, a pool or a transaction.
func NewRepository(db database.DBTX) Repository {
	return Repository{Queries: gen.New(db)}
}

// IsNoRows reports whether err means the query matched no row.
func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
