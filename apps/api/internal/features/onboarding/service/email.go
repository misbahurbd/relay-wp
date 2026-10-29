package service

import (
	"fmt"
	"html"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/mailer"
)

func setupReminderEmail(to, workspace, link string) mailer.Message {
	return message(to, "Finish connecting WhatsApp to Relay",
		fmt.Sprintf("%s is almost ready. Connect your WhatsApp Business number to start sending messages; it takes about 5 minutes.", workspace),
		"Finish setup", link)
}

func trialReminderEmail(to, workspace string, daysLeft int, link string) mailer.Message {
	when := fmt.Sprintf("in %d days", daysLeft)
	if daysLeft == 1 {
		when = "tomorrow"
	}
	return message(to, "Your Relay trial ends "+when,
		fmt.Sprintf("The free trial for %s ends %s. Choose a plan to keep your inbox, broadcasts and automations running.", workspace, when),
		"Open Relay", link)
}

func message(to, subject, intro, action, link string) mailer.Message {
	return mailer.Message{
		To:      []string{to},
		Subject: subject,
		Text:    fmt.Sprintf("%s\n\n%s: %s\n", intro, action, link),
		HTML: fmt.Sprintf(`<p>%s</p><p><a href="%s">%s</a></p>`,
			html.EscapeString(intro), html.EscapeString(link), html.EscapeString(action)),
	}
}
