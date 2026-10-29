package dto

import (
	"net/http"

	"github.com/google/uuid"
)

type InviteSignupInput struct {
	RequestHeaders
	Body struct {
		Token       string  `json:"token" minLength:"20" maxLength:"100" doc:"From the invite link's #t= fragment"`
		Password    string  `json:"password" minLength:"1" maxLength:"128"`
		FirstName   string  `json:"first_name" minLength:"1" maxLength:"100"`
		LastName    *string `json:"last_name,omitempty" maxLength:"100"`
		Timezone    string  `json:"timezone,omitempty" maxLength:"64"`
		AcceptTerms bool    `json:"accept_terms"`
	}
}

type InviteSignupOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      struct {
		WorkspaceID *uuid.UUID `json:"workspace_id" nullable:"true" doc:"Null when the account was made but joining failed"`
		JoinError   *string    `json:"join_error" doc:"Why joining failed; the person is signed in anyway"`
	}
}
