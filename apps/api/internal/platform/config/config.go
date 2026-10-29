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

	// AppBaseURL is the web app's address, used for links in emails.
	AppBaseURL string `env:"APP_BASE_URL" envDefault:"http://localhost:3000"`

	Database Database
	HTTP     HTTP
	Queue    Queue
	Mailer   Mailer
	Storage  Storage
	Auth     Auth
	Trial    Trial
	Admin    Admin
	WhatsApp WhatsApp
	Payment  Payment
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

// Auth configures tokens, cookies and password checks.
// JWT keys are not "required" here so commands like migrate run without them;
// jwt.New validates them when the API starts.
type Auth struct {
	JWTSigningKey   string `env:"JWT_SIGNING_KEY"`
	JWTSigningKeyID string `env:"JWT_SIGNING_KEY_ID"`
	// JWTPreviousPublicKeys are "kid:base64-public-key" pairs still accepted during key rotation.
	JWTPreviousPublicKeys []string      `env:"JWT_PREVIOUS_PUBLIC_KEYS" envSeparator:","`
	JWTIssuer             string        `env:"JWT_ISSUER" envDefault:"relay-api"`
	JWTAudience           string        `env:"JWT_AUDIENCE" envDefault:"relay"`
	AccessTokenTTL        time.Duration `env:"ACCESS_TOKEN_TTL" envDefault:"15m"`
	RefreshTokenTTL       time.Duration `env:"REFRESH_TOKEN_TTL" envDefault:"720h"`
	// SessionMaxAge ends a session this long after sign-in, however often it refreshes.
	SessionMaxAge time.Duration `env:"SESSION_MAX_AGE" envDefault:"2160h"`

	CookieDomain string `env:"COOKIE_DOMAIN"`
	CookieSecure bool   `env:"COOKIE_SECURE" envDefault:"true"`

	PasswordBreachCheck bool `env:"PASSWORD_BREACH_CHECK" envDefault:"true"`

	// APIBaseURL builds OAuth callback URLs: {API_BASE_URL}/v1/auth/oauth/{provider}/callback.
	APIBaseURL string `env:"API_BASE_URL" envDefault:"http://localhost:8080"`
	// AllowedRedirects are the exact URLs a social sign-in may return to
	// (web pages and mobile deep links). APP_BASE_URL is always allowed.
	AllowedRedirects []string `env:"AUTH_ALLOWED_REDIRECTS" envSeparator:","`
	// ReauthMaxAge is how recent a sign-in must be for sensitive actions
	// (set a password, connect or disconnect a provider).
	ReauthMaxAge time.Duration `env:"REAUTH_MAX_AGE" envDefault:"10m"`
	// EmailCodeKey is a base64 32-byte HMAC key for sign-up verification codes.
	// Development makes a random one when empty.
	EmailCodeKey string `env:"EMAIL_CODE_KEY"`
	// TermsVersion is stored with each sign-up's acceptance of the terms.
	TermsVersion string `env:"TERMS_VERSION" envDefault:"2026-10"`

	// Social providers: one with no credentials is simply not offered.
	GoogleClientID     string `env:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `env:"GOOGLE_CLIENT_SECRET"`
	FacebookAppID      string `env:"FACEBOOK_APP_ID"`
	FacebookAppSecret  string `env:"FACEBOOK_APP_SECRET"`
}

// Admin configures the staff-only admin tools (/v1/admin and the web /admin page).
type Admin struct {
	// Emails are the verified accounts allowed in; empty turns the admin tools off.
	Emails []string `env:"ADMIN_EMAILS" envSeparator:","`
}

// Trial configures the free trial a new workspace starts with.
type Trial struct {
	// Plan is the code of the catalog plan a new workspace trials. Its
	// trial_days set the length; 0, a card-required trial or no such plan
	// means new workspaces start on the free plan.
	Plan string `env:"TRIAL_PLAN" envDefault:"growth"`
}

// WhatsApp configures the Meta app used for Embedded Signup, the Cloud API and webhooks.
type WhatsApp struct {
	// Enabled off: the connect endpoints refuse and setup can only skip the WhatsApp step.
	Enabled            bool   `env:"WHATSAPP_ENABLED" envDefault:"false"`
	AppID              string `env:"META_APP_ID"`
	AppSecret          string `env:"META_APP_SECRET"`
	ESConfigID         string `env:"META_ES_CONFIG_ID"`
	GraphVersion       string `env:"META_GRAPH_VERSION" envDefault:"v24.0"`
	WebhookVerifyToken string `env:"META_WEBHOOK_VERIFY_TOKEN"`
	// TokenEncryptionKey is a base64 32-byte AES-256 key for stored Meta tokens and PINs.
	TokenEncryptionKey string `env:"TOKEN_ENCRYPTION_KEY"`
}

// Payment configures payment providers (docs/adr/0006-payment-gateway-registry.md).
// A listed provider with missing keys fails at startup; one not listed is never offered.
type Payment struct {
	// Providers are the enabled providers, e.g. "stripe,creem". In development,
	// "stripe:fake" registers the in-memory fake under Stripe's name.
	Providers []string `env:"PAYMENT_PROVIDERS" envSeparator:","`

	StripeSecretKey string `env:"STRIPE_SECRET_KEY"`
	// StripePublishableKey is safe for the browser (Stripe Elements).
	StripePublishableKey string `env:"STRIPE_PUBLISHABLE_KEY"`
	StripeWebhookSecret  string `env:"STRIPE_WEBHOOK_SECRET"`

	CreemAPIKey        string `env:"CREEM_API_KEY"`
	CreemWebhookSecret string `env:"CREEM_WEBHOOK_SECRET"`
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
