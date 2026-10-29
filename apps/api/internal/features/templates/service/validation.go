package service

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// Limits from Meta's template rules (AD-7).
const (
	maxNameLen       = 512
	maxHeaderTextLen = 60
	maxBodyLen       = 1024
	maxFooterLen     = 60
	maxButtons       = 10
	maxURLButtons    = 2
	maxPhoneButtons  = 1
	maxButtonTextLen = 25
)

var (
	nameRe     = regexp.MustCompile(`^[a-z0-9_]{1,512}$`)
	variableRe = regexp.MustCompile(`\{\{([^{}]*)\}\}`)
	indexRe    = regexp.MustCompile(`^[1-9][0-9]*$`)
	e164Re     = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)
)

// shortLinkHosts are link shorteners Meta tends to reject templates for.
var shortLinkHosts = []string{"bit.ly", "tinyurl.com", "t.co", "goo.gl"}

// normalize trims the draft and fills defaults, so every stored template has the same shape.
func normalize(d *model.Draft) {
	d.Name = strings.TrimSpace(d.Name)
	d.Language = strings.TrimSpace(d.Language)
	d.Category = strings.TrimSpace(d.Category)
	normalizeComponents(&d.Components)
}

func normalizeComponents(c *model.Components) {
	h := &c.Header
	h.Type = strings.TrimSpace(h.Type)
	if h.Type == "" {
		h.Type = model.HeaderNone
	}
	if h.Type == model.HeaderText {
		h.Text, h.Example = strings.TrimSpace(h.Text), strings.TrimSpace(h.Example)
	} else {
		h.Text, h.Example = "", ""
	}
	c.Body.Text = strings.TrimSpace(c.Body.Text)
	if c.Body.Examples == nil {
		c.Body.Examples = []string{}
	}
	for i, e := range c.Body.Examples {
		c.Body.Examples[i] = strings.TrimSpace(e)
	}
	c.Footer.Text = strings.TrimSpace(c.Footer.Text)
	if c.Buttons == nil {
		c.Buttons = []model.Button{}
	}
	for i := range c.Buttons {
		b := &c.Buttons[i]
		b.Type, b.Text, b.Value = strings.TrimSpace(b.Type), strings.TrimSpace(b.Text), strings.TrimSpace(b.Value)
		if b.Type == model.ButtonQuickReply {
			b.Value = ""
		}
	}
}

// problems collects per-field validation details.
type problems []apperr.Detail

func (p *problems) add(field, msg string) { *p = append(*p, apperr.Detail{Field: field, Message: msg}) }

// validate checks a normalized draft against AD-7. A draft may be incomplete;
// complete adds the rules only a submit needs: a body, an example for every
// variable, button values and an uploaded media sample. It returns nil when
// the draft is valid, otherwise ErrInvalidTemplate with the details.
func validate(d model.Draft, sample *model.Sample, complete bool) error {
	var p problems
	if !nameRe.MatchString(d.Name) {
		p.add("name", fmt.Sprintf("Use 1 to %d lowercase letters, numbers and underscores", maxNameLen))
	}
	if !slices.Contains(model.Languages, d.Language) {
		p.add("language", "Choose one of the supported languages")
	}
	switch d.Category {
	case model.CategoryMarketing, model.CategoryUtility:
	case model.CategoryAuthentication:
		p.add("category", "Authentication templates are coming soon")
	default:
		p.add("category", "Choose marketing or utility")
	}
	c := d.Components
	validateHeader(&p, c.Header, sample, complete)
	validateBody(&p, c.Body, complete)
	if utf8.RuneCountInString(c.Footer.Text) > maxFooterLen {
		p.add("footer.text", fmt.Sprintf("Keep the footer to %d characters", maxFooterLen))
	}
	if strings.Contains(c.Footer.Text, "{{") {
		p.add("footer.text", "The footer can't have variables")
	}
	validateButtons(&p, c.Buttons, complete)
	if len(p) > 0 {
		return model.ErrInvalidTemplate.WithDetails(p...)
	}
	return nil
}

func validateHeader(p *problems, h model.Header, sample *model.Sample, complete bool) {
	switch h.Type {
	case model.HeaderNone:
	case model.HeaderText:
		if utf8.RuneCountInString(h.Text) > maxHeaderTextLen {
			p.add("header.text", fmt.Sprintf("Keep the header to %d characters", maxHeaderTextLen))
		}
		vars := variableRe.FindAllStringSubmatch(h.Text, -1)
		switch {
		case len(vars) > 1:
			p.add("header.text", "The header can have at most one variable")
		case len(vars) == 1 && vars[0][1] != "1":
			p.add("header.text", "The header's variable must be {{1}}")
		}
		if complete && h.Text == "" {
			p.add("header.text", "Enter the header text")
		}
		if complete && len(vars) == 1 && h.Example == "" {
			p.add("header.example", "Give an example for the header's variable")
		}
	case model.HeaderImage, model.HeaderVideo, model.HeaderDocument:
		if complete && (sample == nil || sampleKind(sample.ContentType) != h.Type) {
			p.add("header", "Upload a sample "+h.Type+" for the header")
		}
	default:
		p.add("header.type", "Choose none, text, image, video or document")
	}
}

// bodyVariables returns how many variables the body has (N), or an error
// message when they aren't numbered {{1}}..{{N}}.
func bodyVariables(text string) (int, string) {
	seen := map[int]bool{}
	for _, m := range variableRe.FindAllStringSubmatch(text, -1) {
		if !indexRe.MatchString(m[1]) {
			return 0, "Variables look like {{1}}, {{2}} and so on"
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, "Variables look like {{1}}, {{2}} and so on"
		}
		seen[n] = true
	}
	for i := 1; i <= len(seen); i++ {
		if !seen[i] {
			return 0, "Number the variables from {{1}} with no gaps"
		}
	}
	return len(seen), ""
}

func validateBody(p *problems, b model.Body, complete bool) {
	if utf8.RuneCountInString(b.Text) > maxBodyLen {
		p.add("body.text", fmt.Sprintf("Keep the body to %d characters", maxBodyLen))
	}
	if complete && b.Text == "" {
		p.add("body.text", "Enter the message text")
	}
	n, msg := bodyVariables(b.Text)
	if msg != "" {
		p.add("body.text", msg)
		return
	}
	if n > 0 {
		if locs := variableRe.FindAllStringIndex(b.Text, -1); locs[0][0] == 0 || locs[len(locs)-1][1] == len(b.Text) {
			p.add("body.text", "The body can't start or end with a variable")
		}
	}
	if !complete {
		return
	}
	if len(b.Examples) != n {
		p.add("body.examples", fmt.Sprintf("Give one example for each of the %d variables", n))
		return
	}
	for i, e := range b.Examples {
		if e == "" {
			p.add(fmt.Sprintf("body.examples[%d]", i), "Enter an example")
		}
	}
}

func validateButtons(p *problems, buttons []model.Button, complete bool) {
	if len(buttons) > maxButtons {
		p.add("buttons", fmt.Sprintf("Add at most %d buttons", maxButtons))
	}
	urls, phones, lastQuick, grouped := 0, 0, -1, true
	for i, b := range buttons {
		at := fmt.Sprintf("buttons[%d]", i)
		switch b.Type {
		case model.ButtonQuickReply:
			if lastQuick >= 0 && lastQuick != i-1 {
				grouped = false
			}
			lastQuick = i
		case model.ButtonURL:
			urls++
			if b.Value != "" && !isHTTPS(b.Value) {
				p.add(at+".value", "Use a link that starts with https://")
			}
		case model.ButtonPhone:
			phones++
			if b.Value != "" && !e164Re.MatchString(b.Value) {
				p.add(at+".value", "Use the full number with its country code, like +14155550142")
			}
		default:
			p.add(at+".type", "Choose quick reply, link or phone")
			continue
		}
		if b.Type != model.ButtonQuickReply && complete && b.Value == "" {
			p.add(at+".value", "Enter the button's link or number")
		}
		if complete && b.Text == "" {
			p.add(at+".text", "Enter the button text")
		}
		if utf8.RuneCountInString(b.Text) > maxButtonTextLen {
			p.add(at+".text", fmt.Sprintf("Keep button text to %d characters", maxButtonTextLen))
		}
	}
	if urls > maxURLButtons {
		p.add("buttons", fmt.Sprintf("Add at most %d link buttons", maxURLButtons))
	}
	if phones > maxPhoneButtons {
		p.add("buttons", "Add at most one phone button")
	}
	if !grouped {
		p.add("buttons", "Keep the quick replies together")
	}
}

func isHTTPS(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Host != "" && !strings.ContainsAny(s, " \t\n")
}

// warnings are problems that don't block saving or submitting but that Meta may reject for.
func warnings(c model.Components) []model.Warning {
	out := []model.Warning{}
	if host := shortLinkIn(strings.Fields(c.Body.Text)); host != "" {
		out = append(out, shortLinkWarning("body.text", host))
	}
	for i, b := range c.Buttons {
		if b.Type != model.ButtonURL {
			continue
		}
		if host := shortLinkIn([]string{b.Value}); host != "" {
			out = append(out, shortLinkWarning(fmt.Sprintf("buttons[%d].value", i), host))
		}
	}
	return out
}

func shortLinkWarning(field, host string) model.Warning {
	return model.Warning{
		Code: "short_link", Field: field,
		Message: "Meta often rejects templates with " + host + " links. Use the full link instead.",
	}
}

// shortLinkIn returns the first link shortener host among the words, or "".
func shortLinkIn(words []string) string {
	for _, w := range words {
		w = strings.ToLower(strings.Trim(w, `.,;:!?()[]"'<>`))
		w = strings.TrimPrefix(strings.TrimPrefix(w, "https://"), "http://")
		host, _, _ := strings.Cut(w, "/")
		host = strings.TrimPrefix(host, "www.")
		if slices.Contains(shortLinkHosts, host) {
			return host
		}
	}
	return ""
}
