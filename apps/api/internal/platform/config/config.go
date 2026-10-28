// Package config loads typed configuration from environment variables.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config is the full API configuration. Every field maps to one environment variable.
type Config struct {
	AppEnv   string `env:"APP_ENV" envDefault:"development"`
	HTTPAddr string `env:"HTTP_ADDR" envDefault:":8080"`

	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`

	Database Database
	HTTP     HTTP
	Queue    Queue
	Mailer   Mailer
	Storage  Storage
}

// Database holds both connection strings. See docs/ARCHITECTURE.md (resource budget).
type Database struct {
	// URL may go through a transaction-mode pooler; used for queries.
	URL string `env:"DATABASE_URL,required,notEmpty"`
	// DirectURL must bypass any pooler; used by River (LISTEN/NOTIFY) and migrations.
	DirectURL      string `env:"DATABASE_DIRECT_URL,required,notEmpty"`
	MaxConns       int32  `env:"DATABASE_MAX_CONNS" envDefault:"10"`
	DirectMaxConns int32  `env:"DATABASE_DIRECT_MAX_CONNS" envDefault:"4"`
}

// HTTP configures the server and its middleware.
type HTTP struct {
	CORSAllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS" envSeparator:","`
	// TrustedProxies are CIDRs whose X-Forwarded-For header is believed.
	TrustedProxies    []string      `env:"TRUSTED_PROXIES" envSeparator:","`
	RateLimitRequests int           `env:"RATE_LIMIT_REQUESTS" envDefault:"300"`
	RateLimitWindow   time.Duration `env:"RATE_LIMIT_WINDOW" envDefault:"1m"`
	ShutdownTimeout   time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"10s"`
}

// Queue configures River workers.
type Queue struct {
	MaxWorkers int `env:"QUEUE_MAX_WORKERS" envDefault:"10"`
}

// Mailer configures outgoing email.
type Mailer struct {
	Driver   string `env:"MAILER_DRIVER" envDefault:"log"`
	From     string `env:"MAILER_FROM" envDefault:"no-reply@localhost"`
	FromName string `env:"MAILER_FROM_NAME" envDefault:"Relay"`
	SMTPHost string `env:"SMTP_HOST" envDefault:"localhost"`
	SMTPPort int    `env:"SMTP_PORT" envDefault:"1025"`
	SMTPUser string `env:"SMTP_USERNAME"`
	SMTPPass string `env:"SMTP_PASSWORD"`
	// SMTPTLS is "none", "starttls" or "tls".
	SMTPTLS string `env:"SMTP_TLS" envDefault:"none"`
}

// Storage configures object storage.
type Storage struct {
	Driver      string `env:"STORAGE_DRIVER" envDefault:"memory"`
	S3Endpoint  string `env:"S3_ENDPOINT"`
	S3Region    string `env:"S3_REGION" envDefault:"us-east-1"`
	S3AccessKey string `env:"S3_ACCESS_KEY"`
	S3SecretKey string `env:"S3_SECRET_KEY"`
	S3Bucket    string `env:"S3_BUCKET"`
	// S3PathStyle is required by most S3-compatible servers (RustFS, MinIO).
	S3PathStyle bool `env:"S3_PATH_STYLE" envDefault:"true"`
}

// IsDevelopment reports whether the app runs locally.
func (c Config) IsDevelopment() bool { return c.AppEnv == "development" }

// Load reads the configuration from the environment.
func Load() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}
