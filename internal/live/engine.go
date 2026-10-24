package live

import (
	"fmt"
	"log"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"

	"wordprojector/internal/announce"
	"wordprojector/internal/bible"
	"wordprojector/internal/db"
	"wordprojector/internal/media"
	"wordprojector/internal/qt"
	"wordprojector/internal/schedule"
	"wordprojector/internal/settings"
	"wordprojector/internal/slides"
	"wordprojector/internal/songs"
	"wordprojector/internal/theme"
)

// Screens reports how many physical screens exist (0 = unknown / headless).
type Screens interface {
	ScreenCount() int
	// ScreenSize is the size in pixels of screen i (0, 0 when unknown).
	ScreenSize(i int) (int, int)
}

// Engine is the single source of truth for what is projected.
type Engine struct {
	mu sync.Mutex

	testSlots  []int
	testTimer  *time.Timer
	testMillis int

	DB       *db.DB
	Songs    *songs.Repo
	Bibles   *bible.Repo
	Ann      *announce.Repo
	Themes   *theme.Repo
	Slides   *slides.Repo
	Media    *media.Repo
	Schedule *schedule.Schedule

	Settings settings.Settings
	Theme    theme.Theme

	// Broadcast sends a message to every websocket client.
	Broadcast func(msg any)
	// OnDisplays is called when projector windows must be shown or hidden.
	OnDisplays func(st State)
	Screens Screens

	state State
	bgVer int64

	// preview is a sample slide from the theme editor that every display shows until it is cleared; it is dropped
	// by itself when the editor stops refreshing it. previewPrev is how the displays were before it started.
	preview     *Slide
	previewPrev struct{ on, showing bool }
	previewTimer *time.Timer

	// what the display pages (the windows' web pages) have reported, so a window is shown only when its page is
	// ready and hidden only after its page has drawn the final state (no old picture left behind for the next show)
	pagesMu     sync.Mutex
	pageReady   map[int]bool
	pageSettled map[int]int64

	// current sources
	verseIDs  []string
	curSong   songs.Song
	songBlob  string
	curAnn    announce.Announcement
	curPics   []PictureSlide
	curVideo  media.Item
	tmpImages map[string]*slides.Images

	autoStop chan struct{}
}

// New creates the engine and loads settings and the current theme.
func New(d *db.DB) (*Engine, error) {
	e := &Engine{DB: d, tmpImages: map[string]*slides.Images{}}
	e.Songs = &songs.Repo{DB: d}
	e.Bibles = bible.NewRepo(d)
	e.Ann = &announce.Repo{DB: d}
	e.Themes = &theme.Repo{DB: d}
	if err := e.Themes.EnsureBuiltins(); err != nil {
		return nil, err
	}
	e.Slides = &slides.Repo{DB: d}
	e.Media = &media.Repo{DB: d}
	e.Schedule = schedule.New()
	s, err := settings.Load(d)
	if err != nil {
		return nil, err
	}
	e.Settings = s
	t, err := e.Themes.Load(s.General.CurrentThemeID)
	if err != nil {
		return nil, err
	}
	e.Theme = t
	if s.General.CurrentThemeID != t.ID {
		e.Settings.General.CurrentThemeID = t.ID
		_ = settings.Save(d, e.Settings)
	}
	e.ensureBibleDefaults()
	e.state = State{Items: []Item{}, Selection: []int{}, Displays: []DisplayState{}, Video: VideoState{Volume: 100}}
	e.state.DisplayOn = e.Settings.General.DisplayOnStartUp
	e.bgVer = time.Now().Unix()
	e.recompute()
	return e, nil
}

// ensureBibleDefaults picks the first bible as primary when none is set or
// the configured one no longer exists (port of the reload_bible logic).
func (e *Engine) ensureBibleDefaults() {
	list, err := e.Bibles.List()
	if err != nil {
		return
	}
	exists := func(id string) bool {
		for _, b := range list {
			if strconv.FormatInt(b.ID, 10) == id {
				return true
			}
		}
		return false
	}
	changed := false
	for i := range e.Settings.Bible {
		b := &e.Settings.Bible[i]
		if !exists(b.Primary) {
			if len(list) > 0 {
				b.Primary = strconv.FormatInt(list[0].ID, 10)
			} else {
				b.Primary = "none"
			}
			changed = true
		}
		if b.Secondary != "none" && !exists(b.Secondary) {
			b.Secondary = "none"
			changed = true
		}
		if b.Trinary != "none" && !exists(b.Trinary) {
			b.Trinary = "none"
			changed = true
		}
		if i == 0 && b.Operator != "same" && !exists(b.Operator) {
			b.Operator = "same"
			changed = true
		}
	}
	if changed {
		_ = settings.Save(e.DB, e.Settings)
	}
}

// OperatorBibleID returns the bible id used for browsing in the Bible tab.
func (e *Engine) OperatorBibleID() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.operatorBibleID()
}

// operatorBibleID: the Bible browsed in the Bible tab is the one set as default (when it still exists), else the
// primary Bible of display 1.
func (e *Engine) operatorBibleID() int64 {
	if d := int64(e.Settings.App.DefaultBible); d > 0 && e.Bibles != nil {
		if _, err := e.Bibles.Get(d); err == nil {
			return d
		}
	}
	n, _ := strconv.ParseInt(e.Settings.Bible[0].Primary, 10, 64)
	return n
}

// State returns a copy of the current state.
func (e *Engine) State() State {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}

// Locked runs fn with the engine locked (for settings/theme updates).
func (e *Engine) Locked(fn func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	fn()
}

// BumpBackgrounds invalidates cached background URLs.
func (e *Engine) BumpBackgrounds() {
	e.mu.Lock()
	e.bgVer = time.Now().UnixNano()
	e.recompute()
	e.mu.Unlock()
	e.publish()
}

// Refresh recomputes the slides (after settings or theme change) and broadcasts.
func (e *Engine) Refresh() {
	e.mu.Lock()
	e.ensureBibleDefaults()
	e.recompute()
	e.mu.Unlock()
	e.publish()
}

func (e *Engine) publish() {
	e.mu.Lock()
	st := e.state
	e.mu.Unlock()
	if e.Broadcast != nil {
		e.Broadcast(map[string]any{"type": "state", "state": st})
	}
	if e.OnDisplays != nil {
		e.OnDisplays(st)
	}
}

// TempImages stores prepared images for a not yet saved slide show and
// returns the key.
func (e *Engine) AddTempImage(key string, img *slides.Images) {
	e.mu.Lock()
	e.tmpImages[key] = img
	e.mu.Unlock()
}

// TempImage returns a temp image variant ("full","small","prev").
func (e *Engine) TempImage(key, size string) ([]byte, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	img, ok := e.tmpImages[key]
	if !ok {
		// maybe a schedule blob
		if b, ok := e.Schedule.Blob(key + ":" + size); ok {
			return b, true
		}
		if b, ok := e.Schedule.Blob(key); ok {
			return b, true
		}
		return nil, false
	}
	switch size {
	case "small":
		return img.Small, true
	case "prev":
		return img.Preview, true
	}
	return img.Full, true
}

// TempImages returns all temp images (used when saving a slide show).
func (e *Engine) TempImages() map[string]*slides.Images {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := map[string]*slides.Images{}
	for k, v := range e.tmpImages {
		out[k] = v
	}
	return out
}

// ---- display configuration ------------------------------------------------

func (e *Engine) screenCount() int {
	if e.Screens == nil {
		return 0
	}
	return e.Screens.ScreenCount()
}

// activeDisplays returns the display numbers (1-4) and the screen index each
// display is placed on.
func (e *Engine) activeDisplays() []DisplayState {
	g := e.Settings.General
	n := e.screenCount()
	if e.state.Test && len(e.testSlots) > 0 {
		// the test uses the screens picked in the (possibly not yet saved) settings dialog
		var out []DisplayState
		seen := map[int]bool{}
		for i, sc := range e.testSlots {
			if sc < 0 || (n > 0 && sc >= n) || seen[sc] {
				continue
			}
			seen[sc] = true
			out = append(out, DisplayState{Disp: i + 1, Screen: sc})
		}
		if len(out) > 0 {
			return out
		}
	}
	out := []DisplayState{{Disp: 1, Screen: g.DisplayScreen}}
	if n == 1 {
		out[0].Screen = 0
		return out
	}
	if n > 1 && (g.DisplayScreen < 0 || g.DisplayScreen >= n) {
		// the screen display 1 is set to was unplugged: it has no screen for now (never fall back to the
		// operator's own screen, an always-on-top window there would cover the controls)
		out[0].Screen = -1
	}
	add := func(d, screen int) {
		if screen < 0 {
			return
		}
		if n > 0 && screen >= n {
			return
		}
		for _, o := range out {
			if o.Screen == screen {
				return
			}
		}
		if n > 1 && screen >= n {
			return
		}
		out = append(out, DisplayState{Disp: d, Screen: screen})
	}
	add(2, g.DisplayScreen2)
	add(3, g.DisplayScreen3)
	add(4, g.DisplayScreen4)
	for i, sc := range g.DisplayScreensExtra {
		add(5+i, sc)
	}
	return out
}

// ---- show actions -----------------------------------------------------------

// blankFirst clears the preview before new content is started from a stopped presentation, so the old
// content can never flash on a display (or in the operator preview) while the presentation turns on.
func (e *Engine) blankFirst() {
	e.mu.Lock()
	if e.state.Showing || e.state.Type == "" || e.state.Cleared {
		e.mu.Unlock()
		return
	}
	e.state.Cleared = true
	e.recompute()
	e.mu.Unlock()
	e.publish()
}


func (e *Engine) startShowing(kind string, title string, items []Item, index int) {
	e.stopAuto()
	e.dropPreviewLocked()
	e.state.Type = kind
	e.state.Title = title
	e.state.Notes = ""
	e.state.Items = items
	e.state.Index = index
	if index < 0 || index >= len(items) {
		e.state.Index = 0
	}
	e.state.Selection = []int{e.state.Index}
	e.state.MultiVerse = false
	e.state.Showing = true
	e.state.Cleared = false
	e.state.Video = VideoState{Volume: e.state.Video.Volume, Muted: e.state.Video.Muted}
	if e.screenCount() != 1 {
		e.state.DisplayOn = true
	}
}

// ShowBible shows a chapter of the operator bible with the given selected rows.
func (e *Engine) ShowBible(book, chapter int, selection []int) error {
	e.blankFirst()
	e.mu.Lock()
	id := e.operatorBibleID()
	if id == 0 {
		e.mu.Unlock()
		return fmt.Errorf("no bible available")
	}
	verses, err := e.Bibles.Chapter(id, book, chapter)
	if err != nil {
		e.mu.Unlock()
		return err
	}
	books, _ := e.Bibles.Books(id)
	bookName := ""
	for _, b := range books {
		if b.ID == book {
			bookName = b.Name
		}
	}
	items := make([]Item, len(verses))
	e.verseIDs = make([]string, len(verses))
	for i, v := range verses {
		items[i] = Item{Text: fmt.Sprintf("%d. %s", v.Num, v.Text)}
		e.verseIDs[i] = v.IDs
	}
	sel := cleanSelection(selection, len(items))
	first := 0
	if len(sel) > 0 {
		first = sel[0]
	}
	e.startShowing("bible", fmt.Sprintf("%s %d", bookName, chapter), items, first)
	e.state.Selection = sel
	e.state.MultiVerse = len(sel) > 1
	e.recompute()
	e.mu.Unlock()
	e.publish()
	return nil
}

func cleanSelection(sel []int, n int) []int {
	out := []int{}
	seen := map[int]bool{}
	for _, s := range sel {
		if s >= 0 && s < n && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Ints(out)
	if len(out) == 0 && n > 0 {
		out = []int{0}
	}
	return out
}

// ShowBibleRef shows a verse range identified by comma separated verse ids.
func (e *Engine) ShowBibleRef(verseIDs string) error {
	e.blankFirst()
	e.mu.Lock()
	id := e.operatorBibleID()
	e.mu.Unlock()
	if id == 0 {
		return fmt.Errorf("no bible available")
	}
	book, chapter, first, last, _, err := e.Bibles.Ref(id, verseIDs)
	if err != nil {
		return err
	}
	verses, err := e.Bibles.Chapter(id, book, chapter)
	if err != nil {
		return err
	}
	var sel []int
	for i, v := range verses {
		if v.Num >= first && v.Num <= last {
			sel = append(sel, i)
		}
	}
	return e.ShowBible(book, chapter, sel)
}

// ShowSong shows a song (full record) starting at row.
func (e *Engine) ShowSong(s songs.Song, row int, blobKey string) {
	e.blankFirst()
	list := songs.StanzaList(s.Text)
	items := make([]Item, len(list))
	for i, t := range list {
		items[i] = Item{Text: t}
	}
	e.mu.Lock()
	e.curSong = s
	e.songBlob = blobKey
	e.startShowing("song", s.Title, items, row)
	e.state.Notes = s.Notes
	e.recompute()
	e.mu.Unlock()
	e.publish()
	if s.ID > 0 {
		_ = e.Songs.AddCount(s.ID)
	}
}

// ShowAnnouncement shows an announcement starting at row.
func (e *Engine) ShowAnnouncement(a announce.Announcement, row int) {
	e.blankFirst()
	list := announce.SlideList(a.Text)
	items := make([]Item, len(list))
	for i, t := range list {
		items[i] = Item{Text: t}
	}
	e.mu.Lock()
	e.curAnn = a
	e.startShowing("announce", a.Title, items, row)
	e.recompute()
	if a.UseAuto && len(items) > 1 {
		e.startAuto(a.SlideTime, a.Loop)
	}
	e.mu.Unlock()
	e.publish()
}

// ShowSlideshow shows pictures starting at row.
func (e *Engine) ShowSlideshow(ss slides.ShowWithSlides, row int) {
	e.blankFirst()
	items := make([]Item, len(ss.Slides))
	pics := make([]PictureSlide, len(ss.Slides))
	for i, sl := range ss.Slides {
		var full, small string
		if sl.ID > 0 {
			full = fmt.Sprintf("/api/slides/%d/image?size=full", sl.ID)
			small = fmt.Sprintf("/api/slides/%d/image?size=small", sl.ID)
		} else {
			full = "/api/blob/" + url.PathEscape(sl.Temp) + "?size=full"
			small = "/api/blob/" + url.PathEscape(sl.Temp) + "?size=small"
		}
		items[i] = Item{Text: sl.Name, Image: small}
		pics[i] = PictureSlide{URL: full, Width: sl.Width, Height: sl.Height}
	}
	e.mu.Lock()
	e.curPics = pics
	e.startShowing("picture", ss.Name, items, row)
	e.recompute()
	e.mu.Unlock()
	e.publish()
}

// ShowVideo shows (and starts) a media file.
func (e *Engine) ShowVideo(m media.Item) {
	e.blankFirst()
	e.mu.Lock()
	e.curVideo = m
	e.startShowing("video", m.Name, []Item{}, 0)
	e.state.Selection = []int{}
	e.state.Video = VideoState{Volume: 100, Playing: true}
	e.recompute()
	e.mu.Unlock()
	e.publish()
	if e.Broadcast != nil {
		e.Broadcast(map[string]any{"type": "video", "cmd": "play"})
	}
}

// SetIndex selects a live list row. While the presentation is running it is projected at once;
// otherwise the row is only selected, unless force is set (double click), which starts the presentation.
func (e *Engine) SetIndex(i int, force bool) {
	if force {
		e.blankFirst()
	}
	e.mu.Lock()
	if i < 0 || i >= len(e.state.Items) {
		e.mu.Unlock()
		return
	}
	e.state.Index = i
	if !e.state.MultiVerse {
		e.state.Selection = []int{i}
	}
	e.state.Cleared = false
	if e.state.Showing || force {
		e.state.Showing = true
		if force && e.screenCount() != 1 {
			e.state.DisplayOn = true
		}
	}
	e.recompute()
	e.mu.Unlock()
	e.publish()
}

// SetSelection sets a (contiguous) multi verse selection.
func (e *Engine) SetSelection(sel []int, project bool) {
	e.mu.Lock()
	sel = cleanSelection(sel, len(e.state.Items))
	if len(sel) > 0 {
		e.state.Index = sel[0]
	}
	e.state.Selection = sel
	if len(sel) > 0 {
		e.state.Cleared = false
	}
	if project {
		e.state.MultiVerse = len(sel) > 1
		e.state.Showing = true
		e.state.Cleared = false
		if e.screenCount() != 1 {
			e.state.DisplayOn = true
		}
	}
	e.recompute()
	e.mu.Unlock()
	e.publish()
}

// SetMultiVerse toggles multi verse selection mode.
func (e *Engine) SetMultiVerse(on bool) {
	e.mu.Lock()
	e.state.MultiVerse = on
	if !on {
		e.state.Selection = []int{e.state.Index}
		e.recompute()
	}
	e.mu.Unlock()
	e.publish()
}

// Next selects the next slide (port of nextSlide).
func (e *Engine) Next() {
	e.mu.Lock()
	cur := e.state.Index
	n := len(e.state.Items)
	if e.state.MultiVerse && len(e.state.Selection) > 0 {
		cur = e.state.Selection[len(e.state.Selection)-1]
		if cur < n-1 {
			e.state.MultiVerse = false
		}
	}
	if cur < n-1 {
		cur++
		e.state.Index = cur
		e.state.Selection = []int{cur}
		e.state.Showing = true
		e.state.Cleared = false
		e.recompute()
	}
	e.mu.Unlock()
	e.publish()
}

// Prev selects the previous slide.
func (e *Engine) Prev() {
	e.mu.Lock()
	cur := e.state.Index
	if e.state.MultiVerse && len(e.state.Selection) > 0 {
		cur = e.state.Selection[0]
		if cur > 0 {
			e.state.MultiVerse = false
		}
	}
	if cur > 0 {
		cur--
		e.state.Index = cur
		e.state.Selection = []int{cur}
		e.state.Showing = true
		e.state.Cleared = false
		e.recompute()
	}
	e.mu.Unlock()
	e.publish()
}

// Show resumes projecting the selected item (F4).
func (e *Engine) Show() {
	e.mu.Lock()
	if e.state.Type != "" {
		// Play always brings the content back, also after a pause followed by turning the presentation off
		e.state.Cleared = false
		e.state.Showing = true
		if e.screenCount() != 1 {
			e.state.DisplayOn = true
		}
	}
	e.recompute()
	e.mu.Unlock()
	e.publish()
}

// PreviewSelection renders what display 1 would show for a staged (not yet projected) selection
// of the current list, without changing the live state.
func (e *Engine) PreviewSelection(sel []int) Slide {
	e.mu.Lock()
	defer e.mu.Unlock()
	saved := e.state
	e.state.Selection = cleanSelection(sel, len(e.state.Items))
	if len(e.state.Selection) > 0 {
		e.state.Index = e.state.Selection[0]
	}
	e.state.MultiVerse = len(e.state.Selection) > 1
	e.state.Showing, e.state.Cleared = true, false
	slide := e.slideFor(1)
	e.state = saved
	return slide
}

// TestDisplays shows a test pattern on the given screens (the screen index of display 1, 2, ...) for a
// short time, so the operator can see that every display screen works. Nothing else changes.
func (e *Engine) TestDisplays(slots []int, d time.Duration) {
	e.mu.Lock()
	if e.testTimer != nil {
		e.testTimer.Stop()
	}
	e.testSlots = append([]int(nil), slots...)
	e.testMillis = int(d.Milliseconds())
	e.state.Test = true
	e.recompute()
	e.testTimer = time.AfterFunc(d, func() {
		e.mu.Lock()
		e.state.Test = false
		e.testSlots = nil
		e.recompute()
		e.mu.Unlock()
		e.publish()
	})
	e.mu.Unlock()
	e.publish()
}

// Escape is the Esc key, the same wherever it is pressed (operator window or a display window): the first press
// pauses what is displayed, the second turns the presentation off (the Off button): the list of live items stays.
func (e *Engine) Escape() {
	e.mu.Lock()
	single := e.state.SingleScreen
	on := e.state.DisplayOn
	if single {
		on = e.state.Showing
	}
	visible := on && e.state.Type != "" && !e.state.Cleared
	e.mu.Unlock()
	if visible {
		e.Pause()
		return
	}
	e.TurnOff()
}

// Stop turns the presentation off and clears the list of live items (Del).
func (e *Engine) Stop() {
	e.TurnOff()
	e.Clear()
}

// TurnOff is the Off button: the displays go dark and the list of live items stays.
func (e *Engine) TurnOff() {
	e.mu.Lock()
	single := e.state.SingleScreen
	on := e.state.DisplayOn
	if single {
		on = e.state.Showing
	}
	e.mu.Unlock()
	if !on {
		return
	}
	if single {
		e.Hide()
	} else {
		e.SetDisplayOn(false)
	}
}

// Hide shows the passive screen (Esc).
func (e *Engine) Hide() {
	e.mu.Lock()
	e.dropPreviewLocked()
	e.stopAuto()
	e.state.Showing = false
	e.state.Video.Playing = false
	e.recompute()
	e.mu.Unlock()
	e.publish()
	if e.Broadcast != nil {
		e.Broadcast(map[string]any{"type": "video", "cmd": "stop"})
	}
}

// Clear empties the presentation: nothing is displayed and the list of live items is gone.
func (e *Engine) Clear() {
	e.mu.Lock()
	e.dropPreviewLocked()
	e.stopAuto()
	e.state.Type, e.state.Title, e.state.Notes = "", "", ""
	e.state.Items, e.state.Selection = nil, nil
	e.state.Index, e.state.MultiVerse, e.state.Cleared = 0, false, false
	e.state.Video = VideoState{Volume: e.state.Video.Volume, Muted: e.state.Video.Muted}
	e.recompute()
	e.mu.Unlock()
	e.publish()
	if e.Broadcast != nil {
		e.Broadcast(map[string]any{"type": "video", "cmd": "stop"})
	}
}

// Pause blanks the display to plain black until Play (or a newly selected item) shows content again (Shift+Esc).
func (e *Engine) Pause() {
	e.mu.Lock()
	e.dropPreviewLocked()
	// works with the presentation off too: the preview goes blank until something is selected
	if e.state.Type != "" {
		e.state.Cleared = true
	}
	e.recompute()
	e.mu.Unlock()
	e.publish()
}

// SetDisplayOn turns projector windows on/off (multi screen mode).
func (e *Engine) SetDisplayOn(on bool) {
	e.mu.Lock()
	if !on {
		e.dropPreviewLocked()
	}
	e.state.DisplayOn = on
	if on && e.state.Type != "" {
		// turning the displays on shows what the preview box shows
		e.state.Showing = true
	}
	if !on {
		e.state.Showing = false
		e.state.Video.Playing = false
		e.stopAuto()
	}
	e.recompute()
	e.mu.Unlock()
	e.publish()
	if !on && e.Broadcast != nil {
		e.Broadcast(map[string]any{"type": "video", "cmd": "stop"})
	}
}

// VideoCommand forwards a player command to the display windows.
func (e *Engine) VideoCommand(cmd string, value float64) {
	e.mu.Lock()
	switch cmd {
	case "volume":
		e.state.Video.Volume = int(value)
	case "mute":
		e.state.Video.Muted = value != 0
	case "play":
		e.state.Video.Playing = true
		e.state.Video.Ended = false
	case "pause":
		e.state.Video.Playing = false
	case "stop":
		e.state.Video.Playing = false
		e.state.Video.Position = 0
	}
	e.mu.Unlock()
	if e.Broadcast != nil {
		e.Broadcast(map[string]any{"type": "video", "cmd": cmd, "value": value})
	}
	if cmd == "volume" || cmd == "mute" {
		e.publish()
	}
}

// VideoStatus is called by the display window reporting playback progress.
func (e *Engine) VideoStatus(position, duration float64, playing, ended bool) {
	e.mu.Lock()
	e.state.Video.Position = position
	e.state.Video.Duration = duration
	e.state.Video.Playing = playing
	e.state.Video.Ended = ended
	st := e.state
	if ended && e.state.Type == "video" && e.state.Showing {
		// Like the original: when the video stops, go back to passive.
		e.state.Showing = false
		e.recompute()
		st = e.state
		e.mu.Unlock()
		e.publish()
		return
	}
	e.mu.Unlock()
	if e.Broadcast != nil {
		e.Broadcast(map[string]any{"type": "videoStatus", "video": st.Video})
	}
}

// ---- auto advance (announcements) -------------------------------------------

func (e *Engine) startAuto(seconds int, loop bool) {
	if seconds <= 0 {
		seconds = 10
	}
	stop := make(chan struct{})
	e.autoStop = stop
	go func() {
		t := time.NewTicker(time.Duration(seconds) * time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				e.mu.Lock()
				if !e.state.Showing || e.state.Type != "announce" {
					e.mu.Unlock()
					return
				}
				n := len(e.state.Items)
				i := e.state.Index + 1
				if i >= n {
					if !loop {
						e.mu.Unlock()
						return
					}
					i = 0
				}
				e.state.Index = i
				e.state.Selection = []int{i}
				e.recompute()
				e.mu.Unlock()
				e.publish()
			}
		}
	}()
}

func (e *Engine) stopAuto() {
	if e.autoStop != nil {
		close(e.autoStop)
		e.autoStop = nil
	}
}

// ---- slide computation --------------------------------------------------------

func (e *Engine) recompute() {
	e.state.Version++
	n := e.screenCount()
	e.state.SingleScreen = n == 1
	disps := e.activeDisplays()
	g := e.Settings.General
	e.state.Controls = Controls{Visible: e.state.SingleScreen && !e.Settings.App.HideDisplayControls, ButtonSize: g.DisplayControls.ButtonSize,
		AlignV: g.DisplayControls.AlignV, AlignH: g.DisplayControls.AlignH, Opacity: g.DisplayControls.Opacity}
	for i := range disps {
		disps[i].Slide = e.slideFor(disps[i].Disp)
		if e.Screens != nil {
			disps[i].Width, disps[i].Height = e.Screens.ScreenSize(disps[i].Screen)
		}
		if disps[i].Slide.Test != nil {
			disps[i].Slide.Test.Screen = disps[i].Screen
			disps[i].Slide.Test.Width, disps[i].Slide.Test.Height = disps[i].Width, disps[i].Height
		}
		disps[i].Preview = disps[i].Slide
	}
	if !e.state.Showing && e.state.Type != "" {
		// the operator preview shows what Play would put on the displays
		showing := e.state.Showing
		e.state.Showing = true
		for i := range disps {
			disps[i].Preview = e.slideFor(disps[i].Disp)
		}
		e.state.Showing = showing
	}
	e.state.Displays = disps
	if e.state.Items == nil {
		e.state.Items = []Item{}
	}
	if e.state.Selection == nil {
		e.state.Selection = []int{}
	}
}

// passiveBackground is the background shown when nothing is projected: the picture the theme has chosen for it,
// else black.
func (e *Engine) passiveBackground(d int) Background {
	p := e.Theme.Passive
	if p.UseBackground && p.ImageID > 0 {
		fit := map[int]string{0: "fill", 1: "cover", 2: "contain"}[p.BackgroundFit]
		return Background{Type: "image", URL: fmt.Sprintf("/api/bg/theme/passive/1?theme=%d&v=%d&i=%d", e.Theme.ID, e.bgVer, p.ImageID), Fit: fit}
	}
	return Background{Type: "color", Color: "#000000"}
}

// textBackground is the color behind Bible, song and announcement text, as opaque as the theme says. What shows through
// is the picture the theme has for the idle screen, if it uses one, else black.
func (e *Engine) textBackground() Background {
	return e.themeBackground(e.Theme)
}

func (e *Engine) themeBackground(th theme.Theme) Background {
	color := th.BackgroundColor
	if color == "" {
		color = "#000000"
	}
	bg := Background{Type: "color", Color: color, Clear: 100 - th.BackgroundOpacity}
	if bg.Clear < 0 || bg.Clear > 100 {
		bg.Clear = 0
	}
	if bg.Clear > 0 {
		if p := th.Passive; p.UseBackground && p.ImageID > 0 {
			fit := map[int]string{0: "fill", 1: "cover", 2: "contain"}[p.BackgroundFit]
			bg.Under = &Background{Type: "image", URL: fmt.Sprintf("/api/bg/theme/passive/1?theme=%d&v=%d&i=%d", th.ID, e.bgVer, p.ImageID), Fit: fit}
		}
	}
	return bg
}

// slideFor is what display d shows now, with the user's transition (Settings > Presentation) applied.
func (e *Engine) slideFor(d int) Slide {
	sl := e.rawSlideFor(d)
	if sl.Kind != "video" && sl.Kind != "test" && e.preview == nil { // a preview brings its own transition
		t := e.Settings.App.Transition
		sl.Fade, sl.FadeMillis = !t.Cut, t.FadeMillis
	}
	// turning the presentation off is a cut: the window goes away at once and nothing fades out on the way
	if on := e.state.DisplayOn; (e.state.SingleScreen && !e.state.Showing) || (!e.state.SingleScreen && !on) {
		sl.Fade = false
	}
	return sl
}

func (e *Engine) rawSlideFor(d int) Slide {
	st := e.state
	if st.Test {
		return Slide{Kind: "test", Background: Background{Type: "color", Color: "#000000"}, Test: &TestSlide{Disp: d, Millis: e.testMillis}}
	}
	if e.preview != nil {
		return *e.preview
	}
	if !st.Showing || st.Type == "" {
		return Slide{Kind: "passive", Fade: true, Background: e.passiveBackground(d)}
	}
	if st.Cleared {
		// Clear leaves a blank black screen: no text and no theme background
		return Slide{Kind: "clear", Fade: true, Background: Background{Type: "color", Color: "#000000"}}
	}
	switch st.Type {
	case "bible":
		return e.bibleSlide(d)
	case "song":
		return e.songSlide(d)
	case "announce":
		return e.announceSlide(d)
	case "picture":
		return e.pictureSlide(d)
	case "video":
		return e.videoSlide(d)
	}
	return Slide{Kind: "passive", Background: e.passiveBackground(d)}
}

func (e *Engine) bibleSlide(d int) Slide {
	bi := min(max(d, 1), theme.Displays) - 1 // the version buttons are per display; the look is the same
	b := e.Theme.Bible
	sl := Slide{Kind: "bible", Fade: true, Background: e.textBackground()}
	if e.state.Cleared {
		sl.Kind = "clear"
		return sl
	}
	var ids []string
	for _, i := range e.state.Selection {
		if i >= 0 && i < len(e.verseIDs) {
			ids = append(ids, e.verseIDs[i])
		}
	}
	if len(ids) == 0 {
		sl.Kind = "clear"
		return sl
	}
	verseIDs := joinIDs(ids)
	vs := e.Settings.Bible[bi]
	var parts []Passage
	addPart := func(id string) {
		n, err := strconv.ParseInt(id, 10, 64)
		if err != nil || n == 0 {
			return
		}
		p, err := e.Bibles.VerseAndCaption(n, verseIDs, false)
		if err != nil {
			log.Printf("bible passage: %v", err)
			return
		}
		parts = append(parts, Passage{Text: p.Text, Caption: p.Caption})
	}
	if vs.Primary != "none" {
		addPart(vs.Primary)
	}
	if vs.Secondary != "none" && vs.Secondary != vs.Primary {
		addPart(vs.Secondary)
	}
	if vs.Trinary != "none" && vs.Trinary != vs.Primary && vs.Trinary != vs.Secondary {
		addPart(vs.Trinary)
	}
	if parts == nil {
		parts = []Passage{}
	}
	sl.Bible = &BibleSlide{Parts: parts,
		Text:        TextStyle{Font: b.TextFont, Color: b.TextColor, Shadow: b.UseShadow, Blur: b.UseBlurShadow, AlignV: b.AlignV, AlignH: b.AlignH, ScaleH: b.ScaleH},
		CaptionFont: b.CaptionFont, CaptionColor: b.CaptionColor, CaptionAlign: b.CaptionAlign, CaptionPosition: b.CaptionPosition, CaptionScaleH: b.CaptionScaleH,
		ScreenUse: b.ScreenUse, ScreenPosition: b.ScreenPosition, AddBK: b.AddBKColorToText, RecBK: b.TextRecBKColor, BkPadding: b.BkPadding, BkOpacity: b.BkOpacity}
	return sl
}

func joinIDs(ids []string) string {
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ","
		}
		out += id
	}
	return out
}

// PreviewTheme renders a sample song slide in the given theme (display 1), for the theme thumbnails.
func (e *Engine) PreviewTheme(id int64) (Slide, error) {
	th, err := e.Themes.Load(id)
	if err != nil {
		return Slide{}, err
	}
	s := th.Song
	bg := e.themeBackground(th)
	style := TextStyle{Font: s.TextFont, Color: s.TextColor, Shadow: s.UseShadow, Blur: s.UseBlurShadow, AlignV: s.AlignV, AlignH: s.AlignH, ScaleH: s.ScaleH}
	return Slide{Kind: "song", Background: bg, Song: &SongSlide{
		Text: "Amazing grace, how sweet the sound\nThat saved a wretch like me", Style: style, ScreenUse: s.ScreenUse, ScreenPosition: s.ScreenPosition,
		AddBK: s.AddBKColorToText, RecBK: s.TextRecBKColor, BkPadding: s.BkPadding, BkOpacity: s.BkOpacity,
		InfoFont: s.InfoFont, InfoColor: s.InfoColor, EndingFont: s.EndingFont, EndingColor: s.EndingColor,
	}}, nil
}

func (e *Engine) songSlide(d int) Slide {
	s := e.Theme.Song
	song := e.curSong
	bg := e.textBackground()
	style := TextStyle{Font: s.TextFont, Color: s.TextColor, Shadow: s.UseShadow, Blur: s.UseBlurShadow, AlignV: s.AlignV, AlignH: s.AlignH, ScaleH: s.ScaleH}
	infoFont, infoColor := s.InfoFont, s.InfoColor
	endFont, endColor := s.EndingFont, s.EndingColor
	if song.UsePrivate {
		style.Font, style.Color, style.AlignV, style.AlignH = song.Font, song.Color, song.AlignV, song.AlignH
		infoFont, infoColor = song.InfoFont, song.InfoColor
		endFont, endColor = song.EndingFont, song.EndingColor
		if song.UseBackground && song.HasBackground {
			if e.songBlob != "" {
				bg = Background{Type: "image", URL: "/api/blob/" + url.PathEscape(e.songBlob)}
			} else if song.ID > 0 {
				bg = Background{Type: "image", URL: fmt.Sprintf("/api/bg/song/%d?v=%d", song.ID, e.bgVer)}
			}
		} else if song.UseBackground {
			bg = Background{Type: "color", Color: "#000000"}
		}
	}
	sl := Slide{Kind: "song", Fade: true, Background: bg}
	if e.state.Cleared || e.state.Index < 0 || e.state.Index >= len(e.state.Items) {
		sl.Kind = "clear"
		return sl
	}
	st := songs.SplitStanza(e.state.Items[e.state.Index].Text, e.state.Index == len(e.state.Items)-1)
	all := make([]string, len(e.state.Items))
	for i, it := range e.state.Items {
		all[i] = songs.SplitStanza(it.Text, i == len(e.state.Items)-1).Text
	}
	sl.Song = &SongSlide{Stanzas: all,Text: st.Text, StanzaTitle: st.Title, Number: song.Number, Tune: song.Tune, IsLast: st.IsLast, Style: style,
		ShowTitle: s.ShowStanzaTitle, ShowKey: s.ShowKey, ShowNumber: s.ShowNumber, InfoFont: infoFont, InfoColor: infoColor, InfoAlign: s.InfoAlign, InfoScaleH: s.InfoScaleH,
		ShowEnding: s.ShowEnding, EndingFont: endFont, EndingColor: endColor, EndingType: s.EndingType, EndingPosition: s.EndingPosition, EndingScaleH: s.EndingScaleH,
		ScreenUse: s.ScreenUse, ScreenPosition: s.ScreenPosition, AddBK: s.AddBKColorToText, RecBK: s.TextRecBKColor, BkPadding: s.BkPadding, BkOpacity: s.BkOpacity,
		WordsBy: song.WordsBy, MusicBy: song.MusicBy}
	return sl
}

func (e *Engine) announceSlide(d int) Slide {
	a := e.Theme.Song.Text // announcements are set like songs
	sl := Slide{Kind: "announce", Fade: true, Background: e.textBackground()}
	if e.state.Cleared || e.state.Index < 0 || e.state.Index >= len(e.state.Items) {
		sl.Kind = "clear"
		return sl
	}
	sl.Announce = &AnnounceSlide{Text: announce.SlideText(e.curAnn.Text, e.state.Index),
		Style: TextStyle{Font: a.TextFont, Color: a.TextColor, Shadow: a.UseShadow, Blur: a.UseBlurShadow, AlignV: a.AlignV, AlignH: a.AlignH}}
	return sl
}

func (e *Engine) pictureSlide(d int) Slide {
	sl := Slide{Kind: "picture", Fade: true, Background: Background{Type: "color", Color: "#000000"}}
	if e.state.Index < 0 || e.state.Index >= len(e.curPics) {
		sl.Kind = "clear"
		return sl
	}
	p := e.curPics[e.state.Index]
	p.ExpandSmall = e.Settings.Slides.ExpandSmall
	p.FitType = e.Settings.Slides.FitType
	sl.Picture = &p
	return sl
}

func (e *Engine) videoSlide(d int) Slide {
	sl := Slide{Kind: "video", Fade: false, Background: Background{Type: "color", Color: "#000000"}}
	sl.Video = &VideoSlide{URL: "/api/media/file?path=" + url.QueryEscape(e.curVideo.Path), Name: e.curVideo.Name, Muted: d > 1 || e.state.Video.Muted}
	return sl
}

// DefaultFont is exposed for the API.
func DefaultFont() qt.Font { return qt.DefaultFont(16) }

// SongBG returns a song's background bytes (schedule saving).
func (e *Engine) SongBG(id int64) ([]byte, error) { return e.Songs.Background(id) }

// SlideImgs returns the three stored variants of a slide image.
func (e *Engine) SlideImgs(id int64) ([]byte, []byte, []byte) {
	f, _ := e.Slides.Image(id, "full")
	s, _ := e.Slides.Image(id, "small")
	p, _ := e.Slides.Image(id, "prev")
	return f, s, p
}

// SetPreviewSlide shows a sample slide on every display (turning the presentation on when it is off) until
// ClearPreviewSlide, or until 30 s pass without another call: the theme editor calls again whenever the sample
// changes and now and then to keep it up.
func (e *Engine) SetPreviewSlide(sl Slide) {
	e.mu.Lock()
	if e.preview == nil {
		e.previewPrev.on, e.previewPrev.showing = e.state.DisplayOn, e.state.Showing
	}
	e.preview = &sl
	e.state.Showing = true
	if e.screenCount() != 1 {
		e.state.DisplayOn = true
	}
	if e.previewTimer != nil {
		e.previewTimer.Stop()
	}
	e.previewTimer = time.AfterFunc(30*time.Second, e.ClearPreviewSlide)
	e.recompute()
	e.mu.Unlock()
	e.publish()
}

// ClearPreviewSlide ends the preview: the displays go back to what they showed before it.
func (e *Engine) ClearPreviewSlide() {
	e.mu.Lock()
	if e.preview == nil {
		e.mu.Unlock()
		return
	}
	e.dropPreviewLocked()
	e.state.DisplayOn, e.state.Showing = e.previewPrev.on, e.previewPrev.showing
	e.recompute()
	e.mu.Unlock()
	e.publish()
}

// dropPreviewLocked forgets the preview without touching the displays (the caller is changing them anyway).
func (e *Engine) dropPreviewLocked() {
	e.preview = nil
	if e.previewTimer != nil {
		e.previewTimer.Stop()
		e.previewTimer = nil
	}
}

// PageReady records that the page of display disp has drawn its first state; false forgets it (the page is gone).
func (e *Engine) PageReady(disp int, ready bool) {
	e.pagesMu.Lock()
	defer e.pagesMu.Unlock()
	if e.pageReady == nil {
		e.pageReady, e.pageSettled = map[int]bool{}, map[int]int64{}
	}
	e.pageReady[disp] = ready
	if !ready {
		delete(e.pageSettled, disp)
	}
}

// IsPageReady reports whether the page of display disp has drawn its first state.
func (e *Engine) IsPageReady(disp int) bool {
	e.pagesMu.Lock()
	defer e.pagesMu.Unlock()
	return e.pageReady[disp]
}

// PageSettled records the newest state version the page of display disp has finished drawing.
func (e *Engine) PageSettled(disp int, version int64) {
	e.pagesMu.Lock()
	defer e.pagesMu.Unlock()
	if e.pageSettled == nil {
		e.pageReady, e.pageSettled = map[int]bool{}, map[int]int64{}
	}
	if version > e.pageSettled[disp] {
		e.pageSettled[disp] = version
	}
}

// SettledVersion is the newest state version the page of display disp has finished drawing.
func (e *Engine) SettledVersion(disp int) int64 {
	e.pagesMu.Lock()
	defer e.pagesMu.Unlock()
	return e.pageSettled[disp]
}
