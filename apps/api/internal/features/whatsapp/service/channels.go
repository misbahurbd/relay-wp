package service

import (
	"context"
	"fmt"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Channels tells other features whether a workspace can send WhatsApp messages.
// It has no dependency on Service, so features/onboarding can be built first.
type Channels struct {
	pool *pgxpool.Pool
}

// NewChannels returns a Channels reader.
func NewChannels(pool *pgxpool.Pool) *Channels { return &Channels{pool: pool} }

// HasConnectedNumber reports whether the workspace has a number ready to send.
func (c *Channels) HasConnectedNumber(ctx context.Context, workspaceID uuid.UUID) (bool, error) {
	ok, err := repository.NewRepository(c.pool).HasConnectedNumber(ctx, workspaceID)
	if err != nil {
		return false, fmt.Errorf("has connected number: %w", err)
	}
	return ok, nil
}
