package service

import (
	"slices"
	"strings"
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// validDraft is a complete, valid template that each case changes one thing in.
func validDraft() model.Draft {
	return model.Draft{
		Name: "order_update", Language: "en_US", Category: model.CategoryUtility,
		Components: model.Components{
			Header:  model.Header{Type: model.HeaderText, Text: "Order {{1}}", Example: "#4821"},
			Body:    model.Body{Text: "Hi {{1}}, your order {{2}} has shipped.", Examples: []string{"Daniel", "#4821"}},
			Footer:  model.Footer{Text: "Reply STOP to opt out"},
			Buttons: []model.Button{{Type: model.ButtonURL, Text: "Track", Value: "https://acme.com/t"}},
		},
	}
}

// fields returns the field locations of a validation error, or nil when valid.
func fields(t *testing.T, err error) []string {
	t.Helper()
	if err == nil {
		return nil
	}
	ae, ok := apperr.As(err)
	if !ok || ae.Code != "invalid_template" {
		t.Fatalf("not invalid_template: %v", err)
	}
	out := make([]string, len(ae.Details))
	for i, d := range ae.Details {
		out[i] = d.Field
	}
	return out
}

func TestValidate(t *testing.T) {
	image := &model.Sample{ContentType: "image/png", Size: 100}
	tests := []struct {
		name     string
		edit     func(d *model.Draft)
		sample   *model.Sample
		complete bool
		want     string // field with a problem; "" means valid
	}{
		{"valid complete", func(*model.Draft) {}, nil, true, ""},
		{"valid draft with nothing", func(d *model.Draft) { d.Components = model.Components{} }, nil, false, ""},

		// name
		{"name uppercase", func(d *model.Draft) { d.Name = "Order" }, nil, false, "name"},
		{"name space", func(d *model.Draft) { d.Name = "order update" }, nil, false, "name"},
		{"name empty", func(d *model.Draft) { d.Name = "" }, nil, false, "name"},
		{"name 512", func(d *model.Draft) { d.Name = strings.Repeat("a", 512) }, nil, true, ""},
		{"name 513", func(d *model.Draft) { d.Name = strings.Repeat("a", 513) }, nil, false, "name"},
		{"name digits underscore", func(d *model.Draft) { d.Name = "promo_2026_v2" }, nil, true, ""},

		// language and category
		{"language unknown", func(d *model.Draft) { d.Language = "fr" }, nil, false, "language"},
		{"language pt_BR", func(d *model.Draft) { d.Language = "pt_BR" }, nil, true, ""},
		{"category marketing", func(d *model.Draft) { d.Category = model.CategoryMarketing }, nil, true, ""},
		{"category authentication", func(d *model.Draft) { d.Category = model.CategoryAuthentication }, nil, true, ""},
		{"category unknown", func(d *model.Draft) { d.Category = "promo" }, nil, false, "category"},

		// header
		{"header type unknown", func(d *model.Draft) { d.Components.Header.Type = "gif" }, nil, false, "header.type"},
		{"header text 60", func(d *model.Draft) {
			d.Components.Header = model.Header{Type: model.HeaderText, Text: strings.Repeat("a", 60)}
		}, nil, true, ""},
		{"header text 61", func(d *model.Draft) {
			d.Components.Header = model.Header{Type: model.HeaderText, Text: strings.Repeat("a", 61)}
		}, nil, false, "header.text"},
		{"header two variables", func(d *model.Draft) { d.Components.Header.Text = "{{1}} and {{2}}" }, nil, false, "header.text"},
		{"header variable not 1", func(d *model.Draft) { d.Components.Header.Text = "Order {{2}}" }, nil, false, "header.text"},
		{"header variable without example", func(d *model.Draft) { d.Components.Header.Example = "" }, nil, true, "header.example"},
		{"header variable without example in draft", func(d *model.Draft) { d.Components.Header.Example = "" }, nil, false, ""},
		{"header text empty on submit", func(d *model.Draft) {
			d.Components.Header = model.Header{Type: model.HeaderText}
		}, nil, true, "header.text"},
		{"media header without sample", func(d *model.Draft) {
			d.Components.Header = model.Header{Type: model.HeaderImage}
		}, nil, true, "header"},
		{"media header without sample in draft", func(d *model.Draft) {
			d.Components.Header = model.Header{Type: model.HeaderImage}
		}, nil, false, ""},
		{"media header with sample", func(d *model.Draft) {
			d.Components.Header = model.Header{Type: model.HeaderImage}
		}, image, true, ""},
		{"media header with wrong sample", func(d *model.Draft) {
			d.Components.Header = model.Header{Type: model.HeaderVideo}
		}, image, true, "header"},

		// body
		{"body empty on submit", func(d *model.Draft) { d.Components.Body = model.Body{} }, nil, true, "body.text"},
		{"body 1024", func(d *model.Draft) { d.Components.Body = model.Body{Text: strings.Repeat("a", 1024)} }, nil, true, ""},
		{"body 1025", func(d *model.Draft) { d.Components.Body = model.Body{Text: strings.Repeat("a", 1025)} }, nil, false, "body.text"},
		{"body gap", func(d *model.Draft) {
			d.Components.Body = model.Body{Text: "Hi {{1}}, order {{3}}.", Examples: []string{"a", "b"}}
		}, nil, false, "body.text"},
		{"body starts at 2", func(d *model.Draft) {
			d.Components.Body = model.Body{Text: "Hi {{2}} there", Examples: []string{"a"}}
		}, nil, false, "body.text"},
		{"body named variable", func(d *model.Draft) {
			d.Components.Body = model.Body{Text: "Hi {{name}} there", Examples: []string{"a"}}
		}, nil, false, "body.text"},
		{"body zero variable", func(d *model.Draft) {
			d.Components.Body = model.Body{Text: "Hi {{0}} there", Examples: []string{"a"}}
		}, nil, false, "body.text"},
		{"body repeated variable", func(d *model.Draft) {
			d.Components.Body = model.Body{Text: "Hi {{1}}, yes {{1}}!", Examples: []string{"a"}}
		}, nil, true, ""},
		{"body starts with variable", func(d *model.Draft) {
			d.Components.Body = model.Body{Text: "{{1}}, your order shipped", Examples: []string{"a"}}
		}, nil, false, "body.text"},
		{"body ends with variable", func(d *model.Draft) {
			d.Components.Body = model.Body{Text: "Your order is {{1}}", Examples: []string{"a"}}
		}, nil, false, "body.text"},
		{"body too few examples", func(d *model.Draft) { d.Components.Body.Examples = []string{"Daniel"} }, nil, true, "body.examples"},
		{"body too many examples", func(d *model.Draft) { d.Components.Body.Examples = []string{"a", "b", "c"} }, nil, true, "body.examples"},
		{"body missing examples in draft", func(d *model.Draft) { d.Components.Body.Examples = nil }, nil, false, ""},
		{"body empty example", func(d *model.Draft) { d.Components.Body.Examples = []string{"Daniel", " "} }, nil, true, "body.examples[1]"},

		// footer
		{"footer 60", func(d *model.Draft) { d.Components.Footer.Text = strings.Repeat("a", 60) }, nil, true, ""},
		{"footer 61", func(d *model.Draft) { d.Components.Footer.Text = strings.Repeat("a", 61) }, nil, false, "footer.text"},
		{"footer variable", func(d *model.Draft) { d.Components.Footer.Text = "Bye {{1}}" }, nil, false, "footer.text"},

		// buttons
		{"ten quick replies", func(d *model.Draft) { d.Components.Buttons = quickReplies(10) }, nil, true, ""},
		{"eleven buttons", func(d *model.Draft) { d.Components.Buttons = quickReplies(11) }, nil, false, "buttons"},
		{"three url buttons", func(d *model.Draft) {
			u := model.Button{Type: model.ButtonURL, Text: "Go", Value: "https://acme.com"}
			d.Components.Buttons = []model.Button{u, u, u}
		}, nil, false, "buttons"},
		{"two phone buttons", func(d *model.Draft) {
			p := model.Button{Type: model.ButtonPhone, Text: "Call", Value: "+14155550142"}
			d.Components.Buttons = []model.Button{p, p}
		}, nil, false, "buttons"},
		{"quick replies split", func(d *model.Draft) {
			q := model.Button{Type: model.ButtonQuickReply, Text: "Yes"}
			d.Components.Buttons = []model.Button{q, {Type: model.ButtonURL, Text: "Go", Value: "https://acme.com"}, q}
		}, nil, false, "buttons"},
		{"quick replies grouped after link", func(d *model.Draft) {
			q := model.Button{Type: model.ButtonQuickReply, Text: "Yes"}
			d.Components.Buttons = []model.Button{{Type: model.ButtonURL, Text: "Go", Value: "https://acme.com"}, q, q}
		}, nil, true, ""},
		{"button text 25", func(d *model.Draft) { d.Components.Buttons[0].Text = strings.Repeat("a", 25) }, nil, true, ""},
		{"button text 26", func(d *model.Draft) { d.Components.Buttons[0].Text = strings.Repeat("a", 26) }, nil, false, "buttons[0].text"},
		{"button text empty", func(d *model.Draft) { d.Components.Buttons[0].Text = "" }, nil, true, "buttons[0].text"},
		{"button type unknown", func(d *model.Draft) { d.Components.Buttons[0].Type = "copy_code" }, nil, false, "buttons[0].type"},
		{"url http", func(d *model.Draft) { d.Components.Buttons[0].Value = "http://acme.com" }, nil, false, "buttons[0].value"},
		{"url no host", func(d *model.Draft) { d.Components.Buttons[0].Value = "https://" }, nil, false, "buttons[0].value"},
		{"url empty on submit", func(d *model.Draft) { d.Components.Buttons[0].Value = "" }, nil, true, "buttons[0].value"},
		{"phone e164", func(d *model.Draft) {
			d.Components.Buttons = []model.Button{{Type: model.ButtonPhone, Text: "Call", Value: "+447700900412"}}
		}, nil, true, ""},
		{"phone without plus", func(d *model.Draft) {
			d.Components.Buttons = []model.Button{{Type: model.ButtonPhone, Text: "Call", Value: "447700900412"}}
		}, nil, false, "buttons[0].value"},
		{"phone with spaces", func(d *model.Draft) {
			d.Components.Buttons = []model.Button{{Type: model.ButtonPhone, Text: "Call", Value: "+44 7700 900412"}}
		}, nil, false, "buttons[0].value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := validDraft()
			tt.edit(&d)
			normalize(&d)
			got := fields(t, validate(d, tt.sample, tt.complete))
			switch {
			case tt.want == "" && got != nil:
				t.Errorf("want valid, got problems at %v", got)
			case tt.want != "" && !slices.Contains(got, tt.want):
				t.Errorf("want a problem at %q, got %v", tt.want, got)
			}
		})
	}
}

func TestValidateAuthentication(t *testing.T) {
	oneTap := model.Auth{OTPType: model.OTPOneTap, PackageName: "com.acme.app", SignatureHash: "K8a/AINcGX7"}
	tests := []struct {
		name     string
		auth     model.Auth
		complete bool
		want     string // field with a problem; "" means valid
	}{
		{"defaults", model.Auth{}, true, ""},
		{"expiry 90", model.Auth{CodeExpiryMinutes: 90}, true, ""},
		{"expiry 91", model.Auth{CodeExpiryMinutes: 91}, false, "auth.code_expiry_minutes"},
		{"expiry negative", model.Auth{CodeExpiryMinutes: -1}, false, "auth.code_expiry_minutes"},
		{"otp type unknown", model.Auth{OTPType: "zero"}, false, "auth.otp_type"},
		{"button text 26", model.Auth{ButtonText: strings.Repeat("a", 26)}, false, "auth.button_text"},
		{"one tap complete", oneTap, true, ""},
		{"one tap without app in draft", model.Auth{OTPType: model.OTPOneTap}, false, ""},
		{"one tap without package on submit", model.Auth{OTPType: model.OTPOneTap, SignatureHash: "K8a/AINcGX7"}, true, "auth.package_name"},
		{"one tap without hash on submit", model.Auth{OTPType: model.OTPOneTap, PackageName: "com.acme.app"}, true, "auth.signature_hash"},
		{"package one segment", model.Auth{OTPType: model.OTPOneTap, PackageName: "acme"}, false, "auth.package_name"},
		{"hash too short", model.Auth{OTPType: model.OTPOneTap, SignatureHash: "K8a/AIN"}, false, "auth.signature_hash"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := validDraft()
			d.Category = model.CategoryAuthentication
			d.Components.Auth = tt.auth
			normalize(&d)
			got := fields(t, validate(d, nil, tt.complete))
			switch {
			case tt.want == "" && got != nil:
				t.Errorf("want valid, got problems at %v", got)
			case tt.want != "" && !slices.Contains(got, tt.want):
				t.Errorf("want a problem at %q, got %v", tt.want, got)
			}
		})
	}
}

func TestNormalizeAuthentication(t *testing.T) {
	d := validDraft()
	d.Category = model.CategoryAuthentication
	d.Components.Auth = model.Auth{SecurityRecommendation: true, CodeExpiryMinutes: 10, AutofillText: "Fill", PackageName: "com.a.b"}
	normalize(&d)
	c := d.Components
	if c.Header.Type != model.HeaderNone || len(c.Buttons) != 0 || len(c.Body.Examples) != 0 {
		t.Fatalf("custom content kept: %+v", c)
	}
	if c.Body.Text != "*{{1}}* is your verification code. For your security, do not share this code." {
		t.Errorf("body %q", c.Body.Text)
	}
	if c.Footer.Text != "This code expires in 10 minutes." {
		t.Errorf("footer %q", c.Footer.Text)
	}
	if c.Auth.OTPType != model.OTPCopyCode || c.Auth.ButtonText != "Copy code" || c.Auth.AutofillText != "" || c.Auth.PackageName != "" {
		t.Errorf("auth %+v", c.Auth)
	}

	// Other categories drop the auth options.
	d = validDraft()
	d.Components.Auth = model.Auth{OTPType: model.OTPOneTap}
	normalize(&d)
	if d.Components.Auth != (model.Auth{}) {
		t.Errorf("auth kept on a utility template: %+v", d.Components.Auth)
	}
}

func quickReplies(n int) []model.Button {
	out := make([]model.Button, n)
	for i := range out {
		out[i] = model.Button{Type: model.ButtonQuickReply, Text: "Option"}
	}
	return out
}

func TestWarningsFlagShortLinks(t *testing.T) {
	tests := []struct {
		name  string
		c     model.Components
		field string // "" means no warning
	}{
		{"full link", model.Components{Body: model.Body{Text: "See https://acme.com/sale now"}}, ""},
		{"bitly in body", model.Components{Body: model.Body{Text: "See https://bit.ly/x1 now"}}, "body.text"},
		{"bare tinyurl in body", model.Components{Body: model.Body{Text: "Go to tinyurl.com/abc."}}, "body.text"},
		{"lookalike host", model.Components{Body: model.Body{Text: "Visit https://notbit.ly/x"}}, ""},
		{"t.co button", model.Components{Buttons: []model.Button{
			{Type: model.ButtonQuickReply, Text: "Hi"}, {Type: model.ButtonURL, Text: "Go", Value: "https://t.co/abc"},
		}}, "buttons[1].value"},
		{"goo.gl with www", model.Components{Buttons: []model.Button{
			{Type: model.ButtonURL, Text: "Go", Value: "https://www.goo.gl/abc"},
		}}, "buttons[0].value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := warnings(tt.c)
			if tt.field == "" {
				if len(w) != 0 {
					t.Errorf("unexpected warnings %+v", w)
				}
				return
			}
			if len(w) != 1 || w[0].Field != tt.field || w[0].Code != "short_link" {
				t.Errorf("warnings = %+v, want one at %s", w, tt.field)
			}
		})
	}
}

func TestShortLinksAreNotErrors(t *testing.T) {
	d := validDraft()
	d.Components.Buttons[0].Value = "https://bit.ly/x"
	normalize(&d)
	if err := validate(d, nil, true); err != nil {
		t.Fatalf("short link blocked the template: %v", err)
	}
}
