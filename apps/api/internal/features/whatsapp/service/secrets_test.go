package service

import (
	"bytes"
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
)

func TestSecretBoxRoundTrip(t *testing.T) {
	box, err := newSecretBox(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	a, err := box.seal("biz-token")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := box.seal("biz-token")
	if bytes.Equal(a, b) || bytes.Contains(a, []byte("biz-token")) {
		t.Fatal("sealing must be randomized and hide the plain text")
	}
	if got, err := box.open(a); err != nil || got != "biz-token" {
		t.Fatalf("open = %q, %v", got, err)
	}
	a[len(a)-1] ^= 1
	if _, err := box.open(a); err == nil {
		t.Fatal("tampered secret must not open")
	}
	other, _ := newSecretBox(bytes.Repeat([]byte{2}, 32))
	if _, err := other.open(b); err == nil {
		t.Fatal("wrong key must not open")
	}
}

func TestMetaMappings(t *testing.T) {
	for in, want := range map[string]string{"APPROVED": "approved", "AVAILABLE_WITHOUT_REVIEW": "approved", "DECLINED": "declined", "EXPIRED": "declined", "PENDING_REVIEW": "pending_review", "": "pending_review"} {
		if got := nameStatus(in); got != want {
			t.Errorf("nameStatus(%q) = %q, want %q", in, got, want)
		}
	}
	if q := qualityRating("GREEN"); q == nil || *q != "green" {
		t.Errorf("qualityRating(GREEN) = %v", q)
	}
	if q := qualityRating("UNKNOWN"); q != nil {
		t.Errorf("qualityRating(UNKNOWN) = %v", *q)
	}
	if got := model.Digits("+1 (415) 555-0142"); got != "14155550142" {
		t.Errorf("digits = %q", got)
	}
}
