package service

import (
	"strings"
	"time"
	"unicode"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
)

const (
	// agentQuietPeriod keeps automations out of a chat a person just answered.
	agentQuietPeriod = 30 * time.Minute
	// replyCap stops a loop with a customer's own auto-responder.
	replyCap = 3
)

// reply is one automation that could answer a message.
type reply struct {
	kind   model.Kind
	text   string
	delay  time.Duration
	dedupe string
}

// isStop reports a message that is only an opt-out word.
func isStop(text string) bool {
	switch strings.ToUpper(strings.TrimRight(strings.TrimSpace(text), ".!")) {
	case "STOP", "UNSUBSCRIBE":
		return true
	}
	return false
}

// words lowercases text and pads single spaces around its words, so a term
// matches whole words only, in any script.
func words(text string) string {
	f := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r)
	})
	return " " + strings.Join(f, " ") + " "
}

// matchRule returns the first rule that matches text.
func matchRule(rules []model.KeywordRule, text string) (model.KeywordRule, bool) {
	msg := words(text)
	for _, r := range rules {
		terms := []string{r.Keywords}
		if r.Match == model.MatchAny {
			terms = strings.Split(r.Keywords, ",")
		}
		for _, t := range terms {
			if w := words(t); w != "  " && strings.Contains(msg, w) {
				return r, true
			}
		}
	}
	return model.KeywordRule{}, false
}

// isOpen reports whether now falls in working hours. key names the closed
// period otherwise, so one period gets one Away reply.
func isOpen(s *model.AwaySettings, now time.Time) (open bool, key string) {
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)
	clock := local.Format("15:04")
	for offset := range 8 {
		day := local.AddDate(0, 0, offset)
		d := dayHours(s, day.Weekday())
		switch {
		case !d.Open:
			continue
		case offset == 0 && clock >= d.To:
			continue
		case offset == 0 && clock >= d.From:
			return true, ""
		}
		return false, day.Format(time.DateOnly)
	}
	return false, "closed"
}

func dayHours(s *model.AwaySettings, w time.Weekday) model.WorkingDay {
	name := strings.ToLower(w.String()[:3])
	for _, d := range s.Hours {
		if d.Day == name {
			return d
		}
	}
	return model.WorkingDay{}
}

// candidates are the replies that could answer in, best first: a direct answer,
// then closed hours, then a greeting.
func candidates(enabled map[model.Kind]model.Settings, in model.Inbound, now time.Time) []reply {
	var out []reply
	conv := in.ConversationID.String()
	if s, ok := enabled[model.KindKeyword].(*model.KeywordSettings); ok {
		if r, ok := matchRule(s.Rules, in.Text); ok {
			out = append(out, reply{kind: model.KindKeyword, text: r.Reply, dedupe: "keyword:" + in.MessageID.String()})
		}
	}
	if s, ok := enabled[model.KindAway].(*model.AwaySettings); ok {
		if open, period := isOpen(s, now); !open {
			out = append(out, reply{kind: model.KindAway, text: s.Message, dedupe: "away:" + conv + ":" + period})
		}
	}
	if s, ok := enabled[model.KindWelcome].(*model.WelcomeSettings); ok {
		delay := time.Duration(s.DelayMinutes) * time.Minute
		switch {
		case in.NewConversation:
			out = append(out, reply{kind: model.KindWelcome, text: s.Message, delay: delay, dedupe: "welcome:" + conv})
		case in.Returning && !s.FirstTimeOnly:
			out = append(out, reply{kind: model.KindWelcome, text: s.Message, delay: delay, dedupe: "welcome:" + conv + ":" + in.MessageID.String()})
		}
	}
	return out
}

// personalize fills {{name}}.
func personalize(text, contactName string) string {
	first := "there"
	if f := strings.Fields(contactName); len(f) > 0 {
		first = f[0]
	}
	return strings.ReplaceAll(text, "{{name}}", first)
}
