package team

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/mailer"
)

func inviteEmail(to, inviter, workspace, role, note, link string, expires time.Time) mailer.Message {
	intro := fmt.Sprintf("%s invited you to join %s on Relay as %s.", inviter, workspace, article(role))
	text := intro
	htmlBody := "<p>" + html.EscapeString(intro) + "</p>"
	if note = strings.TrimSpace(note); note != "" {
		text += "\n\n“" + note + "”"
		htmlBody += "<blockquote>" + html.EscapeString(note) + "</blockquote>"
	}
	footer := fmt.Sprintf("This link expires on %s and only works for %s. If you weren't expecting this, you can ignore it.",
		expires.Format("Jan 2, 2006"), to)
	return mailer.Message{
		To:      []string{to},
		Subject: fmt.Sprintf("%s invited you to %s on Relay", inviter, workspace),
		Text:    fmt.Sprintf("%s\n\nAccept invite: %s\n\n%s\n", text, link, footer),
		HTML: fmt.Sprintf(`%s<p><a href="%s">Accept invite</a></p><p>%s</p>`,
			htmlBody, html.EscapeString(link), html.EscapeString(footer)),
	}
}

func joinedEmail(to, who, workspace, role, link string) mailer.Message {
	intro := fmt.Sprintf("%s accepted your invite and joined %s as %s.", who, workspace, article(role))
	return simple(to, who+" joined "+workspace, intro, "Open Team", link)
}

func renewalEmail(to, who, workspace, link string) mailer.Message {
	intro := fmt.Sprintf("%s tried to join %s, but their invite had expired. Send them a new one from the Team page.", who, workspace)
	return simple(to, who+" needs a new invite", intro, "Open Team", link)
}

func simple(to, subject, intro, action, link string) mailer.Message {
	return mailer.Message{
		To:      []string{to},
		Subject: subject,
		Text:    fmt.Sprintf("%s\n\n%s: %s\n", intro, action, link),
		HTML: fmt.Sprintf(`<p>%s</p><p><a href="%s">%s</a></p>`,
			html.EscapeString(intro), html.EscapeString(link), html.EscapeString(action)),
	}
}

// article puts "a" or "an" before a role name: "an Admin", "a Viewer".
func article(role string) string {
	if role != "" && strings.ContainsRune("AEIOUaeiou", rune(role[0])) {
		return "an " + role
	}
	return "a " + role
}
