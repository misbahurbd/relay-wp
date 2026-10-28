package auth

import (
	"fmt"
	"html"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/mailer"
)

// Email bodies for auth flows. Links point at the web app (APP_BASE_URL).

func verifyEmail(to, link string) mailer.Message {
	return message(to, "Verify your email address",
		"Confirm this is your email address to finish creating your Relay account.",
		"Verify email", link,
		"This link expires in 24 hours. If you didn't create an account, you can ignore this email.")
}

// finishSignupEmail goes to an address that already has an unverified account.
// It sets a new password instead of verifying the old one, so whoever created
// the account can't keep a password they chose.
func finishSignupEmail(to, link string) mailer.Message {
	return message(to, "Finish setting up your Relay account",
		"Someone tried to sign up with this email. An account already exists but was never confirmed. Set a password to finish setting it up.",
		"Set password", link,
		"This link expires in 1 hour. If you didn't try to sign up, you can ignore this email.")
}

func alreadyRegisteredEmail(to, signInLink string) mailer.Message {
	return message(to, "You already have a Relay account",
		"Someone tried to sign up with this email, but you already have an account. Sign in instead, or reset your password if you've forgotten it.",
		"Sign in", signInLink,
		"If this wasn't you, you can ignore this email. Your account is unchanged.")
}

func resetPasswordEmail(to, link string) mailer.Message {
	return message(to, "Reset your Relay password",
		"Use the link below to choose a new password. Every device signed in to your account will be signed out.",
		"Reset password", link,
		"This link expires in 1 hour. If you didn't ask to reset your password, you can ignore this email.")
}

func message(to, subject, intro, action, link, footer string) mailer.Message {
	return mailer.Message{
		To:      []string{to},
		Subject: subject,
		Text:    fmt.Sprintf("%s\n\n%s: %s\n\n%s\n", intro, action, link, footer),
		HTML: fmt.Sprintf(`<p>%s</p><p><a href="%s">%s</a></p><p style="color:#666">%s</p>`,
			html.EscapeString(intro), html.EscapeString(link), html.EscapeString(action), html.EscapeString(footer)),
	}
}
