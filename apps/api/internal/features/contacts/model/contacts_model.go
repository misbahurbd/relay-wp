package model

import (
	"time"

	"github.com/google/uuid"
)

// Last-message windows for the list filter.
const (
	LastMessage24h    = "24h"
	LastMessage7d     = "7d"
	LastMessage30d    = "30d"
	LastMessageOver90 = "over_90d"
)

// Bulk actions.
const (
	BulkAddTag    = "add_tag"
	BulkRemoveTag = "remove_tag"
	BulkAssign    = "assign"
	BulkDelete    = "delete"
)

// Limits on a contact's fields.
const (
	MaxNameLen       = 200
	MaxLanguageLen   = 35
	MaxAttributes    = 50
	MaxAttributeKey  = 64
	MaxAttributeVal  = 500
	MaxBulkContacts  = 500
	DefaultPageLimit = 25
	MaxPageLimit     = 100
)

// Contact is one row of the contacts list.
type Contact struct {
	ID            uuid.UUID
	Name          string
	Phone         string // E.164
	PhoneDisplay  string // international format, for people
	Email         *string
	OptIn         string
	Source        string
	Language      *string
	AssignedTo    *Person
	LastMessageAt *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Tags          []TagRef
}

// ContactDetail is the full contact.
type ContactDetail struct {
	Contact
	OptInSource *string
	OptInAt     *time.Time
	Attributes  map[string]string
}

// Filter narrows the contacts list and the export.
type Filter struct {
	Query       *string
	OptIn       *string
	TagID       *uuid.UUID
	LastMessage *string
	// After is the last id of the previous page.
	After *uuid.UUID
	Limit int32
}

// Page is one page of contacts.
type Page struct {
	Items []Contact
	Next  *uuid.UUID
	Total int64
}

// Summary is the contacts page header.
type Summary struct {
	Total   int64
	OptedIn int64
}

// ContactInput creates or changes a contact. A nil field is left as it is
// (on create: the default); an empty Email or Language clears it.
type ContactInput struct {
	Name       *string
	Phone      *string
	Email      *string
	OptIn      *string
	Language   *string
	Attributes map[string]string // nil keeps; non-nil replaces
	TagIDs     *[]uuid.UUID      // nil keeps; non-nil replaces
}

// Bulk is one action on many contacts.
type Bulk struct {
	Action     string
	ContactIDs []uuid.UUID
	TagID      *uuid.UUID
	UserID     *uuid.UUID
}
