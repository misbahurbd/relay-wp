// Package storage stores files (objects). The implementation is chosen by STORAGE_DRIVER.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

// ErrNotFound is returned by Get when the key doesn't exist.
var ErrNotFound = errors.New("storage: object not found")

// Storage puts, gets and deletes objects by key.
type Storage interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// New returns the Storage for cfg.Driver: "s3" or "memory".
// With createBucket, the s3 driver creates the bucket if it's missing (local development).
func New(ctx context.Context, cfg config.Storage, createBucket bool) (Storage, error) {
	switch cfg.Driver {
	case "s3":
		return newS3(ctx, cfg, createBucket)
	case "memory":
		return newMemory(), nil
	default:
		return nil, fmt.Errorf("unknown STORAGE_DRIVER %q (want s3 or memory)", cfg.Driver)
	}
}
