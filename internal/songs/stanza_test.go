package songs

import (
	"strings"
	"testing"
)

func titles(list []string) string {
	var t []string
	for _, s := range list {
		t = append(t, strings.SplitN(s, "\n", 2)[0])
	}
	return strings.Join(t, ",")
}

func TestStanzaListExamples(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Verse1\na\n\nRefrain\nr\n\nVerse2\nb\n\nVerse3\nc", "Verse1,Refrain,Verse2,Refrain,Verse3,Refrain"},
		{"Verse1\na\n\nRefrain\nr\n\nVerse2\nb\n\nInsert\ni\n\nVerse3\nc", "Verse1,Refrain,Verse2,Refrain,Insert,Verse3,Refrain"},
		{"Verse1\na\n\nRefrain1\nr\n\nVerse2\nb\n\nVerse3\nc\n\nRefrain2\nq", "Verse1,Refrain1,Verse2,Refrain1,Verse3,Refrain2"},
		{"Verse1a\na\n\n&Verse1b\nb\n\nRefrain\nr\n\nVerse2a\nc\n\n&Verse2b\nd", "Verse1a,Verse1b,Refrain,Verse2a,Verse2b,Refrain"},
		{"Verse1a\na\n\n&Verse1b\nb\n\nRefrain-a\nr\n\n&Refrain-b\ns\n\nVerse2a\nc\n\n&Verse2b\nd", "Verse1a,Verse1b,Refrain-a,Refrain-b,Verse2a,Verse2b,Refrain-a,Refrain-b"},
		{"Куплет 1\nа\n\nПрипев\nп\n\nКуплет 2\nб", "Куплет 1,Припев,Куплет 2,Припев"},
	}
	for _, c := range cases {
		got := titles(StanzaList(c.in))
		if got != c.want {
			t.Errorf("StanzaList(%q)\n got  %s\n want %s", c.in, got, c.want)
		}
	}
}

func TestCleanSongLines(t *testing.T) {
	got := CleanSongLines("Verse 1@%line a@%line b@$Refrain@%r1")
	want := "Verse 1\nline a\nline b\n\nRefrain\nr1"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestSplitStanza(t *testing.T) {
	s := SplitStanza("Verse 1\nhello\nworld", true)
	if s.Title != "Verse 1" || s.Text != "hello\nworld" || !s.IsLast {
		t.Fatalf("%+v", s)
	}
}
