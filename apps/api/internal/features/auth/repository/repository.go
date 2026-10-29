// Package repository is auth's data access: the sqlc queries in this folder and small helpers
// around them.
package repository

import (
	"errors"
	"net/netip"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// Repository is the sqlc query set for auth tables. It works on a pool or a transaction.
type Repository struct {
	*gen.Queries
}

// NewRepository wraps the sqlc queries for db, a pool or a transaction.
func NewRepository(db database.DBTX) Repository {
	return Repository{gen.New(db)}
}

// IsNoRows reports whether err means the query matched no row.
func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// IsUniqueViolation reports whether err is a unique-constraint violation.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// ParseIP returns nil when the address can't be parsed (stored as NULL).
func ParseIP(s string) *netip.Addr {
	ip, err := netip.ParseAddr(s)
	if err != nil {
		return nil
	}
	return &ip
}

// IPString formats ip, or returns "" when there is none.
func IPString(ip *netip.Addr) string {
	if ip == nil {
		return ""
	}
	return ip.String()
}
