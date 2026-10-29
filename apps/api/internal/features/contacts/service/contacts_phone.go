package service

import (
	"strings"

	"github.com/nyaruka/phonenumbers"
)

// NormalizePhone returns raw as E.164 (AD-2). The number must carry its
// country code: "+44 7700 900412", "447700900412" and "0044 7700 900412" all
// give "+447700900412". There is no default region, so a national number such
// as "07700 900412" is invalid, and so is one too short or long for its country.
func NormalizePhone(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" || len(s) > 40 {
		return "", false
	}
	switch {
	case strings.HasPrefix(s, "+"):
	case strings.HasPrefix(s, "00"):
		s = "+" + s[2:]
	case s[0] >= '1' && s[0] <= '9':
		s = "+" + s
	case s[0] == '(' && len(s) > 1 && s[1] >= '1' && s[1] <= '9':
		s = "+" + s
	default:
		return "", false
	}
	// The check is on length for the country, not on its allocated ranges:
	// range data lags new allocations, and refusing a real customer is worse
	// than keeping a number that never answers.
	num, err := phonenumbers.Parse(s, "")
	if err != nil || phonenumbers.IsPossibleNumberWithReason(num) != phonenumbers.IS_POSSIBLE ||
		phonenumbers.GetRegionCodeForCountryCode(int(num.GetCountryCode())) == phonenumbers.UNKNOWN_REGION {
		return "", false
	}
	return phonenumbers.Format(num, phonenumbers.E164), true
}

// DisplayPhone formats an E.164 number for people, for example "+44 7700 900412".
func DisplayPhone(e164 string) string {
	num, err := phonenumbers.Parse(e164, "")
	if err != nil {
		return e164
	}
	return phonenumbers.Format(num, phonenumbers.INTERNATIONAL)
}
