package profiles

import "github.com/google/uuid"

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

type meOutput struct {
	Body struct {
		User        userDTO        `json:"user"`
		Profile     profileDTO     `json:"profile"`
		Preferences preferencesDTO `json:"preferences"`
	}
}

func toMeOutput(m Me) *meOutput {
	out := &meOutput{}
	out.Body.User = userDTO{ID: m.UserID, Email: m.Email, EmailVerified: m.EmailVerified, HasPassword: m.HasPassword}
	out.Body.Profile = profileDTO{FirstName: m.FirstName, LastName: m.LastName, DisplayName: m.DisplayName, AvatarURL: m.AvatarURL}
	out.Body.Preferences = preferencesDTO{Locale: m.Locale, Timezone: m.Timezone}
	return out
}

type updateProfileInput struct {
	Body struct {
		FirstName   *string `json:"first_name,omitempty" maxLength:"100"`
		LastName    *string `json:"last_name,omitempty" maxLength:"100" doc:"Empty string clears it"`
		DisplayName *string `json:"display_name,omitempty" maxLength:"100" doc:"Empty string clears it"`
	}
}

type updatePreferencesInput struct {
	Body struct {
		Locale   *string `json:"locale,omitempty" maxLength:"10"`
		Timezone *string `json:"timezone,omitempty" maxLength:"64"`
	}
}
