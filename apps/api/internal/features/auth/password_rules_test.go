package auth

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestCheckPasswordRules(t *testing.T) {
	tests := []struct {
		pw   string
		want int // number of unmet rules
	}{
		{"Str0ng!pass", 0},
		{"Short1!", 1},                  // length
		{"alllowercase1!", 1},           // uppercase
		{"ALLUPPERCASE1!", 1},           // lowercase
		{"NoNumbers!!", 1},              // digit
		{"NoSymbols123", 1},             // symbol
		{"abc", 4},                      // length, upper, digit, symbol
		{strings.Repeat("Aa1!", 33), 1}, // 132 chars: too long
		{"Pässwörd1€", 0},               // non-ASCII letters and symbols count
	}
	for _, tt := range tests {
		if got := checkPasswordRules(tt.pw); len(got) != tt.want {
			t.Errorf("checkPasswordRules(%q) = %v, want %d unmet", tt.pw, got, tt.want)
		}
	}
}

func TestHashAndVerify(t *testing.T) {
	h, err := hashPassword("Str0ng!pass")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("unexpected hash format: %s", h)
	}

	if ok, rehash, err := verifyPassword("Str0ng!pass", h); err != nil || !ok || rehash {
		t.Errorf("correct password: ok=%v rehash=%v err=%v", ok, rehash, err)
	}
	if ok, _, err := verifyPassword("wrong", h); err != nil || ok {
		t.Errorf("wrong password: ok=%v err=%v", ok, err)
	}

	h2, _ := hashPassword("Str0ng!pass")
	if h == h2 {
		t.Error("two hashes of the same password must differ (random salt)")
	}
}

func TestVerifyFlagsOldParameters(t *testing.T) {
	// A real hash made with weaker parameters: still verifies, but must be upgraded.
	salt := []byte("0123456789abcdef")
	key := argon2.IDKey([]byte("Str0ng!pass"), salt, 1, 4096, 1, argonKeyLen)
	old := fmt.Sprintf("$argon2id$v=%d$m=4096,t=1,p=1$%s$%s", argon2.Version,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))

	ok, rehash, err := verifyPassword("Str0ng!pass", old)
	if err != nil || !ok || !rehash {
		t.Errorf("old hash: ok=%v rehash=%v err=%v, want ok and rehash", ok, rehash, err)
	}
	if _, _, err := verifyPassword("x", "$bcrypt$nope"); err == nil {
		t.Error("expected error for an unknown format")
	}
}
