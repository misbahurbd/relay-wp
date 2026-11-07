package payment

import (
	"errors"
	"fmt"
	"testing"
)

func TestPermanentErrors(t *testing.T) {
	cases := map[error]bool{
		ErrInvalidRequest: true, ErrDeclined: true,
		ErrUnavailable: false, ErrNotFound: false, ErrUnsupported: false, errors.New("network"): false, nil: false,
	}
	for err, want := range cases {
		if got := Permanent(fmt.Errorf("wrapped: %w", err)); err != nil && got != want {
			t.Errorf("Permanent(%v) = %v, want %v", err, got, want)
		}
	}
	if Permanent(nil) {
		t.Error("Permanent(nil) = true")
	}
}
