package logger_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/logger"
)

func TestNewRejectsUnknownValues(t *testing.T) {
	if _, err := logger.New(&bytes.Buffer{}, "xml", "info"); err == nil {
		t.Error("expected error for unknown format")
	}
	if _, err := logger.New(&bytes.Buffer{}, "json", "loud"); err == nil {
		t.Error("expected error for unknown level")
	}
}

func TestNewRespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	l, err := logger.New(&buf, "json", "warn")
	if err != nil {
		t.Fatal(err)
	}
	l.Info("hidden")
	l.Warn("shown")
	if strings.Contains(buf.String(), "hidden") || !strings.Contains(buf.String(), "shown") {
		t.Errorf("unexpected output: %s", buf.String())
	}
}

func TestContextRoundTrip(t *testing.T) {
	l, _ := logger.New(&bytes.Buffer{}, "text", "info")
	ctx := logger.WithContext(context.Background(), l)
	if logger.FromContext(ctx) != l {
		t.Error("FromContext did not return the stored logger")
	}
}
