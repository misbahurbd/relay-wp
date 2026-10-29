package service

import (
	"fmt"
	"html"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/mailer"
)

// Email bodies for auth flows. Links point at the web app (APP_BASE_URL).

// verifyEmail carries the 6-digit code the sign-up page asks for, plus a link for
// people who open the email on another device.
func verifyEmail(to, code, link string) mailer.Message {
	const footer = "The code expires in 10 minutes and the link in 24 hours. If you didn't create an account, you can ignore this email."
	return mailer.Message{
		To:      []string{to},
		Subject: code + " is your Relay verification code",
		Text: fmt.Sprintf("Your Relay verification code is %s. Enter it on the sign-up page to confirm your email address.\n\nOr verify with this link: %s\n\n%s\n",
			code, link, footer),
		HTML: fmt.Sprintf(`<p>Your Relay verification code is</p>`+
			`<p style="font-size:28px;font-weight:700;letter-spacing:6px">%s</p>`+
			`<p>Enter it on the sign-up page, or <a href="%s">verify with this link</a>.</p>`+
			`<p style="color:#666">%s</p>`,
			html.EscapeString(code), html.EscapeString(link), html.EscapeString(footer)),
	}
}

// confirmEmailCode proves an address typed during a social sign-in. No account
// exists for it yet, so there is no link: the code is entered in the same window.
func confirmEmailCode(to, code string) mailer.Message {
	const footer = "The code expires in 10 minutes. If you didn't try to sign in to Relay, you can ignore this email."
	return mailer.Message{
		To:      []string{to},
		Subject: code + " is your Relay confirmation code",
		Text:    fmt.Sprintf("Your Relay confirmation code is %s. Enter it where you signed in to confirm this email address.\n\n%s\n", code, footer),
		HTML: fmt.Sprintf(`<p>Your Relay confirmation code is</p>`+
			`<p style="font-size:28px;font-weight:700;letter-spacing:6px">%s</p>`+
			`<p>Enter it where you signed in to confirm this email address.</p>`+
			`<p style="color:#666">%s</p>`,
			html.EscapeString(code), html.EscapeString(footer)),
	}
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
