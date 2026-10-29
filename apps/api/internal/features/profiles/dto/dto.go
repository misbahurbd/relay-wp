// Package dto holds profiles' HTTP request and response shapes and the converters from domain
// types.
package dto

import (
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/profiles/model"
)

type userDTO struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email" format:"email"`
	EmailVerified bool      `json:"email_verified"`
	HasPassword   bool      `json:"has_password" doc:"False for accounts that only use social sign-in"`
}

type profileDTO struct {
	FirstName   string  `json:"first_name"`
	LastName    *string `json:"last_name"`
	DisplayName *string `json:"display_name" doc:"When null, show first_name and last_name"`
	AvatarURL   *string `json:"avatar_url"`
}

type preferencesDTO struct {
	Locale   string `json:"locale" example:"en"`
	Timezone string `json:"timezone" example:"Asia/Dhaka"`
}

type MeOutput struct {
	Body struct {
		User        userDTO        `json:"user"`
		Profile     profileDTO     `json:"profile"`
		Preferences preferencesDTO `json:"preferences"`
	}
}

func ToMeOutput(m model.Me) *MeOutput {
	out := &MeOutput{}
	out.Body.User = userDTO{ID: m.UserID, Email: m.Email, EmailVerified: m.EmailVerified, HasPassword: m.HasPassword}
	out.Body.Profile = profileDTO{FirstName: m.FirstName, LastName: m.LastName, DisplayName: m.DisplayName, AvatarURL: m.AvatarURL}
	out.Body.Preferences = preferencesDTO{Locale: m.Locale, Timezone: m.Timezone}
	return out
}

type UpdateProfileInput struct {
	Body struct {
		FirstName   *string `json:"first_name,omitempty" maxLength:"100"`
		LastName    *string `json:"last_name,omitempty" maxLength:"100" doc:"Empty string clears it"`
		DisplayName *string `json:"display_name,omitempty" maxLength:"100" doc:"Empty string clears it"`
	}
}

type UpdatePreferencesInput struct {
	Body struct {
		Locale   *string `json:"locale,omitempty" maxLength:"10"`
		Timezone *string `json:"timezone,omitempty" maxLength:"64"`
	}
}
