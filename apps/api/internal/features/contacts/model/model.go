// Package model holds contacts' domain types, constants and errors, shared by every layer of the
// feature. Contacts owns the people a workspace messages, their tags and contact imports
// (docs/PHASE-4-CONTACTS-TEMPLATES.md AD-2 to AD-5).
package model

import "github.com/google/uuid"

// Opt-in states. Only "in" contacts may get marketing messages.
const (
	OptInIn      = "in"
	OptInOut     = "out"
	OptInPending = "pending"
)

// Where a contact came from.
const (
	SourceManual = "manual"
	SourceImport = "import"
	SourceInbox  = "inbox"
	SourceAPI    = "api"
)

// PlanResourceContacts is the billing resource that caps a workspace's contacts (ADR 0008).
const PlanResourceContacts = "contacts"

// OptIns are the allowed opt-in states.
var OptIns = []string{OptInIn, OptInOut, OptInPending}

// Person is a workspace member a contact is assigned to.
type Person struct {
	ID   uuid.UUID
	Name string
}

// TagRef is a tag on a contact.
type TagRef struct {
	ID   uuid.UUID
	Name string
}
