// Package profiles owns what the frontend shows about a person (user_profiles)
// and how the app behaves for them (user_preferences). Auth data lives in features/auth.
package profiles

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
