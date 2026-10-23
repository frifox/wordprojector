// Package announce holds announcements and their slide splitting.
package announce

import (
	"database/sql"
	"fmt"
	"strings"

	"wordprojector/internal/db"
	"wordprojector/internal/qt"
)

var titleWords = []string{"Announce", "Slide", "Объявление", "Слайд", "Оголошення", "Ankündigung", "Oznámení", "Snímek"}

// IsTitle reports whether a line starts a new announcement slide.
func IsTitle(line string) bool {
	for _, w := range titleWords {
		if strings.HasPrefix(line, w) {
			return true
		}
	}
	return false
}

// TitleWords returns the recognised slide title prefixes.
func TitleWords() []string { return append([]string{}, titleWords...) }

// Announcement is one announcement record.
type Announcement struct {
	ID             int64   `json:"id"`
	Title          string  `json:"title"`
	Text           string  `json:"text"`
	UsePrivate     bool    `json:"usePrivate"`
	UseAuto        bool    `json:"useAuto"`
	Loop           bool    `json:"loop"`
	SlideTime      int     `json:"slideTime"`
	UseBackground  bool    `json:"useBackground"`
	BackgroundPath string  `json:"backgroundPath"`
	Font           qt.Font `json:"font"`
	Color          string  `json:"color"`
	AlignV         int     `json:"alignV"`
	AlignH         int     `json:"alignH"`
}

// New returns an announcement with default values.
func New() Announcement {
	return Announcement{UseAuto: true, Loop: true, SlideTime: 10, Font: qt.DefaultFont(28), Color: "#ffffff", AlignV: 0, AlignH: 1}
}

// SlideList splits the text into slides (port of Announcement::getAnnounceList).
func SlideList(text string) []string {
	var out []string
	var part strings.Builder
	for _, s := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if IsTitle(s) {
			if strings.TrimSpace(part.String()) != "" {
				out = append(out, strings.TrimSpace(part.String()))
			}
			part.Reset()
		}
		part.WriteString(s)
		part.WriteString("\n")
	}
	if strings.TrimSpace(part.String()) != "" {
		out = append(out, strings.TrimSpace(part.String()))
	}
	return out
}

// SlideText returns the projected text of slide i (title line removed).
func SlideText(text string, i int) string {
	list := SlideList(text)
	if i < 0 || i >= len(list) {
		return ""
	}
	lines := strings.Split(list[i], "\n")
	if len(lines) > 0 && IsTitle(lines[0]) {
		lines = lines[1:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// Repo is the announcement repository.
type Repo struct{ DB *db.DB }

func scan(sc interface{ Scan(...any) error }) (Announcement, error) {
	a := New()
	var title, text, backPath, font, color, align sql.NullString
	var usePrivate, useAuto, loop any
	var slideTime sql.NullInt64
	var useBack any
	if err := sc.Scan(&a.ID, &title, &text, &usePrivate, &useAuto, &loop, &slideTime, &useBack, &backPath, &font, &color, &align); err != nil {
		return a, err
	}
	a.Title = strings.TrimSpace(title.String)
	a.Text = strings.TrimSpace(text.String)
	a.UsePrivate = qt.ParseBool(usePrivate)
	a.UseAuto = qt.ParseBool(useAuto)
	a.Loop = qt.ParseBool(loop)
	if slideTime.Valid {
		a.SlideTime = int(slideTime.Int64)
	}
	a.UseBackground = qt.ParseBool(useBack)
	a.BackgroundPath = strings.TrimSpace(backPath.String)
	if f, ok := qt.ParseFont(font.String); ok {
		a.Font = f
	}
	if c, ok := qt.ParseInt(color.String); ok && color.String != "" {
		a.Color = qt.RGBToHex(c)
	}
	if parts := strings.Split(align.String, ","); len(parts) == 2 {
		if v, ok := qt.ParseInt(parts[0]); ok {
			a.AlignV = int(v)
		}
		if v, ok := qt.ParseInt(parts[1]); ok {
			a.AlignH = int(v)
		}
	}
	return a, nil
}

const cols = "id, title, text, usePrivate, useAuto, loop, slideTime, useBackground, backgoundPath, font, color, alignment"

func (r *Repo) List() ([]Announcement, error) {
	rows, err := r.DB.Query("SELECT " + cols + " FROM Announcements ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Announcement{}
	for rows.Next() {
		a, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repo) Get(id int64) (Announcement, error) {
	return scan(r.DB.QueryRow("SELECT "+cols+" FROM Announcements WHERE id = ?", id))
}

func args(a Announcement) []any {
	return []any{strings.TrimSpace(a.Title), a.Text, a.UsePrivate, a.UseAuto, a.Loop, a.SlideTime, a.UseBackground,
		a.BackgroundPath, a.Font.String(), fmt.Sprint(qt.HexToRGB(a.Color)), fmt.Sprintf("%d,%d", a.AlignV, a.AlignH)}
}

func (r *Repo) Insert(a Announcement) (int64, error) {
	var id int64
	err := r.DB.Tx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`INSERT INTO Announcements (title, text, usePrivate, useAuto, loop, slideTime, useBackground, backgoundPath, font, color, alignment)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`, args(a)...)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	return id, err
}

func (r *Repo) Update(a Announcement) error {
	return r.DB.Tx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE Announcements SET title=?, text=?, usePrivate=?, useAuto=?, loop=?, slideTime=?, useBackground=?,
			backgoundPath=?, font=?, color=?, alignment=? WHERE id = ?`, append(args(a), a.ID)...)
		return err
	})
}

func (r *Repo) Delete(id int64) error {
	return r.DB.Tx(func(tx *sql.Tx) error {
		_, err := tx.Exec("DELETE FROM Announcements WHERE id = ?", id)
		return err
	})
}
