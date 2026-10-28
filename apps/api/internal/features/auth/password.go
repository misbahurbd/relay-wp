package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

const (
	passwordMinLen = 8
	passwordMaxLen = 128
)

// argon2id parameters (OWASP baseline). Stored in each hash, so they can be raised later.
const (
	argonMemoryKiB = 19 * 1024
	argonTime      = 2
	argonThreads   = 1
	argonKeyLen    = 32
	argonSaltLen   = 16
)

// checkPasswordRules returns one detail per unmet rule, or nil.
func checkPasswordRules(pw string) []apperr.Detail {
	var d []apperr.Detail
	add := func(msg string) { d = append(d, apperr.Detail{Field: "password", Message: msg}) }

	if n := utf8.RuneCountInString(pw); n < passwordMinLen || n > passwordMaxLen {
		add(fmt.Sprintf("Use %d to %d characters", passwordMinLen, passwordMaxLen))
	}
	var lower, upper, digit, symbol bool
	for _, r := range pw {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		case !unicode.IsLetter(r) && !unicode.IsSpace(r):
			symbol = true
		}
	}
	if !lower {
		add("Add a lowercase letter")
	}
	if !upper {
		add("Add an uppercase letter")
	}
	if !digit {
		add("Add a number")
	}
	if !symbol {
		add("Add a symbol")
	}
	return d
}

// hashPassword returns an encoded argon2id hash: $argon2id$v=19$m=…,t=…,p=…$salt$key.
func hashPassword(pw string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("password salt: %w", err)
	}
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version,
		argonMemoryKiB, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

type argonParams struct {
	memory  uint32
	time    uint32
	threads uint8
}

// verifyPassword reports whether pw matches encoded, and whether the hash
// should be upgraded to the current parameters.
func verifyPassword(pw, encoded string) (match, needsRehash bool, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, false, errors.New("password: unsupported hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, false, errors.New("password: unsupported argon2 version")
	}
	var p argonParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return false, false, fmt.Errorf("password: parameters: %w", err)
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[4])
	want, err2 := base64.RawStdEncoding.DecodeString(parts[5])
	if err1 != nil || err2 != nil || len(want) == 0 {
		return false, false, errors.New("password: malformed hash")
	}

	got := argon2.IDKey([]byte(pw), salt, p.time, p.memory, p.threads, uint32(len(want))) //nolint:gosec // len(want) is a small key length
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return false, false, nil
	}
	stale := p.memory != argonMemoryKiB || p.time != argonTime || p.threads != argonThreads || len(want) != argonKeyLen
	return true, stale, nil
}
