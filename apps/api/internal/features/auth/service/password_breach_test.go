package service

import (
	"context"
	"crypto/sha1" //nolint:gosec // matches the API under test
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHIBP(t *testing.T) {
	sum := sha1.Sum([]byte("Passw0rd!")) //nolint:gosec // test fixture
	full := strings.ToUpper(hex.EncodeToString(sum[:]))

	var gotPath, gotPadding string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotPadding = r.URL.Path, r.Header.Get("Add-Padding")
		_, _ = fmt.Fprintf(w, "0000000000000000000000000000000000A:0\r\n%s:42\r\n", full[5:])
	}))
	defer srv.Close()

	h := &hibp{client: srv.Client(), baseURL: srv.URL}

	breached, err := h.IsBreached(context.Background(), "Passw0rd!")
	if err != nil || !breached {
		t.Fatalf("breached=%v err=%v", breached, err)
	}
	if gotPath != "/range/"+full[:5] || gotPadding != "true" {
		t.Errorf("path=%s padding=%s", gotPath, gotPadding)
	}

	if breached, _ := h.IsBreached(context.Background(), "a-different-Passw0rd!"); breached {
		t.Error("unrelated password reported as breached")
	}
}
