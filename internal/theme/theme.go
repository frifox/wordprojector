// Package theme loads and saves projection themes using the original
// ThemePassive/ThemeBible/ThemeSong/ThemeAnnounce tables (one row per display).
package theme

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"wordprojector/internal/db"
	"wordprojector/internal/qt"
)

// Displays is the number of display screens a theme describes.
const Displays = 4

// Info is a theme list entry.
type Info struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Builtin bool   `json:"builtin"` // Light and Dark ship with the program and can't be deleted
}

// Names of the themes that ship with the program.
const (
	LightName = "Light"
	DarkName  = "Dark"
)

// IsBuiltinName reports whether a theme name belongs to a built-in theme.
func IsBuiltinName(name string) bool { return name == LightName || name == DarkName }

// Text holds the settings shared by Bible and song slides.
type Text struct {
	UseShadow     bool    `json:"useShadow"`
	UseBlurShadow bool    `json:"useBlurShadow"`
	TextFont      qt.Font `json:"textFont"`
	TextColor     string  `json:"textColor"`
	AlignV        int     `json:"alignV"`
	AlignH        int     `json:"alignH"`
	ScaleH        int     `json:"scaleH"` // width of the characters, percent of normal
}

// Passive is the idle screen configuration of one display (Settings > Background; not part of a theme).
type Passive struct {
	BackgroundFit  int    `json:"backgroundFit"` // 0 scale (stretch), 1 fill (cover), 2 fit (contain)
	UseBackground  bool   `json:"useBackground"`
	BackgroundName string `json:"backgroundName"`
	HasBackground  bool   `json:"hasBackground"`
	UseDisp1       bool   `json:"useDisp1"`
}

// Bible is the bible slide configuration.
type Bible struct {
	Text
	CaptionFont      qt.Font `json:"captionFont"`
	CaptionColor     string  `json:"captionColor"`
	CaptionAlign     int     `json:"captionAlign"`
	CaptionPosition  int     `json:"captionPosition"`
	CaptionScaleH    int     `json:"captionScaleH"` // width of the caption characters, percent of normal
	ScreenUse        int     `json:"screenUse"`
	ScreenPosition   int     `json:"screenPosition"`
	AddBKColorToText bool    `json:"addBkColorToText"`
	TextRecBKColor   string  `json:"textRecBkColor"`
	BkPadding        int     `json:"bkPadding"` // space above and below the text inside its background color
	BkOpacity        int     `json:"bkOpacity"` // how opaque that color is, percent
}

// Song is the song slide configuration.
type Song struct {
	Text
	ShowStanzaTitle  bool    `json:"showStanzaTitle"`
	ShowKey          bool    `json:"showKey"`
	ShowNumber       bool    `json:"showNumber"`
	InfoColor        string  `json:"infoColor"`
	InfoFont         qt.Font `json:"infoFont"`
	InfoAlign        int     `json:"infoAlign"`
	InfoScaleH       int     `json:"infoScaleH"`
	ShowEnding       bool    `json:"showEnding"`
	EndingColor      string  `json:"endingColor"`
	EndingFont       qt.Font `json:"endingFont"`
	EndingType       int     `json:"endingType"`
	EndingPosition   int     `json:"endingPosition"`
	EndingScaleH     int     `json:"endingScaleH"`
	ScreenUse        int     `json:"screenUse"`
	ScreenPosition   int     `json:"screenPosition"`
	AddBKColorToText bool    `json:"addBkColorToText"`
	TextRecBKColor   string  `json:"textRecBkColor"`
	BkPadding        int     `json:"bkPadding"` // space above and below the text inside its background color
	BkOpacity        int     `json:"bkOpacity"` // how opaque that color is, percent
}

// Theme is the look of Bible, song and announcement slides, the same on every display. The database keeps one row
// per display (the original program's layout), all of them written alike; the first one is read.
type Theme struct {
	Info
	// BackgroundColor is the color of the screen behind the text.
	BackgroundColor string `json:"backgroundColor"`
	Bible           Bible  `json:"bible"`
	Song            Song   `json:"song"`
}

// BackgroundSet is the picture shown on the display screens when nothing is projected (Settings > Background).
// It belongs to the program, not to a theme.
type BackgroundSet struct {
	ID      int64             `json:"id"`
	Passive [Displays]Passive `json:"passive"`
}

func defaultText(size float64) Text {
	return Text{UseShadow: true, TextFont: qt.DefaultFont(size), TextColor: "#ffffff", ScaleH: 100}
}

// Default returns the program defaults (constructors in settings.cpp).
func Default() Theme {
	t := Theme{Info: Info{Name: "Default"}, BackgroundColor: "#000000"}
	t.Bible = Bible{Text: defaultText(16), CaptionFont: qt.DefaultFont(15), CaptionColor: "#ffffff", CaptionAlign: 2, CaptionPosition: 1, CaptionScaleH: 100,
		ScreenUse: 100, ScreenPosition: 0, TextRecBKColor: "#000000", BkPadding: 10, BkOpacity: 100}
	t.Bible.AlignV, t.Bible.AlignH = 0, 0
	t.Song = Song{Text: defaultText(16), InfoColor: "#ffffff", InfoFont: qt.DefaultFont(15), InfoAlign: 0, InfoScaleH: 100, EndingScaleH: 100, ShowEnding: true,
		EndingColor: "#ffffff", EndingFont: qt.DefaultFont(16), EndingType: 0, EndingPosition: 0, ScreenUse: 100,
		TextRecBKColor: "#000000", BkPadding: 10, BkOpacity: 100}
	t.Song.AlignV, t.Song.AlignH = 1, 1
	return t
}

// Repo is the theme repository. With Dir set, the list of background images (Settings > Background) lives as
// files there.
type Repo struct {
	DB  *db.DB
	Dir string
}

func (r *Repo) List() ([]Info, error) {
	rows, err := r.DB.Query("SELECT id, name FROM Themes WHERE COALESCE(comment, '') <> ? ORDER BY id", BackgroundMarker)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Info{}
	for rows.Next() {
		var t Info
		var n sql.NullString
		if err := rows.Scan(&t.ID, &n); err != nil {
			return nil, err
		}
		t.Name = n.String
		t.Builtin = IsBuiltinName(t.Name)
		out = append(out, t)
	}
	return out, rows.Err()
}

// Load reads a theme. If id does not exist the first theme is used; if there
// are no themes a default one is created.
func (r *Repo) Load(id int64) (Theme, error) {
	t := Default()
	var n sql.NullString
	err := r.DB.QueryRow("SELECT id, name FROM Themes WHERE id = ? AND COALESCE(comment, '') <> ?", id, BackgroundMarker).Scan(&t.ID, &n)
	if err != nil {
		err = r.DB.QueryRow("SELECT id, name FROM Themes WHERE COALESCE(comment, '') <> ? ORDER BY id LIMIT 1", BackgroundMarker).Scan(&t.ID, &n)
		if err != nil {
			id, err := r.Insert(t)
			if err != nil {
				return t, err
			}
			t.ID = id
			return t, nil
		}
	}
	t.Name = n.String
	t.Builtin = IsBuiltinName(t.Name)
	var bg sql.NullString
	if r.DB.QueryRow("SELECT bg_color FROM Themes WHERE id = ?", t.ID).Scan(&bg) == nil && len(bg.String) == 7 && bg.String[0] == '#' {
		t.BackgroundColor = bg.String
	}
	r.loadBible(t.ID, 1, &t.Bible)
	r.loadSong(t.ID, 1, &t.Song)
	// the text effects are one setting for both (older themes could differ)
	t.Song.UseShadow, t.Song.UseBlurShadow = t.Bible.UseShadow, t.Bible.UseBlurShadow
	return t, nil
}

// LoadBackground reads the settings of the passive screen's background.
func (r *Repo) LoadBackground(id int64) BackgroundSet {
	b := BackgroundSet{ID: id}
	for d := 1; d <= Displays; d++ {
		b.Passive[d-1] = Passive{UseDisp1: d > 1}
		r.loadPassive(id, d, &b.Passive[d-1])
	}
	return b
}

// SaveBackground saves the settings of the passive screen's background (the picture itself is kept).
func (r *Repo) SaveBackground(b BackgroundSet) error {
	return r.DB.Tx(func(tx *sql.Tx) error {
		for d := 1; d <= Displays; d++ {
			p := b.Passive[d-1]
			if err := upsert(tx, "ThemePassive", b.ID, d); err != nil {
				return err
			}
			if _, err := tx.Exec("UPDATE ThemePassive SET use_background=?, background_name=?, use_disp_1=?, image_scale=? WHERE theme_id=? AND disp=?",
				p.UseBackground, p.BackgroundName, p.UseDisp1, p.BackgroundFit, b.ID, d); err != nil {
				return err
			}
		}
		return nil
	})
}

// colorOrBlack keeps a "#rrggbb" color and makes anything else black.
func colorOrBlack(c string) string {
	if len(c) == 7 && c[0] == '#' {
		return c
	}
	return "#000000"
}

func colorOf(v sql.NullInt64, def string) string {
	if !v.Valid {
		return def
	}
	return qt.RGBToHex(v.Int64)
}

func fontOf(s sql.NullString, def qt.Font) qt.Font {
	if f, ok := qt.ParseFont(s.String); ok {
		return f
	}
	return def
}

func (r *Repo) loadPassive(id int64, d int, p *Passive) {
	var useBack, useDisp1 any
	var name sql.NullString
	var hasBack any
	var fit int
	err := r.DB.QueryRow("SELECT use_background, background_name, (background IS NOT NULL AND length(background)>0), use_disp_1, image_scale FROM ThemePassive WHERE theme_id = ? AND disp = ?", id, d).
		Scan(&useBack, &name, &hasBack, &useDisp1, &fit)
	if err != nil {
		return
	}
	p.BackgroundFit = fit
	p.UseBackground = qt.ParseBool(useBack)
	p.BackgroundName = name.String
	p.HasBackground = qt.ParseBool(hasBack)
	p.UseDisp1 = qt.ParseBool(useDisp1)
}

func (r *Repo) loadBible(id int64, d int, b *Bible) {
	var useShadow, useBlur, addBK any
	var textFont, capFont sql.NullString
	var textColor, capColor, recBK, pad, opa, scale sql.NullInt64
	var alignV, alignH, capAlign, capPos, screenUse, screenPos sql.NullInt64
	err := r.DB.QueryRow(`SELECT use_shadow, use_blur_shadow, text_font, text_color, text_align_v, text_align_h, caption_font, caption_color,
		caption_align, caption_position, screen_use, screen_position, add_background_color_to_text, text_rec_background_color, bk_padding, bk_opacity, text_scale_h
		FROM ThemeBible WHERE theme_id = ? AND disp = ?`, id, d).
		Scan(&useShadow, &useBlur, &textFont, &textColor, &alignV, &alignH, &capFont, &capColor, &capAlign, &capPos,
			&screenUse, &screenPos, &addBK, &recBK, &pad, &opa, &scale)
	if err != nil {
		return
	}
	b.UseShadow, b.UseBlurShadow = qt.ParseBool(useShadow), qt.ParseBool(useBlur)
	b.TextFont = fontOf(textFont, b.TextFont)
	b.TextColor = colorOf(textColor, b.TextColor)
	b.AlignV, b.AlignH = int(alignV.Int64), int(alignH.Int64)
	b.CaptionFont = fontOf(capFont, b.CaptionFont)
	b.CaptionColor = colorOf(capColor, b.CaptionColor)
	b.CaptionAlign, b.CaptionPosition = int(capAlign.Int64), int(capPos.Int64)
	if screenUse.Valid && screenUse.Int64 > 0 {
		b.ScreenUse = int(screenUse.Int64)
	}
	b.ScreenPosition = int(screenPos.Int64)
	b.CaptionScaleH = r.scaleOf("ThemeBible", "caption_scale_h", id, d)
	b.AddBKColorToText = qt.ParseBool(addBK)
	b.TextRecBKColor = colorOf(recBK, b.TextRecBKColor)
	if pad.Valid {
		b.BkPadding = int(pad.Int64)
	}
	if opa.Valid {
		b.BkOpacity = int(opa.Int64)
	}
	if scale.Valid && scale.Int64 > 0 {
		b.ScaleH = int(scale.Int64)
	}
}

func (r *Repo) loadSong(id int64, d int, s *Song) {
	var useShadow, useBlur, showTitle, showKey, showNum, showEnding, addBK any
	var textFont, infoFont, endFont sql.NullString
	var textColor, infoColor, endColor, recBK, pad, opa, scale sql.NullInt64
	var infoAlign, endType, endPos, alignV, alignH, screenUse, screenPos sql.NullInt64
	err := r.DB.QueryRow(`SELECT use_shadow, use_blur_shadow, show_stanza_title, show_key, show_number, info_color, info_font, info_align,
		show_song_ending, ending_color, ending_font, ending_type, ending_position, text_font, text_color, text_align_v, text_align_h,
		screen_use, screen_position, add_background_color_to_text, text_rec_background_color, bk_padding, bk_opacity, text_scale_h FROM ThemeSong WHERE theme_id = ? AND disp = ?`, id, d).
		Scan(&useShadow, &useBlur, &showTitle, &showKey, &showNum, &infoColor, &infoFont, &infoAlign, &showEnding, &endColor, &endFont,
			&endType, &endPos, &textFont, &textColor, &alignV, &alignH, &screenUse, &screenPos, &addBK, &recBK, &pad, &opa, &scale)
	if err != nil {
		return
	}
	s.UseShadow, s.UseBlurShadow = qt.ParseBool(useShadow), qt.ParseBool(useBlur)
	s.ShowStanzaTitle, s.ShowKey, s.ShowNumber, s.ShowEnding = qt.ParseBool(showTitle), qt.ParseBool(showKey), qt.ParseBool(showNum), qt.ParseBool(showEnding)
	s.InfoColor, s.InfoFont, s.InfoAlign = colorOf(infoColor, s.InfoColor), fontOf(infoFont, s.InfoFont), int(infoAlign.Int64)
	s.EndingColor, s.EndingFont, s.EndingType, s.EndingPosition = colorOf(endColor, s.EndingColor), fontOf(endFont, s.EndingFont), int(endType.Int64), int(endPos.Int64)
	if s.EndingType > 6 { // "Song Copyright Info" is no longer offered
		s.EndingType = 0
	}
	s.TextFont, s.TextColor = fontOf(textFont, s.TextFont), colorOf(textColor, s.TextColor)
	s.AlignV, s.AlignH = int(alignV.Int64), int(alignH.Int64)
	if screenUse.Valid && screenUse.Int64 > 0 {
		s.ScreenUse = int(screenUse.Int64)
	}
	s.ScreenPosition = int(screenPos.Int64)
	s.InfoScaleH = r.scaleOf("ThemeSong", "info_scale_h", id, d)
	s.EndingScaleH = r.scaleOf("ThemeSong", "ending_scale_h", id, d)
	s.AddBKColorToText = qt.ParseBool(addBK)
	s.TextRecBKColor = colorOf(recBK, s.TextRecBKColor)
	if pad.Valid {
		s.BkPadding = int(pad.Int64)
	}
	if opa.Valid {
		s.BkOpacity = int(opa.Int64)
	}
	if scale.Valid && scale.Int64 > 0 {
		s.ScaleH = int(scale.Int64)
	}
}

// scaleOf reads a character width (percent) column; anything unusable means normal width.
func (r *Repo) scaleOf(table, col string, id int64, d int) int {
	var n sql.NullInt64
	if r.DB.QueryRow("SELECT "+col+" FROM "+table+" WHERE theme_id = ? AND disp = ?", id, d).Scan(&n) == nil && n.Valid && n.Int64 > 0 {
		return int(n.Int64)
	}
	return 100
}

// Insert creates a new theme (with rows for all displays) and returns its id.
func (r *Repo) Insert(t Theme) (int64, error) {
	var id int64
	err := r.DB.Tx(func(tx *sql.Tx) error {
		res, err := tx.Exec("INSERT INTO Themes (name, comment, bg_color) VALUES (?,?,?)", t.Name, "", colorOrBlack(t.BackgroundColor))
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		if err != nil {
			return err
		}
		return writeRows(tx, id, t)
	})
	return id, err
}

// writeRows stores the Bible and song looks in the rows of every display (all alike; the other displays are marked
// as "use display 1", as the original program does) and makes sure the idle screen rows exist.
func writeRows(tx *sql.Tx, id int64, t Theme) error {
	b, s := t.Bible, t.Song
	s.UseShadow, s.UseBlurShadow = b.UseShadow, b.UseBlurShadow
	for d := 1; d <= Displays; d++ {
		for _, table := range []string{"ThemePassive", "ThemeBible", "ThemeSong", "ThemeAnnounce"} {
			if err := upsert(tx, table, id, d); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`UPDATE ThemeBible SET use_shadow=?, use_blur_shadow=?, text_font=?, text_color=?, text_align_v=?, text_align_h=?,
			caption_font=?, caption_color=?, caption_align=?, caption_position=?, screen_use=?, screen_position=?, use_disp_1=?,
			add_background_color_to_text=?, text_rec_background_color=?, bk_padding=?, bk_opacity=?, text_scale_h=? WHERE theme_id=? AND disp=?`,
			b.UseShadow, b.UseBlurShadow, b.TextFont.String(), qt.HexToRGB(b.TextColor), b.AlignV, b.AlignH,
			b.CaptionFont.String(), qt.HexToRGB(b.CaptionColor), b.CaptionAlign, b.CaptionPosition, b.ScreenUse, b.ScreenPosition, d > 1,
			b.AddBKColorToText, qt.HexToRGB(b.TextRecBKColor), b.BkPadding, b.BkOpacity, b.ScaleH, id, d); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE ThemeSong SET use_shadow=?, use_blur_shadow=?, show_stanza_title=?, show_key=?, show_number=?, info_color=?,
			info_font=?, info_align=?, show_song_ending=?, ending_color=?, ending_font=?, ending_type=?, ending_position=?, text_font=?, text_color=?,
			text_align_v=?, text_align_h=?, screen_use=?, screen_position=?, use_disp_1=?, add_background_color_to_text=?, text_rec_background_color=?, bk_padding=?, bk_opacity=?, text_scale_h=?
			WHERE theme_id=? AND disp=?`,
			s.UseShadow, s.UseBlurShadow, s.ShowStanzaTitle, s.ShowKey, s.ShowNumber, qt.HexToRGB(s.InfoColor), s.InfoFont.String(), s.InfoAlign,
			s.ShowEnding, qt.HexToRGB(s.EndingColor), s.EndingFont.String(), s.EndingType, s.EndingPosition, s.TextFont.String(), qt.HexToRGB(s.TextColor),
			s.AlignV, s.AlignH, s.ScreenUse, s.ScreenPosition, d > 1, s.AddBKColorToText, qt.HexToRGB(s.TextRecBKColor), s.BkPadding, s.BkOpacity, s.ScaleH, id, d); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE ThemeBible SET caption_scale_h=? WHERE theme_id=? AND disp=?", b.CaptionScaleH, id, d); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE ThemeSong SET info_scale_h=?, ending_scale_h=? WHERE theme_id=? AND disp=?", s.InfoScaleH, s.EndingScaleH, id, d); err != nil {
			return err
		}
	}
	return nil
}

// Update saves a theme.
func (r *Repo) Update(t Theme) error {
	return r.DB.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec("UPDATE Themes SET name = ?, bg_color = ? WHERE id = ?", t.Name, colorOrBlack(t.BackgroundColor), t.ID); err != nil {
			return err
		}
		return writeRows(tx, t.ID, t)
	})
}

// upsert makes sure a per-display row exists (older databases only had rows for displays 1-2).
func upsert(tx *sql.Tx, table string, id int64, d int) error {
	var n int
	if err := tx.QueryRow(fmt.Sprintf("SELECT count(*) FROM %s WHERE theme_id = ? AND disp = ?", table), id, d).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := tx.Exec(fmt.Sprintf("INSERT INTO %s (theme_id, disp) VALUES (?,?)", table), id, d)
	return err
}

func (r *Repo) Rename(id int64, name string) error {
	_, err := r.DB.Exec("UPDATE Themes SET name = ? WHERE id = ?", name, id)
	return err
}

func (r *Repo) Delete(id int64) error {
	files := r.imageFiles("theme_id = ?", id)
	err := r.DB.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM ThemeBackgrounds WHERE theme_id = ?", id); err != nil {
			return err
		}
		for _, t := range []string{"ThemePassive", "ThemeBible", "ThemeSong", "ThemeAnnounce"} {
			if _, err := tx.Exec("DELETE FROM "+t+" WHERE theme_id = ?", id); err != nil {
				return err
			}
		}
		_, err := tx.Exec("DELETE FROM Themes WHERE id = ?", id)
		return err
	})
	if err == nil {
		for _, f := range files {
			r.removeFile(f)
		}
	}
	return err
}

// only the idle screen has pictures; themes have a background color
var kindTable = map[string]string{"passive": "ThemePassive"}

// Background returns the stored background image for a kind/display.
func (r *Repo) Background(id int64, kind string, disp int) ([]byte, string, error) {
	table, ok := kindTable[strings.ToLower(kind)]
	if !ok {
		return nil, "", fmt.Errorf("unknown kind %q", kind)
	}
	var b []byte
	var name sql.NullString
	err := r.DB.QueryRow("SELECT background, background_name FROM "+table+" WHERE theme_id = ? AND disp = ?", id, disp).Scan(&b, &name)
	return b, name.String, err
}

// SetBackground stores a background image (JPEG bytes) for a kind/display.
func (r *Repo) SetBackground(id int64, kind string, disp int, name string, jpeg []byte) error {
	table, ok := kindTable[strings.ToLower(kind)]
	if !ok {
		return fmt.Errorf("unknown kind %q", kind)
	}
	return r.DB.Tx(func(tx *sql.Tx) error {
		if err := upsert(tx, table, id, disp); err != nil {
			return err
		}
		_, err := tx.Exec("UPDATE "+table+" SET background = ?, background_name = ?, use_background = ? WHERE theme_id = ? AND disp = ?", jpeg, name, len(jpeg) > 0, id, disp)
		return err
	})
}

// lightTheme is the built-in light theme: dark text on a white background.
func lightTheme() Theme {
	t := Default()
	t.Name, t.BackgroundColor = LightName, "#ffffff"
	const ink = "#1a1a1a"
	t.Bible.TextColor, t.Bible.CaptionColor = ink, ink
	t.Bible.UseShadow, t.Bible.UseBlurShadow = false, false
	t.Song.TextColor, t.Song.InfoColor, t.Song.EndingColor = ink, ink, ink
	t.Song.UseShadow, t.Song.UseBlurShadow = false, false
	return t
}

// darkTheme is the built-in dark theme: white text on black (the program default).
func darkTheme() Theme {
	t := Default()
	t.Name = DarkName
	return t
}

// EnsureBuiltins makes sure the Light and Dark themes exist. A lone untouched
// "Default" theme (what older versions created) becomes the Dark theme.
func (r *Repo) EnsureBuiltins() error {
	list, err := r.List()
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, t := range list {
		have[t.Name] = true
	}
	if !have[DarkName] {
		for _, t := range list {
			var c sql.NullString
			if t.Name == "Default" && r.DB.QueryRow("SELECT comment FROM Themes WHERE id = ?", t.ID).Scan(&c) == nil && c.String == "Default SoftProjector Theme" {
				if err := r.Rename(t.ID, DarkName); err != nil {
					return err
				}
				have[DarkName] = true
				break
			}
		}
	}
	if !have[DarkName] {
		if _, err := r.Insert(darkTheme()); err != nil {
			return err
		}
	}
	if !have[LightName] {
		if _, err := r.Insert(lightTheme()); err != nil {
			return err
		}
	}
	return nil
}

// Tidy removes what themes no longer use: pictures behind Bible, song and announcement text, and the idle screen
// pictures of themes (the idle screen belongs to the program). A theme's background is a color; the Light theme's
// white picture becomes the color white.
func (r *Repo) Tidy() error {
	bg := r.DB.QueryRow
	var keep int64
	_ = bg("SELECT COALESCE((SELECT id FROM Themes WHERE comment = ? LIMIT 1), 0)", BackgroundMarker).Scan(&keep)
	files := r.imageFiles("kind <> 'passive' OR theme_id <> ?", keep)
	err := r.DB.Tx(func(tx *sql.Tx) error {
		for _, q := range []string{
			"DELETE FROM ThemeBackgrounds WHERE kind <> 'passive' OR theme_id <> " + strconv.FormatInt(keep, 10),
			"UPDATE ThemeBible SET use_background = 0, background_name = '', background = x''",
			"UPDATE ThemeSong SET use_background = 0, background_name = '', background = x''",
			"UPDATE ThemeAnnounce SET use_background = 0, background_name = '', background = x''",
			"UPDATE ThemePassive SET use_background = 0, background_name = '', background = x'' WHERE theme_id <> " + strconv.FormatInt(keep, 10),
			"UPDATE Themes SET bg_color = '#ffffff' WHERE name = '" + LightName + "' AND COALESCE(bg_color, '#000000') = '#000000' AND COALESCE(comment, '') <> '" + BackgroundMarker + "'",
			"UPDATE Themes SET comment = '' WHERE COALESCE(comment, '') <> '" + BackgroundMarker + "'",
		} {
			if _, err := tx.Exec(q); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		for _, f := range files {
			r.removeFile(f)
		}
	}
	return err
}
