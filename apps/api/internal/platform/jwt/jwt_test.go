package jwt_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/jwt"
)

func manager(t *testing.T, kid string, prev ...string) (*jwt.Manager, string) {
	t.Helper()
	seed, pub, err := jwt.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	m, err := jwt.New(config.Auth{
		JWTSigningKey: seed, JWTSigningKeyID: kid, JWTPreviousPublicKeys: prev,
		JWTIssuer: "relay-api", JWTAudience: "relay", AccessTokenTTL: 15 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	return m, pub
}

func TestIssueAndVerify(t *testing.T) {
	m, _ := manager(t, "k1")
	uid, sid := uuid.New(), uuid.New()

	tok, exp, err := m.Issue(uid, sid, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c, err := m.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if c.UserID != uid || c.SessionID != sid || !c.ExpiresAt.Equal(exp.Truncate(time.Second)) {
		t.Errorf("claims = %+v", c)
	}
}

func TestVerifyRejects(t *testing.T) {
	m, _ := manager(t, "k1")
	other, _ := manager(t, "k1")
	uid, sid := uuid.New(), uuid.New()

	expired, _, _ := m.Issue(uid, sid, time.Now().Add(-time.Hour))
	foreign, _, _ := other.Issue(uid, sid, time.Now())

	for name, tok := range map[string]string{"expired": expired, "wrong key": foreign, "garbage": "a.b.c"} {
		if _, err := m.Verify(tok); !errors.Is(err, jwt.ErrInvalidToken) {
			t.Errorf("%s: err = %v, want ErrInvalidToken", name, err)
		}
	}
}

func TestPreviousKeyStillVerifies(t *testing.T) {
	old, oldPub := manager(t, "old")
	tok, _, _ := old.Issue(uuid.New(), uuid.New(), time.Now())

	rotated, _ := manager(t, "new", "old:"+oldPub)
	if _, err := rotated.Verify(tok); err != nil {
		t.Errorf("token signed with previous key rejected: %v", err)
	}
}

func TestNewRejectsBadConfig(t *testing.T) {
	if _, err := jwt.New(config.Auth{}); err == nil {
		t.Error("expected error without a key")
	}
	if _, err := jwt.New(config.Auth{JWTSigningKey: "bm9wZQ==", JWTSigningKeyID: "k"}); err == nil {
		t.Error("expected error for a short key")
	}
}
