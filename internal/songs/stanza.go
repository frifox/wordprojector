// Package songs holds the song model, the song text (stanza) parser ported
// from SoftProjector and the database repository for songbooks and songs.
package songs

import (
	"regexp"
	"strings"
)

var (
	verseTitles   = []string{"Verse", "Куплет", "Strophe", "Verš"}
	refrainTitles = []string{"Chorus", "Sbor", "Refrain", "Припев", "Приспів", "Refrén"}
	slideTitles   = []string{"Slide", "Слайд", "Dia", "Snímek", "Insert", "Вставка", "Einfügung", "Vložka",
		"Intro", "Вступление", "Einleitung", "Úvod", "Ending", "Окончание", "Закінчення", "Ende", "Závěr"}
)

func startsWithAny(s string, prefixes []string, amp bool) bool {
	for _, p := range prefixes {
		if amp {
			p = "&" + p
		}
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// IsStanzaVerseTitle reports whether the line starts a verse block.
func IsStanzaVerseTitle(s string) bool { return startsWithAny(s, verseTitles, false) }

// IsStanzaAndVerseTitle reports whether the line continues a verse block (&Verse).
func IsStanzaAndVerseTitle(s string) bool { return startsWithAny(s, verseTitles, true) }

// IsStanzaRefrainTitle reports whether the line starts a refrain block.
func IsStanzaRefrainTitle(s string) bool { return startsWithAny(s, refrainTitles, false) }

// IsStanzaAndRefrainTitle reports whether the line continues a refrain block (&Chorus).
func IsStanzaAndRefrainTitle(s string) bool { return startsWithAny(s, refrainTitles, true) }

// IsStanzaSlideTitle reports whether the line starts a slide/insert/intro/ending block.
func IsStanzaSlideTitle(s string) bool { return startsWithAny(s, slideTitles, false) }

// IsStanzaTitle reports whether the line is any stanza title.
func IsStanzaTitle(s string) bool {
	return IsStanzaVerseTitle(s) || IsStanzaAndVerseTitle(s) || IsStanzaRefrainTitle(s) ||
		IsStanzaAndRefrainTitle(s) || IsStanzaSlideTitle(s)
}

// TitlePrefixes exposes the recognised stanza title words (for editor highlighting).
func TitlePrefixes() (verse, refrain, slide []string) {
	return append([]string{}, verseTitles...), append([]string{}, refrainTitles...), append([]string{}, slideTitles...)
}

func getStanzaBlock(i *int, list []string) string {
	var block strings.Builder
	j := *i
	for *i < len(list) {
		line := list[*i]
		if strings.HasPrefix(line, "&") {
			line = strings.Replace(line, "&", "", -1)
		}
		if IsStanzaTitle(line) && *i != j {
			*i--
			break
		}
		block.WriteString(line)
		block.WriteString("\n")
		*i++
	}
	return strings.TrimSpace(block.String())
}

func removeLast(n int, list []string) []string {
	if n > len(list) {
		n = len(list)
	}
	return list[:len(list)-n]
}

// StanzaList prepares the list of slides for a song in correct singing order,
// automatically repeating the refrain after each verse. This is a faithful
// port of Song::getSongTextList() from the original program.
func StanzaList(songText string) []string {
	var formatted []string
	var chorus []string
	hasChorus := false
	hasVstavka := false
	chor := 0
	songlist := strings.Split(strings.ReplaceAll(songText, "\r\n", "\n"), "\n")
	listcount := len(songlist)
	pnum := 0
	for pnum < listcount {
		line := songlist[pnum]
		switch {
		case IsStanzaVerseTitle(line):
			text := getStanzaBlock(&pnum, songlist)
			formatted = append(formatted, text)
			if hasChorus {
				formatted = append(formatted, chorus...)
			}
			hasVstavka = false
		case IsStanzaAndVerseTitle(line):
			text := getStanzaBlock(&pnum, songlist)
			if hasChorus {
				formatted = removeLast(len(chorus), formatted)
			}
			formatted = append(formatted, text)
			if hasChorus {
				formatted = append(formatted, chorus...)
			}
			hasVstavka = false
		case IsStanzaSlideTitle(line):
			text := getStanzaBlock(&pnum, songlist)
			formatted = append(formatted, text)
			hasVstavka = true
		case IsStanzaRefrainTitle(line):
			text := getStanzaBlock(&pnum, songlist)
			chorusOld := chorus
			chorus = []string{text}
			hasChorus = true
			chor++
			if chor == 1 {
				formatted = append(formatted, chorus...)
			} else if chor == 2 && !hasVstavka {
				formatted = removeLast(len(chorusOld), formatted)
				formatted = append(formatted, chorus...)
				chor--
			} else if chor == 2 && hasVstavka {
				formatted = append(formatted, chorus...)
				chor--
			}
			hasVstavka = false
		case IsStanzaAndRefrainTitle(line):
			text := getStanzaBlock(&pnum, songlist)
			formatted = removeLast(len(chorus), formatted)
			chorus = append(chorus, text)
			formatted = append(formatted, chorus...)
		}
		pnum++
	}
	return formatted
}

// Stanza is one slide of a song ready for projection.
type Stanza struct {
	Title  string `json:"title"`
	Text   string `json:"text"`
	IsLast bool   `json:"isLast"`
}

// SplitStanza separates the stanza title line (if any) from the stanza text.
func SplitStanza(block string, isLast bool) Stanza {
	lines := strings.Split(block, "\n")
	st := Stanza{IsLast: isLast}
	if len(lines) > 0 && IsStanzaTitle(lines[0]) {
		st.Title = lines[0]
		lines = lines[1:]
	}
	st.Text = strings.TrimSpace(strings.Join(lines, "\n"))
	return st
}

var oldLineMarkers = regexp.MustCompile(`@\$|@%`)

// HasOldLineMarkers reports whether text uses the pre-2.0 "@$"/"@%" separators.
func HasOldLineMarkers(s string) bool { return oldLineMarkers.MatchString(s) }

// CleanSongLines converts the pre-2.0 single-line song encoding ("@$" between
// stanzas and "@%" between lines) into plain multi-line text.
func CleanSongLines(songText string) string {
	var out strings.Builder
	for _, stanza := range strings.Split(songText, "@$") {
		lines := strings.Split(stanza, "@%")
		out.WriteString(strings.TrimSpace(strings.Join(lines, "\n")))
		out.WriteString("\n\n")
	}
	return strings.TrimSpace(out.String())
}

// ToMultiLine converts "@%" separated text to newline separated text.
func ToMultiLine(s string) string {
	return strings.TrimSpace(strings.Join(strings.Split(s, "@%"), "\n"))
}

// ToSingleLine converts newline separated text into "@%" separated text.
func ToSingleLine(s string) string {
	return strings.TrimSpace(strings.Join(strings.Split(s, "\n"), "@%"))
}

var nonWord = regexp.MustCompile(`[^\p{L}\p{N}_]+`)

// Clean removes all non alphanumeric characters (port of clean()).
func Clean(s string) string {
	s = nonWord.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

// Categories are the fixed song categories of the original program, indexed by id.
var Categories = []string{"Other", "Bible Stories", "Gospel", "God, His love and greatness",
	"The Resurrection of Christ", "Time", "The second coming of Christ and the judgement",
	"Children and Family", "For new converts", "Spiritual struggle and victory",
	"Harvest", "Jesus Christ", "Love", "Mother", "Prayer", "Youth",
	"Wedding", "Sunset / Sunrise", "Baptism",
	"New Years", "Funeral", "At the ordination",
	"On the Lord's Supper", "Heavenly abode", "Instruction and self-test",
	"Holy Ghost", "Church", "Before church meeting",
	"Last Days", "Practical life with God", "At the end of church meeting",
	"Welcome and farewell", "The call to work", "Call to repentance",
	"Journey of faith, faith and hope", "Various Christian holidays",
	"Determination and faithfulness", "Christmas", "Following Christ",
	"The Word of God", "Salvation", "Suffering and death of Christ",
	"Consolation and encouragement", "Praise and thanksgiving", "Christian Joy"}
