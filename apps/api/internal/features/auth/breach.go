package auth

import (
	"bufio"
	"context"
	"crypto/sha1" //nolint:gosec // the Have I Been Pwned range API is defined in terms of SHA-1
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// BreachChecker reports whether a password appears in known data breaches.
type BreachChecker interface {
	IsBreached(ctx context.Context, password string) (bool, error)
}

// hibp queries the Have I Been Pwned range API. Only the first 5 hex characters
// of the password's SHA-1 leave the server (k-anonymity); the password never does.
type hibp struct {
	client  *http.Client
	baseURL string
}

// NewHIBP returns a BreachChecker backed by api.pwnedpasswords.com.
func NewHIBP() BreachChecker {
	return &hibp{client: &http.Client{Timeout: 3 * time.Second}, baseURL: "https://api.pwnedpasswords.com"}
}

func (h *hibp) IsBreached(ctx context.Context, password string) (bool, error) {
	sum := sha1.Sum([]byte(password)) //nolint:gosec // see import comment
	full := strings.ToUpper(hex.EncodeToString(sum[:]))
	prefix, suffix := full[:5], full[5:]

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.baseURL+"/range/"+prefix, nil)
	if err != nil {
		return false, fmt.Errorf("breach check: %w", err)
	}
	req.Header.Set("Add-Padding", "true") // hides which prefix was really asked for
	req.Header.Set("User-Agent", "relay-api")

	resp, err := h.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("breach check: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("breach check: status %d", resp.StatusCode)
	}

	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		s, count, ok := strings.Cut(strings.TrimSpace(sc.Text()), ":")
		// Padding entries have a count of 0.
		if ok && s == suffix && count != "0" {
			return true, nil
		}
	}
	if err := sc.Err(); err != nil {
		return false, fmt.Errorf("breach check: read: %w", err)
	}
	return false, nil
}
