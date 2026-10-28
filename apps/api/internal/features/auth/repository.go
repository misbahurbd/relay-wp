package auth

import (
	"errors"
	"net/netip"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// repository is the sqlc query set for auth tables. It works on a pool or a transaction.
type repository struct {
	*gen.Queries
}

func newRepository(db database.DBTX) repository {
	return repository{gen.New(db)}
}

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// parseIP returns nil when the address can't be parsed (stored as NULL).
func parseIP(s string) *netip.Addr {
	ip, err := netip.ParseAddr(s)
	if err != nil {
		return nil
	}
	return &ip
}

func ipString(ip *netip.Addr) string {
	if ip == nil {
		return ""
	}
	return ip.String()
}
