// Package dto holds contacts' HTTP request and response shapes and the converters from domain types.
package dto

import (
	"encoding/base64"
	"strings"

	"github.com/google/uuid"
)

type WorkspaceInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
}

// EncodeCursor packs the last id of a page into an opaque string.
func EncodeCursor(id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString(id[:])
}

// DecodeCursor reads a cursor made by EncodeCursor.
func DecodeCursor(s string) (uuid.UUID, bool) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return uuid.Nil, false
	}
	id, err := uuid.FromBytes(b)
	return id, err == nil
}

func optional(s string) *string {
	if s = strings.TrimSpace(s); s != "" {
		return &s
	}
	return nil
}

func optionalID(s string) *uuid.UUID {
	if id, err := uuid.Parse(s); err == nil {
		return &id
	}
	return nil
}
