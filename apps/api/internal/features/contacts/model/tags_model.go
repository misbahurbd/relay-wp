package model

import "github.com/google/uuid"

// MaxTagLen is the longest tag name.
const MaxTagLen = 50

// Tag is a workspace's contact tag with how many contacts carry it.
type Tag struct {
	ID           uuid.UUID
	Name         string
	ContactCount int64
}
