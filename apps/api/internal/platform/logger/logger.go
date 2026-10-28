// Package logger builds the application's slog logger and carries request-scoped loggers in context.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// New builds a logger. format is "json" or "text"; level is "debug", "info", "warn" or "error".
func New(w io.Writer, format, level string) (*slog.Logger, error) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(strings.ToUpper(level))); err != nil {
		return nil, fmt.Errorf("unknown LOG_LEVEL %q", level)
	}
	opts := &slog.HandlerOptions{Level: lvl}

	switch format {
	case "json":
		return slog.New(slog.NewJSONHandler(w, opts)), nil
	case "text":
		return slog.New(slog.NewTextHandler(w, opts)), nil
	default:
		return nil, fmt.Errorf("unknown LOG_FORMAT %q (want json or text)", format)
	}
}

type ctxKey struct{}

// WithContext returns a copy of ctx carrying l.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns the request-scoped logger, or slog.Default if none is set.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
