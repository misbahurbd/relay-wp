package auth

type resetPasswordInput struct {
	RequestHeaders
	Body struct {
		Token       string `json:"token" minLength:"1" maxLength:"256"`
		NewPassword string `json:"new_password" minLength:"1" maxLength:"128"`
	}
}

type changePasswordInput struct {
	RequestHeaders
	Body struct {
		CurrentPassword string `json:"current_password" minLength:"1" maxLength:"128"`
		NewPassword     string `json:"new_password" minLength:"1" maxLength:"128"`
	}
}

type setPasswordInput struct {
	RequestHeaders
	Body struct {
		NewPassword string `json:"new_password" minLength:"1" maxLength:"128"`
	}
}
