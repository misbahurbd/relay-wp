package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"slices"
	"testing"
)

func TestNewProviders(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ProvidersConfig
		want    []string
		wantErr bool
	}{
		{"none configured", ProvidersConfig{}, []string{}, false},
		{"google only", ProvidersConfig{Google: GoogleConfig{ClientID: "id", ClientSecret: "s"}}, []string{"google"}, false},
		{"both", ProvidersConfig{
			Google:   GoogleConfig{ClientID: "id", ClientSecret: "s"},
			Facebook: FacebookConfig{AppID: "id", AppSecret: "s"},
		}, []string{"google", "facebook"}, false},
		{"google id without secret", ProvidersConfig{Google: GoogleConfig{ClientID: "id"}}, nil, true},
		{"facebook secret without id", ProvidersConfig{Facebook: FacebookConfig{AppSecret: "s"}}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewProviders(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !slices.Equal(p.Names(), tt.want) {
				t.Errorf("Names() = %v, want %v", p.Names(), tt.want)
			}
		})
	}
}

func TestParseSignedRequest(t *testing.T) {
	sign := func(secret, payload string) string {
		p := base64.RawURLEncoding.EncodeToString([]byte(payload))
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(p))
		return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) + "." + p
	}
	good := `{"algorithm":"HMAC-SHA256","user_id":"123"}`

	if id, err := parseSignedRequest(sign("secret", good), "secret"); err != nil || id != "123" {
		t.Errorf("valid request: id=%q err=%v", id, err)
	}
	for name, signed := range map[string]string{
		"wrong secret":    sign("other", good),
		"no user":         sign("secret", `{"algorithm":"HMAC-SHA256"}`),
		"wrong algorithm": sign("secret", `{"algorithm":"none","user_id":"123"}`),
		"no dot":          "abc",
		"garbage":         "!!!.???",
	} {
		if _, err := parseSignedRequest(signed, "secret"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
