package middleware

import "testing"

func TestSameOrigin(t *testing.T) {
	allowed := []string{"http://localhost:3000"}
	tests := []struct {
		name, method, origin, referer string
		want                          bool
	}{
		{"GET always passes", "GET", "", "", true},
		{"POST from allowed origin", "POST", "http://localhost:3000", "", true},
		{"POST from other origin", "POST", "https://evil.test", "", false},
		{"POST without origin or referer", "POST", "", "", false},
		{"POST falls back to referer", "POST", "", "http://localhost:3000/settings", true},
		{"POST with foreign referer", "DELETE", "", "https://evil.test/x", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SameOrigin(tt.method, tt.origin, tt.referer, allowed); got != tt.want {
				t.Errorf("SameOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBearerToken(t *testing.T) {
	for in, want := range map[string]string{"Bearer abc": "abc", "bearer abc": "abc", "Basic abc": "", "": "", "Bearer": ""} {
		if got := bearerToken(in); got != want {
			t.Errorf("bearerToken(%q) = %q, want %q", in, got, want)
		}
	}
}
