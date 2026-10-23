package announce

import "testing"

func TestSlideList(t *testing.T) {
	l := SlideList("Announce\n - a\n\nSlide\n - b\nmore")
	if len(l) != 2 || SlideText("Announce\n - a\n\nSlide\n - b\nmore", 1) != "- b\nmore" {
		t.Fatalf("%q", l)
	}
	if len(SlideList("just text\nno titles")) != 1 {
		t.Fatal("one slide expected")
	}
}
