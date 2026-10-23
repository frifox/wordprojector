// Package translit turns Latin typing into regular expressions that match the
// same sounds written in Cyrillic ("moi" matches "мой" and "мои"), so Cyrillic
// texts can be searched from a Latin keyboard.
package translit

import (
	"regexp"
	"strings"
	"unicode"
)

// units are tried longest first so that digraphs win over single letters.
var units = []struct{ lat, cyr string }{
	{"shch", "щ"}, {"sch", "щ"},
	{"zh", "ж"}, {"kh", "х"}, {"ch", "ч"}, {"sh", "ш"}, {"ts", "ц"},
	{"yu", "ю"}, {"ju", "ю"}, {"ya", "я"}, {"ja", "я"}, {"yo", "ёо"}, {"jo", "ёо"}, {"ye", "еєэ"}, {"je", "еєэ"},
	{"a", "а"}, {"b", "б"}, {"c", "цкс"}, {"d", "д"}, {"e", "еёэє"}, {"f", "ф"}, {"g", "гґ"}, {"h", "хг"},
	{"i", "иіїйы"}, {"j", "йж"}, {"k", "к"}, {"l", "л"}, {"m", "м"}, {"n", "н"}, {"o", "о"}, {"p", "п"},
	{"q", "к"}, {"r", "р"}, {"s", "с"}, {"t", "т"}, {"u", "у"}, {"v", "в"}, {"w", "в"}, {"x", "х"},
	{"y", "ыйиі"}, {"z", "з"},
}

// HasCyrillic reports whether s contains any Cyrillic letter.
func HasCyrillic(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Cyrillic, r) {
			return true
		}
	}
	return false
}

// IsLatinQuery reports whether s has Latin letters and no Cyrillic ones.
func IsLatinQuery(s string) bool {
	latin := false
	for _, r := range s {
		if unicode.Is(unicode.Cyrillic, r) {
			return false
		}
		if unicode.Is(unicode.Latin, r) {
			latin = true
		}
	}
	return latin
}

// Pattern returns a regexp fragment (RE2 and JavaScript compatible, to be used
// case-insensitively) matching the Cyrillic spellings of one Latin word. Doubled
// letters and soft/hard signs are optional so "ioan" still finds "Иоанна".
func Pattern(word string) string {
	w := strings.ToLower(word)
	var b strings.Builder
	for i := 0; i < len(w); {
		matched := false
		for _, u := range units {
			if strings.HasPrefix(w[i:], u.lat) {
				cyr := []rune(u.cyr)
				if len(cyr) == 1 {
					b.WriteString(string(cyr[0]) + "+")
				} else {
					b.WriteString("[" + u.cyr + "]+")
				}
				b.WriteString("[ьъ]?")
				i += len(u.lat)
				matched = true
				break
			}
		}
		if !matched {
			r := []rune(w[i:])[0]
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				b.WriteString(regexp.QuoteMeta(string(r)))
			}
			i += len(string(r))
		}
	}
	return b.String()
}
