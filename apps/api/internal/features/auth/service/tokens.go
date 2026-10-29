package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/big"
)

// newToken returns a random 256-bit token for the client and its SHA-256 for the database.
// Only the hash is ever stored.
func newToken() (raw string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) []byte {
	h := sha256.Sum256([]byte(raw))
	return h[:]
}

// newEmailCode returns a random 6-digit code and its HMAC with the server key.
// A plain hash of one of a million codes could be reversed offline; the keyed
// HMAC can't without the key.
func newEmailCode(key []byte) (code string, hash []byte, err error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", nil, fmt.Errorf("email code: %w", err)
	}
	code = fmt.Sprintf("%06d", n.Int64())
	return code, hashEmailCode(key, code), nil
}

func hashEmailCode(key []byte, code string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(code))
	return m.Sum(nil)
}
