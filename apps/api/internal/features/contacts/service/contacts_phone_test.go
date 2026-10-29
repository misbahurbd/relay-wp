package service

import "testing"

func TestNormalizePhone(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"+44 7700 900412", "+447700900412", true},
		{"447700900412", "+447700900412", true},
		{"0044 7700 900412", "+447700900412", true},
		{"+44 (0)7700-900412", "+447700900412", true},
		{" +1 (415) 555-2671 ", "+14155552671", true},
		{"+880 1712-345678", "+8801712345678", true},
		{"07700 900412", "", false}, // national format: no country code
		{"", "", false},
		{"+44 12", "", false},
		{"not a number", "", false},
		{"+999 1234567", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizePhone(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizePhone(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestSpellingsOfOneNumberAreOneContact(t *testing.T) {
	a, _ := NormalizePhone("+44 7700 900412")
	b, _ := NormalizePhone("447700900412")
	if a != b {
		t.Fatalf("%q and %q should be the same contact", a, b)
	}
	if got := DisplayPhone(a); got != "+44 7700 900412" {
		t.Errorf("DisplayPhone = %q", got)
	}
}

func TestPhoneDigits(t *testing.T) {
	if d, ok := phoneDigits("+44 7700"); !ok || d != "447700" {
		t.Errorf("phoneDigits(+44 7700) = %q, %v", d, ok)
	}
	if _, ok := phoneDigits("Flat 42"); ok {
		t.Error("a name with a number is not a phone search")
	}
	if _, ok := phoneDigits("12"); ok {
		t.Error("two digits are too few to search phones")
	}
}
