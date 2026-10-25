// Package shell hosts the operator and projector windows with Wails v3.
package shell

import (
	_ "embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"wordprojector/internal/appdata"
	"wordprojector/internal/desktop"
	"wordprojector/internal/live"
	"wordprojector/internal/settings"
)

//go:embed icon.png
var iconPNG []byte

// Shell implements desktop.Desktop with Wails windows.
type Shell struct {
	app        *application.App
	name       string
	url        string
	engine     *live.Engine
	main       *application.WebviewWindow
	projectors map[int]*application.WebviewWindow
	mu         sync.Mutex
	ready      bool
	quitting   bool
	watching   sync.Once
	want       map[int]bool        // display windows that should be visible
	shown      map[int]bool // display windows that are visible
	pending    map[int]bool // display windows on their way to being shown
	visible    map[int]bool // display windows the native side has reported visible (macOS)
	revealAt   map[int]time.Time // when each window started to be shown (for the log)
	normal     application.Rect // the window's bounds when not maximised
	haveNormal bool
	answered   bool // the disconnect question was answered: the program is ending
	geoTimer   *time.Timer
	urlHash    string // passed to the operator page (see quitHelper)
	other      bool // connected to another running instance: it owns the displays and the data
	noUI       bool // no operator window: only the display windows, the operator uses a browser
}

// New creates the Wails application (the event loop starts in Run).
func New(name string, noUI bool) *Shell {
	s := &Shell{name: name, noUI: noUI, projectors: map[int]*application.WebviewWindow{}, want: map[int]bool{}, shown: map[int]bool{}, pending: map[int]bool{}, visible: map[int]bool{}, revealAt: map[int]time.Time{}}
	projectorEvents = s.projectorEvent
	policy := application.ActivationPolicyRegular
	if noUI {
		policy = application.ActivationPolicyAccessory // a background app: no Dock icon, never steals focus
	}
	s.app = application.New(application.Options{
		Name:        name,
		Description: "Church projection software",
		Icon:        iconPNG,
		Mac: application.MacOptions{
			// with an operator window, closing it ends the program; without one the display windows come and go
			ApplicationShouldTerminateAfterLastWindowClosed: !noUI,
			ActivationPolicy:                                policy,
		},
		OnShutdown: func() { s.quitting = true },
		// Cmd+Q
		ShouldQuit: func() bool { return s.confirmDisconnect() },
	})
	return s
}

// Configure wires the engine before Run.
func (s *Shell) Configure(e *live.Engine) {
	s.engine = e
	e.Screens = s
	e.OnDisplays = s.onDisplays
}

// Fatal shows a startup error.
func (s *Shell) Fatal(title, msg string) {
	fmt.Fprintln(os.Stderr, title+": "+msg)
	log.Println(title + ": " + msg)
}

// Choices of AskPortBusy.
const (
	PortConnect = iota // use the program that is already listening
	PortNext           // listen on the next free port
	PortQuit
)

// AskPortBusy tells the user the port is taken and asks what to do. existing describes the program that holds it
// when that is a Word Projector (then it can be connected to), else it is empty.
func (s *Shell) AskPortBusy(port int, existing string, canConnect bool) int {
	choice := PortQuit
	msg := fmt.Sprintf("Port %d is already in use.", port)
	d := s.app.Dialog.Question().SetTitle(s.name)
	if existing != "" {
		msg += fmt.Sprintf("\n\n%s is already running there.", existing)
		if canConnect {
			d.AddButton("Connect to Existing").OnClick(func() { choice = PortConnect })
		}
	} else {
		msg += "\n\nAnother program is using it."
	}
	d.AddButton("Use Next Available Port").OnClick(func() { choice = PortNext })
	q := d.AddButton("Quit")
	q.OnClick(func() { choice = PortQuit })
	d.SetCancelButton(q).SetMessage(msg).Show()
	return choice
}

// Run runs the event loop (blocks). start is called once the loop is up (dialogs need it): it binds the backend and
// returns the URL the windows load, and whether that is another running instance (then this one only shows a window
// onto it). An error ends the program.
func (s *Shell) Run(start func() (url string, other bool, err error)) error {
	if !s.noUI && runtime.GOOS == "darwin" {
		menu := s.app.Menu.New()
		menu.AddRole(application.AppMenu)
		menu.AddRole(application.EditMenu)
		menu.AddRole(application.WindowMenu)
		s.app.Menu.Set(menu)
	}
	s.app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go func() {
			url, other, err := start()
			if err != nil {
				if err.Error() != "quit" {
					s.Fatal("Word Projector could not start", err.Error())
					s.app.Dialog.Error().SetTitle(s.name).SetMessage(err.Error()).Show()
				}
				s.quitting = true
				s.app.Quit()
				return
			}
			s.url = url
			s.other = other
			if other {
				s.quitHelper()
			}
			if !s.noUI {
				s.createMain()
			}
			s.mu.Lock()
			s.ready = true
			s.mu.Unlock()
			if other {
				return // the other instance owns the displays
			}
			// the system's screen list is filled in just after the application starts: refresh again once it is known
			s.engine.Refresh()
			time.Sleep(700 * time.Millisecond)
			s.engine.Refresh()
			s.watchScreens()
			// the screens are known now: a window remembered on a monitor that is gone moves to the middle of the main one
			if s.main != nil {
				if x, y := s.main.Position(); !s.visibleOnAScreen(settings.WindowGeometry{X: x, Y: y}) {
					s.main.Center()
				}
			}
		}()
	})
	return s.app.Run()
}

// quitOther asks the instance this one is connected to to quit.
func (s *Shell) quitOther() {
	if !s.other || s.url == "" {
		return
	}
	c := http.Client{Timeout: 2 * time.Second}
	if resp, err := c.Post(s.url+"/api/app/quit", "application/json", nil); err == nil {
		resp.Body.Close()
	}
}

// confirmDisconnect is asked when a window connected to another running instance is about to end: disconnect only,
// or shut that backend down too. It returns false when the user cancels. Asked once per exit.
func (s *Shell) confirmDisconnect() bool {
	s.mu.Lock()
	done := s.answered
	s.mu.Unlock()
	if !s.other || done {
		return true
	}
	choice := "cancel"
	d := s.app.Dialog.Question().SetTitle(s.name).SetMessage("This window is connected to a Word Projector that is already running.\n\nWhat should happen to it?")
	d.AddButton("Disconnect Only").SetAsDefault().OnClick(func() { choice = "disconnect" })
	d.AddButton("Shut Down Backend Too").OnClick(func() { choice = "shutdown" })
	d.AddButton("Cancel").SetAsCancel().OnClick(func() { choice = "cancel" })
	d.Show() // not attached to the window: a sheet returns before it is answered

	if choice == "cancel" {
		return false
	}
	if choice == "shutdown" {
		s.quitOther()
	}
	s.mu.Lock()
	s.answered = true
	s.mu.Unlock()
	return true
}

// quitHelper lets the operator page, which is served by another instance here, quit this program: the page's
// Quit (File menu, Cmd+Q) would otherwise quit that other instance. The page finds the port in its URL hash.
func (s *Shell) quitHelper() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return
	}
	s.urlHash = "#wpquit=" + strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /quit", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
		go func() {
			if s.confirmDisconnect() {
				s.Quit()
			}
		}()
	})
	go http.Serve(ln, mux)
}

// visibleOnAScreen reports whether the top left part of a window at g (its title bar) lies on a connected screen,
// so a window remembered on a monitor that is gone is not opened out of sight.
func (s *Shell) visibleOnAScreen(g settings.WindowGeometry) bool {
	screens := s.app.Screen.GetAll()
	if len(screens) == 0 {
		return true // the system's screen list is filled in just after the application starts: trust what was saved
	}
	for _, sc := range screens {
		b := sc.Bounds
		if g.X+100 >= b.X && g.X+100 < b.X+b.Width && g.Y+20 >= b.Y && g.Y+20 < b.Y+b.Height {
			return true
		}
	}
	return false
}

func (s *Shell) createMain() {
	var geo settings.WindowGeometry
	var maximised bool
	s.engine.Locked(func() { geo, maximised = s.engine.Settings.App.Window, s.engine.Settings.Main.IsWindowMaximized })
	// maximised only after the window has its normal bounds: on Windows, bounds set to a maximised window leave it small
	var startMax bool
	opts := application.WebviewWindowOptions{
		Name:             "main",
		Title:            s.name,
		URL:              s.url + "/" + s.urlHash,
		Width:            1280,
		Height:           800,
		MinWidth:         800,
		MinHeight:        500,
		BackgroundColour: application.NewRGB(236, 236, 236),
		Mac: application.MacWindow{
			TitleBar: application.MacTitleBarDefault,
		},
		Windows: application.WindowsWindow{DisableMenu: true},
	}
	// open where it was last used: same monitor, position and size (the size from before it was maximised)
	if geo.Width >= 600 && geo.Height >= 400 && s.visibleOnAScreen(geo) {
		opts.Width, opts.Height = geo.Width, geo.Height
		opts.X, opts.Y = geo.X, geo.Y
		opts.InitialPosition = application.WindowXY
		s.normal = application.Rect{X: geo.X, Y: geo.Y, Width: geo.Width, Height: geo.Height}
		s.haveNormal = true
		startMax = maximised
	} else {
		opts.InitialPosition = application.WindowCentered
		// the very first start (no geometry was ever saved) fills the screen; the size it has when it is made smaller again
		// is a comfortable one in the middle of the main screen, not the whole screen
		if geo.Width == 0 && geo.Height == 0 {
			startMax = true
			if sc := s.app.Screen.GetPrimary(); sc != nil {
				w, h := min(1280, sc.Bounds.Width*8/10), min(820, sc.Bounds.Height*85/100)
				s.normal = application.Rect{X: sc.Bounds.X + (sc.Bounds.Width-w)/2, Y: sc.Bounds.Y + (sc.Bounds.Height-h)/2, Width: w, Height: h}
				s.haveNormal = true
			}
		}
	}
	s.main = s.app.Window.NewWithOptions(opts)
	if opts.InitialPosition == application.WindowXY {
		s.main.SetBounds(s.normal) // the saved bounds include the title bar, the size option does not
	}
	s.main.Show() // a window created after the loop started is not shown by itself
	if startMax {
		s.main.Maximise()
	}
	s.main.Focus()
	// remember the geometry as it changes (the program can end any way), not only when the window closes
	track := func(*application.WindowEvent) { s.noteGeometry() }
	for _, ev := range []events.WindowEventType{events.Common.WindowDidMove, events.Common.WindowDidResize, events.Common.WindowMaximise, events.Common.WindowUnMaximise} {
		s.main.OnWindowEvent(ev, track)
	}
	s.main.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if !s.confirmClose() || !s.confirmDisconnect() {
			e.Cancel()
			return
		}
		s.saveGeometry()
		s.quitting = true
		for _, w := range s.projectors {
			w.Close()
		}
	})
}

// noteGeometry keeps the window's normal (not maximised) bounds and saves the geometry shortly after it settles.
func (s *Shell) noteGeometry() {
	if s.main == nil || s.other || s.quitting {
		return
	}
	// the window is asked before the lock is taken: asking needs the main thread, and this runs on it (a window event)
	// while the timer's save may hold the lock and wait for that same thread
	normal := !s.main.IsMaximised() && !s.main.IsFullscreen()
	var b application.Rect
	if normal {
		b = s.main.Bounds()
	}
	s.mu.Lock()
	if normal {
		s.normal, s.haveNormal = b, true
	}
	if s.geoTimer != nil {
		s.geoTimer.Stop()
	}
	s.geoTimer = time.AfterFunc(800*time.Millisecond, s.saveGeometry)
	s.mu.Unlock()
}

// screenSignature describes the connected screens, to notice when one is plugged in or removed.
func (s *Shell) screenSignature() string {
	var b strings.Builder
	for _, sc := range s.wailsScreens() {
		fmt.Fprintf(&b, "%d,%d,%d,%d,%v;", sc.Bounds.X, sc.Bounds.Y, sc.Bounds.Width, sc.Bounds.Height, sc.IsPrimary)
	}
	return b.String()
}

// watchScreens polls for screens being plugged in or removed while the app runs. The display windows follow
// (a window whose screen vanished is hidden, one whose screen returns is shown again) and open Settings
// dialogs are told to reload their list of screens.
func (s *Shell) watchScreens() {
	s.watching.Do(func() {
		go func() {
			last := s.screenSignature()
			for range time.Tick(1500 * time.Millisecond) {
				if s.quitting {
					return
				}
				sig := s.screenSignature()
				if sig == last || sig == "" {
					continue
				}
				last = sig
				log.Printf("screens changed: %d connected", len(s.wailsScreens()))
				s.engine.Refresh()
				if s.engine.Broadcast != nil {
					s.engine.Broadcast(map[string]any{"type": "data", "what": "screens"})
				}
			}
		}()
	})
}

func (s *Shell) saveGeometry() {
	if s.main == nil || s.other { // the other instance's settings are not ours to overwrite
		return
	}
	// never ask the window while holding the lock: the main thread may be waiting for the lock (see noteGeometry)
	maximised := s.main.IsMaximised()
	fullscreen := s.main.IsFullscreen()
	current := s.main.Bounds()
	s.mu.Lock()
	b := s.normal
	if !s.haveNormal || (!maximised && !fullscreen) {
		b = current
	}
	s.mu.Unlock()
	s.engine.Locked(func() {
		s.engine.Settings.App.Window = settings.WindowGeometry{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height}
		s.engine.Settings.Main.IsWindowMaximized = maximised
		_ = settings.Save(s.engine.DB, s.engine.Settings)
	})
}

// confirmClose asks about an unsaved schedule. Returns true to allow closing.
func (s *Shell) confirmClose() bool {
	items, file, saved := s.engine.Schedule.Snapshot()
	if saved || file == "" || len(items) == 0 {
		return true
	}
	result := make(chan string, 1)
	d := s.app.Dialog.Question().SetTitle("Schedule not saved").SetMessage("Do you want to save current schedule?")
	d.AddButton("Save").SetAsDefault().OnClick(func() { result <- "save" })
	d.AddButton("Discard").OnClick(func() { result <- "discard" })
	d.AddButton("Cancel").SetAsCancel().OnClick(func() { result <- "cancel" })
	d.AttachToWindow(s.main)
	d.Show()
	var r string
	select {
	case r = <-result:
	default:
		// Some platforms show the dialog synchronously and callbacks already fired.
		select {
		case r = <-result:
		default:
			r = "cancel"
		}
	}
	switch r {
	case "save":
		path := file
		if path == "untitled.spsc" {
			p, err := s.SaveFile("Save SoftProjector schedule as:", "untitled.spsc", []desktop.Filter{{Name: "SoftProjector schedule file (*.spsc)", Pattern: "*.spsc"}})
			if err != nil || p == "" {
				return false
			}
			path = p
		}
		if err := s.engine.Schedule.Save(path, scheduleBlobs{s.engine}); err != nil {
			s.Message("Error", err.Error())
			return false
		}
		return true
	case "discard":
		return true
	}
	return false
}

type scheduleBlobs struct{ e *live.Engine }

func (b scheduleBlobs) SongBackground(id int64) []byte { bg, _ := b.e.SongBG(id); return bg }
func (b scheduleBlobs) SlideImages(id int64) (f, s, p []byte) {
	return b.e.SlideImgs(id)
}

// ---- desktop.Desktop ------------------------------------------------------------

func (s *Shell) Available() bool { return true }

func (s *Shell) wailsScreens() []*application.Screen {
	s.mu.Lock()
	ready := s.ready
	s.mu.Unlock()
	if !ready {
		return nil
	}
	list := s.app.Screen.GetAll()
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].IsPrimary != list[j].IsPrimary {
			return list[i].IsPrimary
		}
		if list[i].Bounds.X != list[j].Bounds.X {
			return list[i].Bounds.X < list[j].Bounds.X
		}
		return list[i].Bounds.Y < list[j].Bounds.Y
	})
	return list
}

func (s *Shell) Screens() []desktop.Screen {
	var out []desktop.Screen
	for i, sc := range s.wailsScreens() {
		out = append(out, desktop.Screen{Index: i, Name: sc.Name, X: sc.Bounds.X, Y: sc.Bounds.Y, Width: sc.Bounds.Width, Height: sc.Bounds.Height, IsPrimary: sc.IsPrimary})
	}
	return out
}

func (s *Shell) ScreenCount() int { return len(s.wailsScreens()) }

// ScreenSize is the pixel size of screen i.
func (s *Shell) ScreenSize(i int) (int, int) {
	list := s.wailsScreens()
	if i < 0 || i >= len(list) {
		return 0, 0
	}
	return list[i].Bounds.Width, list[i].Bounds.Height
}

// extensions turns file filters ("*.png;*.jpg") into bare extensions ("png", "jpg"); none when anything is allowed.
func extensions(filters []desktop.Filter) []string {
	var out []string
	for _, f := range filters {
		for _, p := range strings.Split(f.Pattern, ";") {
			p = strings.TrimPrefix(strings.TrimSpace(p), "*")
			p = strings.TrimPrefix(p, ".")
			if p == "" || p == "*" {
				return nil
			}
			out = append(out, p)
		}
	}
	return out
}

func (s *Shell) nativeWindow() unsafe.Pointer {
	if s.main == nil {
		return nil
	}
	return s.main.NativeWindow()
}

func (s *Shell) OpenFiles(title string, filters []desktop.Filter, multiple bool) ([]string, error) {
	if haveNativeOpen {
		paths, _ := nativeOpen(s.nativeWindow(), title, extensions(filters), multiple, false)
		return paths, nil
	}
	d := s.app.Dialog.OpenFile().SetTitle(title).CanChooseFiles(true)
	for _, f := range filters {
		d.AddFilter(f.Name, f.Pattern)
	}
	if s.main != nil {
		d.AttachToWindow(s.main)
	} else {
		bringToFront()
	}
	if multiple {
		return d.PromptForMultipleSelection()
	}
	p, err := d.PromptForSingleSelection()
	if err != nil || p == "" {
		return nil, err
	}
	return []string{p}, nil
}

func (s *Shell) OpenFolder(title string) (string, error) {
	if haveNativeOpen {
		paths, _ := nativeOpen(s.nativeWindow(), title, nil, false, true)
		if len(paths) == 0 {
			return "", nil
		}
		return paths[0], nil
	}
	d := s.app.Dialog.OpenFile().SetTitle(title).CanChooseFiles(false).CanChooseDirectories(true).CanCreateDirectories(true)
	if s.main != nil {
		d.AttachToWindow(s.main)
	} else {
		bringToFront()
	}
	return d.PromptForSingleSelection()
}

func (s *Shell) SaveFile(title, defaultName string, filters []desktop.Filter) (string, error) {
	d := s.app.Dialog.SaveFile().SetMessage(title).SetFilename(defaultName)
	// exports start in the folder of the program's own files in the user's Documents (~/Documents/WordProjector)
	if dir, err := appdata.DocumentsDir(""); err == nil {
		d.SetDirectory(dir)
	}
	for _, f := range filters {
		d.AddFilter(f.Name, f.Pattern)
	}
	if s.main != nil {
		d.AttachToWindow(s.main)
	} else {
		bringToFront()
	}
	return d.PromptForSingleSelection()
}

func (s *Shell) Message(title, message string) {
	d := s.app.Dialog.Info().SetTitle(title).SetMessage(message)
	if s.main != nil {
		d.AttachToWindow(s.main)
	}
	d.Show()
}

func (s *Shell) Quit() {
	s.quitting = true
	s.app.Quit()
}

// onDisplays shows/hides projector windows according to the live state.
func (s *Shell) onDisplays(st live.State) {
	s.mu.Lock()
	ready := s.ready
	s.mu.Unlock()
	if !ready || s.quitting {
		return
	}
	// the projection is an overlay covering the whole screen, above every other window
	const onTop = true
	show := st.DisplayOn || st.Test
	if st.SingleScreen {
		show = st.Showing || st.Test
	}
	active := map[int]bool{}
	for _, d := range st.Displays {
		active[d.Disp] = true
		if haveWarmWindows && d.Screen >= 0 {
			// the window exists (invisible) before it is first needed, so showing it later is only making it visible
			s.mu.Lock()
			have, busy := s.projectors[d.Disp] != nil, s.pending[d.Disp]
			s.mu.Unlock()
			if !have && !busy {
				go s.prepareProjector(d.Disp, d.Screen, onTop)
			}
		}
		// a display whose screen is gone stays hidden until the screen comes back
		s.applyProjector(d.Disp, d.Screen, show && d.Screen >= 0, onTop, st.Version)
	}
	s.mu.Lock()
	var stale []int
	for disp := range s.projectors {
		if !active[disp] {
			stale = append(stale, disp)
		}
	}
	s.mu.Unlock()
	for _, disp := range stale {
		s.applyProjector(disp, 0, false, onTop, st.Version)
	}
}

// applyProjector shows or hides a display window. Both wait for the page of the window, not for a time: a window is shown
// when its page has the state that asked for it (before that it would flash what it showed last) and hidden when its page
// has drawn the state that asked for that (hiding earlier would leave the last picture in the window for the next show).
func (s *Shell) applyProjector(disp, screen int, want, onTop bool, version int64) {
	s.mu.Lock()
	s.want[disp] = want
	if !want {
		s.shown[disp] = false
		s.mu.Unlock()
		s.hideWhenSettled(disp, screen, onTop, version)
		return
	}
	if s.shown[disp] {
		s.mu.Unlock()
		s.ShowProjector(disp, screen, true, onTop)
		return
	}
	if s.pending[disp] {
		s.mu.Unlock()
		return // already on its way
	}
	s.pending[disp] = true
	s.mu.Unlock()
	go s.revealWhenReady(disp, screen, onTop, version)
}

// revealWhenReady shows the window of display disp as soon as its page has the state of that version. The waits end at the
// page's report; their limits are only for a page that never answers.
func (s *Shell) revealWhenReady(disp, screen int, onTop bool, version int64) {
	defer func() {
		s.mu.Lock()
		delete(s.pending, disp)
		s.mu.Unlock()
	}()
	start := time.Now()
	s.mu.Lock()
	s.revealAt[disp] = start
	s.mu.Unlock()
	s.prepareProjector(disp, screen, onTop)
	if !s.engine.WaitPageReady(disp, 3*time.Second) {
		log.Printf("display %d: its page did not report in 3 s, showing it anyway", disp)
	}
	// a window that is on screen (even if invisible) has painted the state when the page says so; one that is hidden only
	// has it in its document
	wait := s.engine.WaitPageApplied
	if haveWarmWindows {
		wait = s.engine.WaitPageSettled
	}
	if !wait(disp, version, time.Second) {
		log.Printf("display %d: its page did not report state %d in 1 s, showing it anyway", disp, version)
	}
	s.mu.Lock()
	ok := s.want[disp]
	s.shown[disp] = ok
	s.mu.Unlock()
	if ok {
		log.Printf("display %d: page had state %d after %d ms", disp, version, time.Since(start).Milliseconds())
		s.ShowProjector(disp, screen, true, onTop)
	}
}

// hideWhenSettled hides a display window as soon as its page has drawn the state that asked for it. Turning the
// presentation off is a cut (no fade), so that takes a couple of frames.
func (s *Shell) hideWhenSettled(disp, screen int, onTop bool, version int64) {
	s.mu.Lock()
	w := s.projectors[disp]
	visible := s.visible[disp]
	s.mu.Unlock()
	if w != nil && !haveWarmWindows {
		application.InvokeSync(func() { visible = w.IsVisible() })
	}
	if !visible {
		s.ShowProjector(disp, screen, false, onTop)
		return
	}
	go func() {
		s.engine.WaitPageSettled(disp, version, 500*time.Millisecond)
		s.mu.Lock()
		still := !s.want[disp]
		s.mu.Unlock()
		if still {
			s.ShowProjector(disp, screen, false, onTop)
		}
	}()
}

// projectorEvent is what the native side reports about a display window (see presentation_darwin.go).
func (s *Shell) projectorEvent(disp, kind int) {
	s.mu.Lock()
	s.visible[disp] = kind == 2
	start := s.revealAt[disp]
	if kind == 2 {
		delete(s.revealAt, disp)
	}
	s.mu.Unlock()
	if kind == 2 && !start.IsZero() {
		log.Printf("display %d: visible %d ms after the state reached it", disp, time.Since(start).Milliseconds())
	}
}

// prepareProjector makes sure the window of display disp exists. Where windows stay on screen it is also ordered in,
// invisible, so its page runs from the start.
func (s *Shell) prepareProjector(disp, screen int, onTop bool) {
	screens := s.wailsScreens()
	if screen < 0 || screen >= len(screens) {
		return
	}
	sc := screens[screen]
	application.InvokeSync(func() { s.projectorWindow(disp, sc, onTop) })
}

// projectorWindow returns the window of display disp, made (hidden, or invisible and ordered in) when it does not exist.
// It runs on the main thread.
func (s *Shell) projectorWindow(disp int, sc *application.Screen, onTop bool) *application.WebviewWindow {
	s.mu.Lock()
	w := s.projectors[disp]
	s.mu.Unlock()
	if w != nil {
		return w
	}
	level := application.MacWindowLevelNormal
	if onTop {
		level = application.MacWindowLevelStatus
	}
	w = s.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             fmt.Sprintf("projector-%d", disp),
		Title:            fmt.Sprintf("%s Display %d", s.name, disp),
		URL:              fmt.Sprintf("%s/projector/%d", s.url, disp),
		Frameless:        true,
		DisableResize:    true,
		Hidden:           true,
		X:                sc.Bounds.X,
		Y:                sc.Bounds.Y,
		Width:            sc.Bounds.Width,
		Height:           sc.Bounds.Height,
		InitialPosition:  application.WindowXY,
		AlwaysOnTop:      onTop,
		BackgroundColour: application.NewRGB(0, 0, 0),
		// a true borderless window: square corners, edge to edge on the screen
		Mac:     application.MacWindow{DisableShadow: true, WindowLevel: level, DisableEscapeExitsFullscreen: true, CornerType: application.MacWindowCornerTypeSquare},
		Windows: application.WindowsWindow{DisableMenu: true},
	})
	s.mu.Lock()
	s.projectors[disp] = w
	s.mu.Unlock()
	w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		s.mu.Lock()
		delete(s.projectors, disp)
		delete(s.visible, disp)
		s.mu.Unlock()
	})
	if haveWarmWindows {
		w.SetBounds(application.Rect{X: sc.Bounds.X, Y: sc.Bounds.Y, Width: sc.Bounds.Width, Height: sc.Bounds.Height})
		setProjectorVisible(w.NativeWindow(), disp, false, true)
	}
	return w
}

// ShowProjector shows or hides a display window (making it first when it is not there yet).
func (s *Shell) ShowProjector(disp int, screen int, show bool, onTop bool) {
	screens := s.wailsScreens()
	if len(screens) == 0 {
		return
	}
	if screen < 0 || screen >= len(screens) {
		// the screen is not there (any more): never move the window to another one
		show = false
		screen = 0
	}
	sc := screens[screen]
	log.Printf("display %d: %s on screen %d (%d,%d %dx%d)", disp, map[bool]string{true: "show", false: "hide"}[show], screen, sc.Bounds.X, sc.Bounds.Y, sc.Bounds.Width, sc.Bounds.Height)
	application.InvokeSync(func() {
		s.mu.Lock()
		w := s.projectors[disp]
		s.mu.Unlock()
		if w == nil {
			if !show {
				return
			}
			w = s.projectorWindow(disp, sc, onTop)
		}
		if show {
			w.SetAlwaysOnTop(onTop)
			w.SetBounds(application.Rect{X: sc.Bounds.X, Y: sc.Bounds.Y, Width: sc.Bounds.Width, Height: sc.Bounds.Height})
			if haveWarmWindows {
				setProjectorVisible(w.NativeWindow(), disp, true, false)
			} else {
				w.Show()
				if runtime.GOOS != "darwin" {
					w.SetBounds(application.Rect{X: sc.Bounds.X, Y: sc.Bounds.Y, Width: sc.Bounds.Width, Height: sc.Bounds.Height})
				}
			}
			// one screen: the projection covers the operator's screen, so the Dock and menu bar must not show on it
			if len(screens) == 1 {
				hideDockAndMenuBar(true)
			}
			if s.main != nil {
				s.main.Focus()
			}
		} else if haveWarmWindows {
			setProjectorVisible(w.NativeWindow(), disp, false, false)
			hideDockAndMenuBar(false)
		} else if w.IsVisible() {
			w.Hide()
			hideDockAndMenuBar(false)
		}
	})
}
