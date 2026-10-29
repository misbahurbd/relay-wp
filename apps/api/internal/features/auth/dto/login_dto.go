package dto

type LoginInput struct {
	RequestHeaders
	Body struct {
		Email    string `json:"email" maxLength:"254"`
		Password string `json:"password" maxLength:"128"`
		Client   string `json:"client,omitempty" enum:"web,mobile" default:"web" doc:"web gets httpOnly cookies; mobile gets tokens in the body"`
	}
}
