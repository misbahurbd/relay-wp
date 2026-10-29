package service

import (
	"fmt"
	"html"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/mailer"
)

// installmentDueEmail tells the billing contact an app_link installment is due.
// It links to the billing page, whose Pay now starts the checkout.
func installmentDueEmail(to string, seq, count int, amountMinor int64, currency string, due time.Time, link string) mailer.Message {
	intro := fmt.Sprintf("Installment %d of %d for your Relay lifetime deal, %s, is due on %s.",
		seq, count, money(amountMinor, currency), due.Format("Jan 2, 2006"))
	footer := "Your plan stays active while it is paid within the grace period."
	return mailer.Message{
		To:      []string{to},
		Subject: fmt.Sprintf("Relay: installment %d of %d is due", seq, count),
		Text:    fmt.Sprintf("%s\n\nPay now: %s\n\n%s\n", intro, link, footer),
		HTML: fmt.Sprintf(`<p>%s</p><p><a href="%s">Pay now</a></p><p>%s</p>`,
			html.EscapeString(intro), html.EscapeString(link), html.EscapeString(footer)),
	}
}

// money formats a minor-unit amount in a two-decimal currency: "USD 103.33".
func money(minor int64, currency string) string {
	sign := ""
	if minor < 0 {
		sign, minor = "-", -minor
	}
	return fmt.Sprintf("%s %s%d.%02d", currency, sign, minor/100, minor%100)
}
