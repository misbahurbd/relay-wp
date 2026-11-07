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

// invoiceReceiptEmail is the "payment received" email, with the invoice PDF
// attached. An invoice a discount made free thanks nobody for a payment.
func invoiceReceiptEmail(doc invoiceDoc, file []byte, link string) mailer.Message {
	intro := fmt.Sprintf("Thanks for your payment of %s. Your invoice %s is attached.", doc.Total, doc.Number)
	amountLabel, dateLabel := "Amount paid", "Paid on"
	if !doc.Charged {
		intro = fmt.Sprintf("Your invoice %s is attached. Nothing was charged.", doc.Number)
		amountLabel, dateLabel = "Amount", "Issued on"
	}
	return mailer.Message{
		To:      []string{doc.Email},
		Subject: fmt.Sprintf("Relay receipt %s · %s", doc.Number, doc.Total),
		Text: fmt.Sprintf("%s\n\n%s: %s\nFor: %s\n%s: %s\n\nBilling settings: %s\n",
			intro, amountLabel, doc.Total, doc.Description, dateLabel, doc.IssueDate, link),
		HTML: fmt.Sprintf(`<p>%s</p><p>%s: <strong>%s</strong><br>For: %s<br>%s: %s</p><p><a href="%s">Billing settings</a></p>`,
			html.EscapeString(intro), amountLabel, html.EscapeString(doc.Total), html.EscapeString(doc.Description),
			dateLabel, html.EscapeString(doc.IssueDate), html.EscapeString(link)),
		Attachments: []mailer.Attachment{{Filename: doc.Number + ".pdf", ContentType: "application/pdf", Data: file}},
	}
}

// currencySymbols are the currencies money writes with a symbol.
var currencySymbols = map[string]string{"USD": "$", "EUR": "€", "GBP": "£"}

// money formats a minor-unit amount in a two-decimal currency: "$103.33",
// or "BDT 103.33" for a currency without a symbol here.
func money(minor int64, currency string) string {
	sign := ""
	if minor < 0 {
		sign, minor = "-", -minor
	}
	if sym, ok := currencySymbols[currency]; ok {
		return fmt.Sprintf("%s%s%d.%02d", sign, sym, minor/100, minor%100)
	}
	return fmt.Sprintf("%s %s%d.%02d", currency, sign, minor/100, minor%100)
}
