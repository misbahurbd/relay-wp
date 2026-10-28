// Package jwt signs and verifies access tokens with Ed25519.
// Keys are identified by "kid", so a new key can be rolled out while tokens
// signed with the previous one stay valid until they expire.
package jwt

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

// ErrInvalidToken covers every reason a token is rejected (bad signature, expired, wrong audience…).
var ErrInvalidToken = errors.New("jwt: invalid token")

// Claims are the values an access token carries.
type Claims struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	ExpiresAt time.Time
}

type claims struct {
	SessionID string `json:"sid"`
	gojwt.RegisteredClaims
}

// Manager issues and verifies access tokens.
type Manager struct {
	kid      string
	private  ed25519.PrivateKey
	public   map[string]ed25519.PublicKey // by kid, current and previous
	issuer   string
	audience string
	ttl      time.Duration
}

// New loads keys from cfg. It fails if the signing key is missing or malformed.
func New(cfg config.Auth) (*Manager, error) {
	if cfg.JWTSigningKey == "" || cfg.JWTSigningKeyID == "" {
		return nil, errors.New("jwt: JWT_SIGNING_KEY and JWT_SIGNING_KEY_ID are required (generate with `relay keygen`)")
	}
	seed, err := base64.StdEncoding.DecodeString(cfg.JWTSigningKey)
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("jwt: JWT_SIGNING_KEY must be a base64 Ed25519 seed (32 bytes)")
	}
	priv := ed25519.NewKeyFromSeed(seed)

	m := &Manager{
		kid:      cfg.JWTSigningKeyID,
		private:  priv,
		public:   map[string]ed25519.PublicKey{cfg.JWTSigningKeyID: priv.Public().(ed25519.PublicKey)},
		issuer:   cfg.JWTIssuer,
		audience: cfg.JWTAudience,
		ttl:      cfg.AccessTokenTTL,
	}
	for _, entry := range cfg.JWTPreviousPublicKeys {
		kid, b64, ok := strings.Cut(strings.TrimSpace(entry), ":")
		if !ok || kid == "" {
			return nil, fmt.Errorf("jwt: JWT_PREVIOUS_PUBLIC_KEYS entry %q must be kid:base64", entry)
		}
		pub, err := base64.StdEncoding.DecodeString(b64)
		if err != nil || len(pub) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("jwt: previous public key %q is not a base64 Ed25519 public key", kid)
		}
		m.public[kid] = pub
	}
	return m, nil
}

// TTL is how long issued access tokens live.
func (m *Manager) TTL() time.Duration { return m.ttl }

// Issue signs an access token for a user's session.
func (m *Manager) Issue(userID, sessionID uuid.UUID, now time.Time) (string, time.Time, error) {
	exp := now.Add(m.ttl)
	jti, err := uuid.NewV7()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("jwt: token id: %w", err)
	}
	tok := gojwt.NewWithClaims(gojwt.SigningMethodEdDSA, claims{
		SessionID: sessionID.String(),
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    m.issuer,
			Audience:  gojwt.ClaimStrings{m.audience},
			IssuedAt:  gojwt.NewNumericDate(now),
			ExpiresAt: gojwt.NewNumericDate(exp),
			ID:        jti.String(),
		},
	})
	tok.Header["kid"] = m.kid
	signed, err := tok.SignedString(m.private)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("jwt: sign: %w", err)
	}
	return signed, exp, nil
}

// Verify checks signature, expiry, issuer and audience, and returns the claims.
func (m *Manager) Verify(token string) (Claims, error) {
	var c claims
	_, err := gojwt.ParseWithClaims(token, &c, func(t *gojwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		pub, ok := m.public[kid]
		if !ok {
			return nil, fmt.Errorf("unknown kid %q", kid)
		}
		return pub, nil
	},
		gojwt.WithValidMethods([]string{gojwt.SigningMethodEdDSA.Alg()}),
		gojwt.WithIssuer(m.issuer),
		gojwt.WithAudience(m.audience),
		gojwt.WithExpirationRequired(),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	uid, err1 := uuid.Parse(c.Subject)
	sid, err2 := uuid.Parse(c.SessionID)
	if err1 != nil || err2 != nil {
		return Claims{}, fmt.Errorf("%w: malformed subject or session", ErrInvalidToken)
	}
	return Claims{UserID: uid, SessionID: sid, ExpiresAt: c.ExpiresAt.Time}, nil
}

// GenerateKey returns a new base64 Ed25519 seed (for JWT_SIGNING_KEY) and its public key.
func GenerateKey() (seed, public string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("jwt: generate key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(priv.Seed()), base64.StdEncoding.EncodeToString(pub), nil
}
