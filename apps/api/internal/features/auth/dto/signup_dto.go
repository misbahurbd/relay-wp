package dto

type SignupInput struct {
	RequestHeaders
	Body struct {
		Email     string  `json:"email" format:"email" maxLength:"254"`
		Password  string  `json:"password" minLength:"1" maxLength:"128"`
		FirstName string  `json:"first_name" minLength:"1" maxLength:"100"`
		LastName  *string `json:"last_name,omitempty" maxLength:"100"`
		Timezone  string  `json:"timezone,omitempty" maxLength:"64" doc:"IANA timezone from the browser, e.g. Asia/Dhaka. Defaults to UTC."`
		// AcceptTerms must be true: the person agreed to the terms, privacy policy and WhatsApp policy.
		AcceptTerms bool `json:"accept_terms"`
	}
}

type VerifyCodeInput struct {
	RequestHeaders
	Body struct {
		Email    string `json:"email" format:"email" maxLength:"254"`
		Code     string `json:"code" pattern:"^[0-9]{6}$" doc:"The 6-digit code from the verification email"`
		Password string `json:"password,omitempty" maxLength:"128" doc:"Required when the account has a password: the code alone doesn't prove who chose it"`
		Client   string `json:"client,omitempty" enum:"web,mobile" default:"web" doc:"web gets httpOnly cookies; mobile gets tokens in the body"`
	}
}

type VerifyEmailInput struct {
	RequestHeaders
	Body struct {
		Token    string `json:"token" minLength:"1" maxLength:"256"`
		Password string `json:"password,omitempty" maxLength:"128" doc:"Required when the account has a password: the link alone doesn't prove who chose it"`
	}
}
