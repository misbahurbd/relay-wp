// Package dto holds integrations' HTTP request and response shapes and the converters from domain
// types.
package dto

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/google/uuid"
)

type WorkspaceInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
}

// EncodeCursor packs a (time, id) pair into an opaque string.
func EncodeCursor(t time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(t.Format(time.RFC3339Nano) + "|" + id.String()))
}

// DecodeCursor reverses EncodeCursor; ok is false for anything it didn't make.
func DecodeCursor(s string) (time.Time, uuid.UUID, bool) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	ts, id, ok := strings.Cut(string(b), "|")
	if !ok {
		return time.Time{}, uuid.Nil, false
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	u, err := uuid.Parse(id)
	return t, u, err == nil
}
