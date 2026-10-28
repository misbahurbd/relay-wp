package whatsapp

import (
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// repository is the sqlc query set for WhatsApp tables. It works on a pool or a transaction.
type repository struct {
	*gen.Queries
}

func newRepository(db database.DBTX) repository {
	return repository{gen.New(db)}
}

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
