package profiles

import (
	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// repository is the sqlc query set for profile tables. It works on a pool or a transaction.
type repository struct {
	*gen.Queries
}

func newRepository(db database.DBTX) repository {
	return repository{gen.New(db)}
}
