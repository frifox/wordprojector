package translit

import (
	"regexp"
	"testing"
)

func TestPattern(t *testing.T) {
	cases := []struct {
		q, text string
		want    bool
	}{
		{"moi", "мой", true}, {"moi", "мои", true}, {"moy", "Мой друг", true},
		{"ioan", "Иоанна", true}, {"bitie", "Бытие", true}, {"gospod", "Господь", true},
		{"bog", "Бог", true}, {"lyubov", "любовь", true}, {"shchit", "щит", true},
		{"zhizn", "жизнь", true}, {"xyz", "мой", false}, {"mir", "мой", false},
	}
	for _, c := range cases {
		re := regexp.MustCompile("(?i)" + Pattern(c.q))
		if got := re.MatchString(c.text); got != c.want {
			t.Errorf("%q vs %q = %v, want %v (%s)", c.q, c.text, got, c.want, Pattern(c.q))
		}
	}
}
