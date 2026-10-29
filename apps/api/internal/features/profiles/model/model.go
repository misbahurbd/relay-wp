// Package model holds profiles' domain types, constants and errors, shared by every layer of the
// feature.
package model

import "github.com/google/uuid"

// Me is everything the frontend needs about the signed-in user.
type Me struct {
	UserID        uuid.UUID
	Email         string
	EmailVerified bool
	HasPassword   bool

	FirstName   string
	LastName    *string
	DisplayName *string
	AvatarURL   *string

	Locale   string
	Timezone string
}

// ProfileUpdate changes profile fields. nil keeps a field; "" clears an optional one.
type ProfileUpdate struct {
	FirstName   *string
	LastName    *string
	DisplayName *string
}

// PreferencesUpdate changes preferences. nil keeps a field.
type PreferencesUpdate struct {
	Locale   *string
	Timezone *string
}
