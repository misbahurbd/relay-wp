// Package dto holds team's HTTP request and response shapes and the converters from domain types.
package dto

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ---------- shared inputs ----------

type WorkspaceInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
}

type MemberPath struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	UserID      uuid.UUID `path:"userId"`
}

// metaHeaders is mixed into inputs that change access, for the access log.
type MetaHeaders struct {
	UserAgent string `header:"User-Agent"`
}

// ---------- helpers ----------

// encodeCursor packs a (time, id) pair into an opaque string.
func EncodeCursor(t time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(t.Format(time.RFC3339Nano) + "|" + id.String()))
}

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

func OptionalID(s string) *uuid.UUID {
	if id, err := uuid.Parse(s); err == nil {
		return &id
	}
	return nil
}

func Optional(s string) *string {
	if s = strings.TrimSpace(s); s != "" {
		return &s
	}
	return nil
}

func NonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nonNilIDs(s []uuid.UUID) []uuid.UUID {
	if s == nil {
		return []uuid.UUID{}
	}
	return s
}

func rawOrNil(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}

func itoa(n int32) string { return strconv.FormatInt(int64(n), 10) }
