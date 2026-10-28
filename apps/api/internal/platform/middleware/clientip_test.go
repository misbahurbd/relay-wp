package middleware

import (
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	r, err := NewClientIPResolver([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name, remote, xff, want string
	}{
		{"untrusted peer ignores header", "203.0.113.9:1234", "1.2.3.4", "203.0.113.9"},
		{"trusted peer uses header", "10.0.0.5:1234", "198.51.100.7", "198.51.100.7"},
		{"skips trusted hops", "10.0.0.5:1234", "198.51.100.7, 10.1.1.1", "198.51.100.7"},
		{"spoofed left entries ignored", "10.0.0.5:1234", "6.6.6.6, 198.51.100.7", "198.51.100.7"},
		{"trusted peer without header", "10.0.0.5:1234", "", "10.0.0.5"},
		{"garbage header falls back", "10.0.0.5:1234", "not-an-ip", "10.0.0.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), "GET", "/", nil)
			req.RemoteAddr = tt.remote
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if got := r.resolve(req); got != tt.want {
				t.Errorf("resolve() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewClientIPResolverRejectsBadCIDR(t *testing.T) {
	if _, err := NewClientIPResolver([]string{"10.0.0.0/99"}); err == nil {
		t.Error("expected error")
	}
}
