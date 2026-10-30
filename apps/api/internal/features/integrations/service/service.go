// Package service holds integrations' business rules (no HTTP, no SQL): API keys, outgoing
// webhooks and store connections (docs/PHASE-5-INTEGRATIONS.md).
package service

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// Enqueuer inserts background jobs in a transaction.
type Enqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Config holds integrations settings from the environment.
type Config struct {
	// SecretKey is the AES-256 key for webhook secrets and store credentials (TOKEN_ENCRYPTION_KEY).
	SecretKey []byte
	// AllowPrivateTargets lets webhooks and store checks reach localhost and private
	// networks, and use http://. Development and tests only.
	AllowPrivateTargets bool
	// AppBaseURL is the web app; connect flows send the person back to its Integrations page.
	AppBaseURL string
	// APIBaseURL is this API's public address, for provider callback URLs.
	APIBaseURL string
	// ShopifyBaseURL replaces https://{shop} for Shopify calls; tests point it at a fake.
	ShopifyBaseURL string
	Google         GoogleConfig
	Shopify        ShopifyConfig
}

// Service manages API keys, webhook endpoints and deliveries, and store connections.
type Service struct {
	pool   *pgxpool.Pool
	queue  Enqueuer
	box    *secretBox
	cfg    Config
	log    *slog.Logger
	client *http.Client // for user-supplied URLs: no private addresses, no redirects
	google *http.Client
	now    func() time.Time
}

// New builds the service.
func New(pool *pgxpool.Pool, queue Enqueuer, cfg Config, log *slog.Logger) (*Service, error) {
	box, err := newSecretBox(cfg.SecretKey)
	if err != nil {
		return nil, err
	}
	return &Service{
		pool: pool, queue: queue, box: box, cfg: cfg, log: log,
		client: newSafeClient(cfg.AllowPrivateTargets),
		google: &http.Client{Timeout: 15 * time.Second},
		now:    func() time.Time { return time.Now().UTC() },
	}, nil
}
