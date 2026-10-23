// Package desktop abstracts the native shell (windows, screens, dialogs) so
// the server can run headless (browser mode) or inside the Wails desktop app.
package desktop

// Screen is one physical display.
type Screen struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	IsPrimary bool   `json:"isPrimary"`
}

// Filter is a file dialog filter.
type Filter struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
}

// Desktop is implemented by the Wails shell.
type Desktop interface {
	Available() bool
	Screens() []Screen
	OpenFiles(title string, filters []Filter, multiple bool) ([]string, error)
	SaveFile(title, defaultName string, filters []Filter) (string, error)
	OpenFolder(title string) (string, error)
	Message(title, message string)
	// ShowProjector shows the display window disp on screen index, or hides it.
	ShowProjector(disp int, screen int, show bool, onTop bool)
	Quit()
}

// Noop is the headless implementation.
type Noop struct{}

func (Noop) Available() bool   { return false }
func (Noop) Screens() []Screen { return CachedScreens() }
func (Noop) OpenFiles(string, []Filter, bool) ([]string, error) {
	return nil, ErrNoDesktop
}
func (Noop) SaveFile(string, string, []Filter) (string, error) { return "", ErrNoDesktop }
func (Noop) OpenFolder(string) (string, error)                 { return "", ErrNoDesktop }
func (Noop) Message(string, string)                            {}
func (Noop) ShowProjector(int, int, bool, bool)                {}
func (Noop) Quit()                                             {}

// ErrNoDesktop is returned when native dialogs are unavailable.
var ErrNoDesktop = errNoDesktop{}

type errNoDesktop struct{}

func (errNoDesktop) Error() string { return "native dialogs are not available" }
