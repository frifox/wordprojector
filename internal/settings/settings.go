// Package settings reads and writes the Settings table in the same
// "name = value" text format the original program uses, plus a JSON row for
// settings that only this program knows about.
package settings

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"wordprojector/internal/db"
	"wordprojector/internal/slides"
)

// DisplayControls configures the on-screen prev/next/exit buttons used in
// single screen mode.
type DisplayControls struct {
	ButtonSize int     `json:"buttonSize"` // 0=16,1=24,2=32,3=48,4=64,5=96
	AlignV     int     `json:"alignV"`
	AlignH     int     `json:"alignH"`
	Opacity    float64 `json:"opacity"`
}

// General mirrors GeneralSettings.
type General struct {
	DisplayIsOnTop   bool            `json:"displayIsOnTop"`
	DisplayOnStartUp bool            `json:"displayOnStartUp"`
	CurrentThemeID   int64           `json:"currentThemeId"`
	DisplayScreen    int             `json:"displayScreen"`
	DisplayScreen2   int             `json:"displayScreen2"`
	DisplayScreen3   int             `json:"displayScreen3"`
	DisplayScreen4   int             `json:"displayScreen4"`
	// DisplayScreensExtra lists the screens of displays 5, 6, ... (display 5 onwards use the settings of display 1).
	DisplayScreensExtra []int `json:"displayScreensExtra"`
	DisplayControls  DisplayControls `json:"displayControls"`
}

// BibleVersions selects which translations are shown on one display.
// Values are bible ids as strings, "none", or "same" (operator only).
type BibleVersions struct {
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
	Trinary   string `json:"trinary"`
	Operator  string `json:"operator"`
}

// Main mirrors SpSettings (window state, language).
type Main struct {
	UITranslation     string `json:"uiTranslation"`
	IsWindowMaximized bool   `json:"isWindowMaximized"`
}

// WindowGeometry stores the operator window placement.
type WindowGeometry struct {
	X, Y, Width, Height int
}

// Remote configures network access to the backend: loopback only, or every network interface (Public).
type Remote struct {
	Public bool `json:"public"`
	Port   int  `json:"port"`
}

// DefaultPort is the port the backend listens on unless the user chose another.
const DefaultPort = 8777

// ListenAddr is the address to bind: 127.0.0.1, or 0.0.0.0 when public, on the chosen port.
func (r Remote) ListenAddr() string {
	host, port := "127.0.0.1", r.Port
	if r.Public {
		host = "0.0.0.0"
	}
	if port < 1 || port > 65535 {
		port = DefaultPort
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// Transition is how the displays change between slides (items, and starting, stopping, pausing, clearing):
// a cut, or a cross fade of FadeMillis.
type Transition struct {
	Cut        bool `json:"cut"`
	FadeMillis int  `json:"fadeMillis"`
}

// DefaultFadeMillis is the fade time unless the user chose another.
const DefaultFadeMillis = 500

// App holds settings that exist only in this program (stored as JSON).
type App struct {
	Window     WindowGeometry     `json:"window"`
	Splitters  map[string]float64 `json:"splitters"`
	Language   string             `json:"language"`
	// Remote says where the backend listens (applied on the next start).
	Remote      Remote            `json:"remote"`
	Transition  Transition        `json:"transition"`
	// HideDisplayControls turns the on-screen prev/next/exit buttons off in single screen mode (shown by default).
	HideDisplayControls bool `json:"hideDisplayControls"`
	LivePreview bool              `json:"livePreview"`
	UITheme    string             `json:"uiTheme"` // "light" | "dark" | "system"
	// SlideshowOrder is the order of the slide shows in the Photos tab (slide shows have no order of their own).
	SlideshowOrder []int `json:"slideshowOrder"`
	// DefaultSongbook is the songbook pre-selected in the Songs tab on start (0 = all songbooks).
	DefaultSongbook int `json:"defaultSongbook"`
	// SkipUpdateCheck turns off the look for a newer release when the program starts (it is on by default).
	SkipUpdateCheck bool `json:"skipUpdateCheck"`
	// BibleButtonOrder is the order of the Bible version buttons (Bible ids), as the user dragged them.
	BibleButtonOrder []string `json:"bibleButtonOrder"`
	// ModuleSource ("softprojector" or "custom") and ModuleURLs (the address typed for Custom, per kind of book) are
	// what Books > Import from Cloud last used.
	ModuleSource string            `json:"moduleSource"`
	ModuleURLs   map[string]string `json:"moduleUrls"`
	// AutoInstallUpdates installs the latest release by itself when one is found; it is used the next time the
	// program starts.
	AutoInstallUpdates bool `json:"autoInstallUpdates"`
	// Hotkeys maps an action to the key combination the user chose for it; an action that is missing has its default
	// key, one set to "" has none.
	Hotkeys map[string]string `json:"hotkeys"`
}

// Settings is everything stored in the Settings table.
type Settings struct {
	General General           `json:"general"`
	Main    Main              `json:"main"`
	Bible   [4]BibleVersions  `json:"bible"`
	Slides  slides.Settings   `json:"slides"`
	App     App               `json:"app"`
	// raw keeps lines we do not interpret (e.g. Qt splitter states) so they
	// round-trip untouched.
	raw map[string]map[string]string
}

// Default returns the program defaults.
func Default() Settings {
	s := Settings{
		General: General{DisplayScreen2: -1, DisplayScreen3: -1, DisplayScreen4: -1,
			DisplayControls: DisplayControls{ButtonSize: 3, AlignV: 2, AlignH: 1, Opacity: 0.3}},
		Main:   Main{UITranslation: "en"},
		Slides: slides.DefaultSettings(),
		App:    App{Splitters: map[string]float64{}, Language: "en", LivePreview: true, UITheme: "system", Remote: Remote{Port: DefaultPort}, Transition: Transition{FadeMillis: DefaultFadeMillis}},
		raw:    map[string]map[string]string{},
	}
	for i := range s.Bible {
		s.Bible[i] = BibleVersions{Primary: "none", Secondary: "none", Trinary: "none", Operator: "same"}
	}
	return s
}

func parseKV(sets string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(sets, "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return m
}

func atoi(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}

func atob(s string) bool { return strings.TrimSpace(s) == "true" }

// Load reads all settings rows; missing rows get defaults.
func Load(d *db.DB) (Settings, error) {
	s := Default()
	rows, err := d.Query("SELECT type, sets FROM Settings")
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var t, sets sql.NullString
		if err := rows.Scan(&t, &sets); err != nil {
			return s, err
		}
		kv := parseKV(sets.String)
		switch t.String {
		case "general":
			g := &s.General
			if v, ok := kv["displayIsOnTop"]; ok {
				g.DisplayIsOnTop = atob(v)
			}
			if v, ok := kv["displayOnStartUp"]; ok {
				g.DisplayOnStartUp = atob(v)
			}
			g.CurrentThemeID = int64(atoi(kv["currentThemeId"], 0))
			g.DisplayScreen = atoi(kv["displayScreen"], 0)
			g.DisplayScreen2 = atoi(kv["displayScreen2"], -1)
			g.DisplayScreen3 = atoi(kv["displayScreen3"], -1)
			g.DisplayScreen4 = atoi(kv["displayScreen4"], -1)
			g.DisplayScreensExtra = nil
			for _, part := range strings.Split(kv["displayScreensExtra"], ",") {
				if n := atoi(strings.TrimSpace(part), -1); n >= 0 {
					g.DisplayScreensExtra = append(g.DisplayScreensExtra, n)
				}
			}
			g.DisplayControls.ButtonSize = atoi(kv["dcIconSize"], 3)
			if a := strings.Split(kv["dcAlignment"], ","); len(a) == 2 {
				g.DisplayControls.AlignV = atoi(a[0], 2)
				g.DisplayControls.AlignH = atoi(a[1], 1)
			}
			if f, err := strconv.ParseFloat(kv["dcOpacity"], 64); err == nil {
				g.DisplayControls.Opacity = f
			}
		case "spMain":
			if v, ok := kv["uiTranslation"]; ok && v != "" {
				s.Main.UITranslation = v
			}
			s.Main.IsWindowMaximized = atob(kv["isWindowMaximized"])
			s.raw["spMain"] = kv
		case "bible1", "bible2", "bible3", "bible4":
			i := int(t.String[5]-'0') - 1
			b := &s.Bible[i]
			if v, ok := kv["primary"]; ok && v != "" {
				b.Primary = v
			}
			if v, ok := kv["secondary"]; ok && v != "" {
				b.Secondary = v
			}
			if v, ok := kv["trinary"]; ok && v != "" {
				b.Trinary = v
			}
			if v, ok := kv["operator"]; ok && v != "" {
				b.Operator = v
			}
		case "pix":
			if v, ok := kv["expandSmall"]; ok {
				s.Slides.ExpandSmall = atob(v)
			}
			s.Slides.FitType = atoi(kv["fitType"], 0)
			if v, ok := kv["resize"]; ok {
				s.Slides.Resize = atob(v)
			}
			s.Slides.BoundType = atoi(kv["boundType"], 2)
			s.Slides.BoundWidth = atoi(kv["boundWidth"], 1280)
		case "wordprojector":
			var a App
			if err := json.Unmarshal([]byte(sets.String), &a); err == nil {
				if a.Splitters == nil {
					a.Splitters = map[string]float64{}
				}
				if a.Language == "" {
					a.Language = "en"
				}
				if a.UITheme == "" {
					a.UITheme = "system"
				}
				if a.Remote.Port == 0 {
					a.Remote.Port = DefaultPort
				}
				if a.Transition.FadeMillis < 50 || a.Transition.FadeMillis > 5000 {
					a.Transition.FadeMillis = DefaultFadeMillis
				}
				s.App = a
			}
		}
	}
	return s, rows.Err()
}

func b2s(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// Save writes every settings row (INSERT OR REPLACE semantics by type).
func Save(d *db.DB, s Settings) error {
	g := s.General
	gset := fmt.Sprintf("displayIsOnTop = %s\ndisplayOnStartUp = %s\ncurrentThemeId = %d\ndisplayScreen = %d\ndisplayScreen2 = %d\ndisplayScreen3 = %d\ndisplayScreen4 = %d\ndisplayScreensExtra = %s\ndcIconSize = %d\ndcAlignment = %d,%d\ndcOpacity = %s",
		b2s(g.DisplayIsOnTop), b2s(g.DisplayOnStartUp), g.CurrentThemeID, g.DisplayScreen, g.DisplayScreen2, g.DisplayScreen3, g.DisplayScreen4, joinInts(g.DisplayScreensExtra),
		g.DisplayControls.ButtonSize, g.DisplayControls.AlignV, g.DisplayControls.AlignH, strconv.FormatFloat(g.DisplayControls.Opacity, 'f', -1, 64))
	raw := s.raw["spMain"]
	if raw == nil {
		raw = map[string]string{}
	}
	spset := fmt.Sprintf("spSplitter = %s\nbibleHiddenSplitter = %s\nbibleShowSplitter = %s\nsongSplitter = %s\nuiTranslation = %s\nisWindowMaximized = %s",
		raw["spSplitter"], raw["bibleHiddenSplitter"], raw["bibleShowSplitter"], raw["songSplitter"], s.Main.UITranslation, b2s(s.Main.IsWindowMaximized))
	bsets := make([]string, 4)
	for i, b := range s.Bible {
		if i == 0 {
			bsets[i] = fmt.Sprintf("primary = %s\nsecondary = %s\ntrinary = %s\noperator = %s", b.Primary, b.Secondary, b.Trinary, b.Operator)
		} else {
			bsets[i] = fmt.Sprintf("primary = %s\nsecondary = %s\ntrinary = %s", b.Primary, b.Secondary, b.Trinary)
		}
	}
	p := s.Slides
	pset := fmt.Sprintf("expandSmall = %s\nfitType = %d\nresize = %s\nboundType = %d\nboundWidth = %d", b2s(p.ExpandSmall), p.FitType, b2s(p.Resize), p.BoundType, p.BoundWidth)
	appJSON, _ := json.Marshal(s.App)
	rowsToWrite := map[string]string{"general": gset, "spMain": spset, "bible1": bsets[0], "bible2": bsets[1], "bible3": bsets[2], "bible4": bsets[3], "pix": pset, "wordprojector": string(appJSON)}
	return d.Tx(func(tx *sql.Tx) error {
		for t, v := range rowsToWrite {
			res, err := tx.Exec("UPDATE Settings SET sets = ? WHERE type = ?", v, t)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				if _, err := tx.Exec("INSERT INTO Settings (type, sets) VALUES (?,?)", t, v); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func joinInts(v []int) string {
	parts := make([]string, 0, len(v))
	for _, n := range v {
		parts = append(parts, strconv.Itoa(n))
	}
	return strings.Join(parts, ",")
}
