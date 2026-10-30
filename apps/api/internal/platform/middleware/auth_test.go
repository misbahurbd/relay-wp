package middleware

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/httpx"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/jwt"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

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

type fakeVerifier struct{ claims jwt.Claims }

func (f fakeVerifier) Verify(token string) (jwt.Claims, error) {
	if token != "good" {
		return jwt.Claims{}, jwt.ErrInvalidToken
	}
	return f.claims, nil
}

type fakeSessions struct {
	active bool
	err    error
}

func (f fakeSessions) SessionActive(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return f.active, f.err
}

func TestRequireAuthChecksSession(t *testing.T) {
	httpx.InstallErrorHandler()
	v := fakeVerifier{claims: jwt.Claims{UserID: uuid.New(), SessionID: uuid.New()}}
	tests := []struct {
		name     string
		token    string
		sessions fakeSessions
		want     int
	}{
		{"active session", "good", fakeSessions{active: true}, http.StatusNoContent},
		{"revoked or expired session", "good", fakeSessions{active: false}, http.StatusUnauthorized},
		{"session lookup fails", "good", fakeSessions{err: errors.New("db down")}, http.StatusServiceUnavailable},
		{"bad token never reaches the session check", "bad", fakeSessions{err: errors.New("must not be called")}, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, api := humatest.New(t)
			api.UseMiddleware(RequireAuth(api, v, tt.sessions, nil, nil))
			huma.Get(api, "/me", func(ctx context.Context, _ *struct{}) (*struct{}, error) {
				if sid, _ := authctx.SessionID(ctx); sid != v.claims.SessionID {
					t.Error("session id not in context")
				}
				return nil, nil
			})
			if got := api.Get("/me", "Authorization: Bearer "+tt.token).Code; got != tt.want {
				t.Errorf("status = %d, want %d", got, tt.want)
			}
		})
	}
}
