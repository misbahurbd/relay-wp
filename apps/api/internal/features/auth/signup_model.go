package auth

// SignupInput is a new password account.
type SignupInput struct {
	Email       string
	Password    string
	FirstName   string
	LastName    *string
	Timezone    string
	AcceptTerms bool
}
