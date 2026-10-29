package service

import (
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/workspaces/model"

	"github.com/google/uuid"
)

func TestNormalizeWebsite(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{"", "", true},
		{"acme.com", "https://acme.com", true},
		{"  http://acme.com/shop ", "http://acme.com/shop", true},
		{"localhost", "", false},
		{"ftp://acme.com", "", false},
		{"https://user:pass@acme.com", "", false},
		{"not a site", "", false},
	}
	for _, tt := range tests {
		got, ok := normalizeWebsite(tt.in)
		if ok != tt.ok || (got != nil && *got != tt.want) || (got == nil && tt.want != "") {
			t.Errorf("normalizeWebsite(%q) = %v, %v; want %q, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestBusinessParams(t *testing.T) {
	p, err := businessParams(uuid.Nil, model.BusinessInput{
		Name: " Acme ", Industry: "travel", Country: "bd", TeamSize: "6_20", Goals: []string{"sales", "sales", "support"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Acme" || *p.Country != "BD" || len(p.Goals) != 2 || p.Website != nil {
		t.Errorf("params = %+v", p)
	}
	if _, err := businessParams(uuid.Nil, model.BusinessInput{Name: "Acme", Industry: "mining", Country: "BDX", TeamSize: "huge", Goals: []string{"fun"}}); err == nil {
		t.Fatal("invalid input accepted")
	}
}
