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

// Case 94: production never starts on a payment provider's test keys.
func TestLoadRefusesTestPaymentKeysInProduction(t *testing.T) {
	keys := func(creem, stripe string) {
		t.Setenv("DATABASE_URL", "postgres://pooled")
		t.Setenv("DATABASE_DIRECT_URL", "postgres://direct")
		t.Setenv("CREEM_API_KEY", creem)
		t.Setenv("STRIPE_SECRET_KEY", stripe)
	}
	for _, c := range []struct {
		env, creem, stripe string
		ok                 bool
	}{
		{"production", "creem_test_abc", "", false},
		{"production", "", "sk_test_abc", false},
		{"production", "", "rk_test_abc", false},
		{"production", "creem_abc", "sk_live_abc", true},
		{"development", "creem_test_abc", "sk_test_abc", true},
		{"staging", "creem_test_abc", "sk_test_abc", true},
	} {
		t.Setenv("APP_ENV", c.env)
		keys(c.creem, c.stripe)
		if _, err := config.Load(); (err == nil) != c.ok {
			t.Errorf("APP_ENV=%s creem=%q stripe=%q: err = %v, want ok %v", c.env, c.creem, c.stripe, err, c.ok)
		}
	}
}

func TestPaymentModes(t *testing.T) {
	p := config.Payment{
		Providers:   []string{"stripe", " creem", "stripe:fake"},
		CreemAPIKey: "creem_test_abc", StripeSecretKey: "sk_live_abc",
	}
	got := p.Modes()
	want := map[string]string{"stripe": "live", "creem": "test", "stripe:fake": "fake"}
	if len(got) != len(want) {
		t.Fatalf("modes = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("mode of %s = %q, want %q", k, got[k], v)
		}
	}
}
