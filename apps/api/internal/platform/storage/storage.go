// Package storage stores files (objects). The implementation is chosen by STORAGE_DRIVER.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"path"
	"strings"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

// ErrNotFound is returned by Get when the key doesn't exist.
var ErrNotFound = errors.New("storage: object not found")

// Storage puts, gets and deletes objects by key.
type Storage interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	// SignedURL is a link that downloads key as filename until ttl passes,
	// straight from storage. It doesn't check that key exists.
	SignedURL(ctx context.Context, key, filename string, ttl time.Duration) (string, error)
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

// attachment is the Content-Disposition and Content-Type a signed link serves
// filename with. Quotes, backslashes and non-ASCII are dropped from the name.
func attachment(filename string) (disposition, contentType string) {
	safe := strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' {
			return -1
		}
		return r
	}, filename)
	contentType = mime.TypeByExtension(path.Ext(safe))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return `attachment; filename="` + safe + `"`, contentType
}
