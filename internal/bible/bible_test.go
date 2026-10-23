package bible

import "testing"

func TestVerseID(t *testing.T) {
	if VerseID(1, 1, 1) != "B001C001V001" {
		t.Fatal(VerseID(1, 1, 1))
	}
	b, c, v := decodeVerseID("B066C022V021")
	if b != 66 || c != 22 || v != 21 {
		t.Fatal(b, c, v)
	}
}
