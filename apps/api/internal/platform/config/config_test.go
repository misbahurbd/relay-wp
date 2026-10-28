package config_test

import (
	"testing"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

func TestLoadRequiresDatabaseURLs(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DATABASE_DIRECT_URL", "")

	if _, err := config.Load(); err == nil {
		t.Fatal("expected an error when database URLs are missing")
	}
}

func TestLoadParsesValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://pooled")
	t.Setenv("DATABASE_DIRECT_URL", "postgres://direct")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://a.test,http://b.test")
	t.Setenv("RATE_LIMIT_WINDOW", "30s")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.HTTP.CORSAllowedOrigins; len(got) != 2 || got[1] != "http://b.test" {
		t.Errorf("CORSAllowedOrigins = %v", got)
	}
	if cfg.HTTP.RateLimitWindow != 30*time.Second {
		t.Errorf("RateLimitWindow = %v", cfg.HTTP.RateLimitWindow)
	}
	if cfg.Database.MaxConns != 10 {
		t.Errorf("MaxConns default = %d", cfg.Database.MaxConns)
	}
}
