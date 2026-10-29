// Package repository is billing's data access: the sqlc queries in this folder and small helpers
// around them.
//
// Every query on tenant data takes the workspace ID, except the Find… lookups
// webhooks use to discover which workspace an event belongs to, and the List…
// queries that only background jobs run across all workspaces.
package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// Unique indexes and constraints the service maps to friendly errors.
const (
	// One trialing, active or past_due subscription per workspace.
	LiveSubscriptionIndex = "subscriptions_live_uq"
	// One in_installments, paid or defaulted order per workspace.
	LiveOrderIndex = "orders_live_uq"
	// One active or suspended grant per workspace.
	CurrentGrantIndex = "plan_grants_current_uq"
	// One redemption per checkout session.
	RedemptionSessionKey = "discount_redemptions_checkout_session_id_key"
	// A discount code is unique.
	DiscountCodeKey = "discounts_code_key"
)

// Repository is the sqlc query set for billing tables. It works on a pool or a transaction.
type Repository struct {
	*gen.Queries
}

// NewRepository wraps the sqlc queries for db, a pool or a transaction.
func NewRepository(db database.DBTX) Repository {
	return Repository{gen.New(db)}
}

// IsNoRows reports whether err means the query matched no row.
func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// IsUnique reports a unique-constraint violation on the named index or constraint.
func IsUnique(err error, name string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == name
}

// IsCheck reports a CHECK violation, such as paying more than an order's total.
func IsCheck(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}

// IsForeignKey reports a foreign-key violation (an ID from another workspace, or gone).
func IsForeignKey(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
