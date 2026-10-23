package songs

import "testing"

func TestUnicodeWholeWord(t *testing.T) {
	re, _, _, err := BuildSearchRegex("свет", SearchContainsWordPhrase, "\n")
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString("И сказал Бог: да будет свет. И стал") || re.MatchString("светильник") || !re.MatchString("Свет") {
		t.Fatal("whole word cyrillic")
	}
	if !ContainsAllWords("Бог создал свет", []string{"свет", "бог"}) || ContainsAllWords("светильник", []string{"свет"}) {
		t.Fatal("contains all")
	}
	re2, _, _, _ := BuildSearchRegex("light", SearchContainsPhrase, "\n")
	if !re2.MatchString("Lighten") {
		t.Fatal("loose match")
	}
}

func TestWordMatcher(t *testing.T) {
	cases := []struct {
		q            string
		exact, full  bool
		text         string
		want         bool
	}{
		{"бог свет", false, false, "И сказал Бог: да будет свет", true},
		{"свет бог", false, false, "И сказал Бог: да будет свет", true},   // any order
		{"свет бог", true, false, "И сказал Бог: да будет свет", false},   // exact needs order+adjacent
		{"бог да", true, false, "И сказал Бог: да будет", true},
		{"свет", false, false, "светильник", true},                        // partial allowed
		{"свет", false, true, "светильник", false},                        // full word only
		{"СВЕТ", false, true, "и свет.", true},
		{"in the", true, true, "In the beginning", true},
		{"the begin", true, true, "In the beginning", false},
		{"the begin", true, false, "In the beginning", true},
	}
	for _, c := range cases {
		m, err := NewWordMatcher(c.q, c.exact, c.full)
		if err != nil {
			t.Fatal(err)
		}
		if got := m.Match(c.text); got != c.want {
			t.Errorf("%q exact=%v full=%v on %q: got %v want %v", c.q, c.exact, c.full, c.text, got, c.want)
		}
	}
}
