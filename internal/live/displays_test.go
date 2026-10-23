package live

import (
	"testing"

	"wordprojector/internal/settings"
)

type fakeScreens struct{ n int }

func (f fakeScreens) ScreenCount() int           { return f.n }
func (f fakeScreens) ScreenSize(int) (int, int) { return 1920, 1080 }

func engineWith(n int, g settings.General) *Engine {
	e := &Engine{Screens: fakeScreens{n}}
	e.Settings.General = g
	return e
}

func screensOf(d []DisplayState) []int {
	var out []int
	for _, x := range d {
		out = append(out, x.Screen)
	}
	return out
}

func TestActiveDisplaysWhenScreensDisappear(t *testing.T) {
	g := settings.General{DisplayScreen: 3, DisplayScreen2: 4, DisplayScreen3: 1, DisplayScreen4: -1, DisplayScreensExtra: []int{2, 7}}

	// everything plugged in: display 1..5 on their screens
	got := screensOf(engineWith(8, g).activeDisplays())
	want := []int{3, 4, 1, 2, 7}
	if len(got) != len(want) {
		t.Fatalf("all connected: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("all connected: %v want %v", got, want)
		}
	}

	// only screens 0..2 left: display 1 has no screen (-1, never the main screen), 2 is gone, 3 and 5 stay
	got = screensOf(engineWith(3, g).activeDisplays())
	if len(got) < 1 || got[0] != -1 {
		t.Fatalf("display 1 must have no screen when its screen is gone: %v", got)
	}
	for _, s := range got[1:] {
		if s < 0 || s >= 3 {
			t.Fatalf("a missing screen was kept: %v", got)
		}
	}

	// a single screen left: the one display shows on it
	got = screensOf(engineWith(1, g).activeDisplays())
	if len(got) != 1 || got[0] != 0 {
		t.Fatalf("single screen: %v", got)
	}
}
