package live

import (
	"testing"

	"wordprojector/internal/settings"
)

// Esc pauses what is displayed; the second Esc turns the presentation off and ends the pause, so the preview shows what
// was displayed before the first press.
func TestEscapeTwiceEndsThePause(t *testing.T) {
	e := engineWith(2, settings.General{DisplayScreen: 0, DisplayScreen2: 1, DisplayScreen3: -1, DisplayScreen4: -1})
	e.state.Type = "song"
	e.state.Items = []Item{{Text: "one"}, {Text: "two"}}
	e.state.DisplayOn, e.state.Showing = true, true

	e.Escape()
	if !e.state.Cleared || !e.state.DisplayOn {
		t.Fatalf("first Esc: cleared %v, display on %v (want paused and still on)", e.state.Cleared, e.state.DisplayOn)
	}
	e.Escape()
	if e.state.DisplayOn || e.state.Showing {
		t.Fatalf("second Esc: the presentation should be off (on %v, showing %v)", e.state.DisplayOn, e.state.Showing)
	}
	if e.state.Cleared {
		t.Fatal("second Esc: the pause should have ended, so the preview shows the slide again")
	}
	if e.state.Type != "song" || len(e.state.Items) != 2 {
		t.Fatal("the list of live items stays")
	}
	// a third Esc changes nothing
	e.Escape()
	if e.state.Cleared || e.state.DisplayOn {
		t.Fatal("third Esc should do nothing")
	}
}

// The On button works with nothing prepared: the displays show a blank slide, and Esc turns them off again.
func TestDisplaysOnWithNothingPrepared(t *testing.T) {
	for _, screens := range []int{1, 2} {
		e := engineWith(screens, settings.General{DisplayScreen: 0, DisplayScreen2: 1, DisplayScreen3: -1, DisplayScreen4: -1})
		e.SetDisplayOn(true)
		if !e.state.Showing || e.state.DisplayOn != (screens > 1) {
			t.Fatalf("%d screens: on with nothing prepared: showing %v, display on %v", screens, e.state.Showing, e.state.DisplayOn)
		}
		for _, d := range e.state.Displays {
			if d.Slide.Kind != "passive" {
				t.Fatalf("%d screens: the blank slide should be the passive one, got %q", screens, d.Slide.Kind)
			}
		}
		e.Escape()
		if e.state.Showing || e.state.DisplayOn {
			t.Fatalf("%d screens: Esc should turn the blank displays off", screens)
		}
	}
}
