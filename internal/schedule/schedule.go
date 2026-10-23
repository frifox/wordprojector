// Package schedule keeps the service schedule (playlist) and reads/writes
// .spsc schedule files in the original SQLite layout.
package schedule

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"

	"wordprojector/internal/announce"
	"wordprojector/internal/db"
	"wordprojector/internal/media"
	"wordprojector/internal/qt"
	"wordprojector/internal/slides"
	"wordprojector/internal/songs"
)

// BibleRef is a bible history / schedule entry.
type BibleRef struct {
	VerseIDs    string `json:"verseIds"`
	Caption     string `json:"caption"`
	CaptionLong string `json:"captionLong"`
}

// Item is one schedule entry. Exactly one of the payload fields is set.
type Item struct {
	Type      string                 `json:"type"` // bible, song, slideshow, media, announce
	Name      string                 `json:"name"`
	Scid      int64                  `json:"scid"` // row id in the schedule file, -1 when unsaved
	Bible     *BibleRef              `json:"bible,omitempty"`
	Song      *songs.Song            `json:"song,omitempty"`
	Slideshow *slides.ShowWithSlides `json:"slideshow,omitempty"`
	Media     *media.Item            `json:"media,omitempty"`
	Announce  *announce.Announcement `json:"announce,omitempty"`
}

// Schedule is the in-memory schedule plus the images that came from a file.
type Schedule struct {
	mu       sync.Mutex
	Items    []Item
	FilePath string
	Saved    bool
	// Blobs holds images for slides/songs loaded from a schedule file, keyed by temp id.
	Blobs map[string][]byte
}

// New returns an empty schedule.
func New() *Schedule { return &Schedule{Saved: true, Blobs: map[string][]byte{}} }

func newKey() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Snapshot returns a copy of the items for JSON.
func (s *Schedule) Snapshot() (items []Item, file string, saved bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items = append([]Item{}, s.Items...)
	if items == nil {
		items = []Item{}
	}
	return items, s.FilePath, s.Saved
}

func (s *Schedule) Add(it Item) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it.Scid = -1
	s.Items = append(s.Items, it)
	s.Saved = false
}

func (s *Schedule) Remove(i int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i < 0 || i >= len(s.Items) {
		return
	}
	s.Items = append(s.Items[:i], s.Items[i+1:]...)
	s.Saved = false
}

func (s *Schedule) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Items = nil
	s.Saved = false
}

// Move moves item i to position j.
func (s *Schedule) Move(i, j int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.Items)
	if i < 0 || i >= n || j < 0 || j >= n || i == j {
		return
	}
	it := s.Items[i]
	s.Items = append(s.Items[:i], s.Items[i+1:]...)
	s.Items = append(s.Items[:j], append([]Item{it}, s.Items[j:]...)...)
	s.Saved = false
}

// Get returns item i.
func (s *Schedule) Get(i int) (Item, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i < 0 || i >= len(s.Items) {
		return Item{}, false
	}
	return s.Items[i], true
}

// NewFile starts an untitled schedule.
func (s *Schedule) NewFile() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FilePath = "untitled.spsc"
	s.Items = nil
	s.Saved = false
}

// Close discards the schedule.
func (s *Schedule) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FilePath = ""
	s.Items = nil
	s.Saved = true
	s.Blobs = map[string][]byte{}
}

// Blob returns an image stored for a schedule item.
func (s *Schedule) Blob(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.Blobs[key]
	return b, ok
}

// SetBlob stores an image for a schedule item and returns its key.
func (s *Schedule) SetBlob(b []byte) string {
	k := newKey()
	s.mu.Lock()
	s.Blobs[k] = b
	s.mu.Unlock()
	return k
}

var ddl = []string{
	"PRAGMA user_version = 2",
	`CREATE TABLE IF NOT EXISTS 'schedule' ('id' INTEGER PRIMARY KEY  AUTOINCREMENT  NOT NULL, 'stype' TEXT, 'name' TEXT, 'sorder' INTEGER )`,
	`CREATE TABLE IF NOT EXISTS 'bible' ('scid' INTEGER, 'verseIds' TEXT, 'caption' TEXT, 'captionLong' TEXT)`,
	`CREATE TABLE IF NOT EXISTS 'song' ('scid' INTEGER, 'songid' INTEGER, 'sbid' INTEGER, 'sbName' TEXT, 'number' INTEGER, 'title' TEXT, 'category' INTEGER, 'tune' TEXT, 'wordsBy' TEXT, 'musicBy' TEXT, 'songText' TEXT, 'notes' TEXT, 'usePrivate' BOOL, 'alignV' INTEGER, 'alignH' INTEGER, 'color' INTEGER, 'font' TEXT, 'infoColor' INTEGER, 'infoFont' TEXT, 'endingColor' INTEGER, 'endingFont' TEXT, 'useBack' BOOL, 'backImage' BLOB, 'backName' TEXT)`,
	`CREATE TABLE IF NOT EXISTS 'slideshow' ('scid' INTEGER, 'ssid' INTEGER, 'name' TEXT, 'info' TEXT)`,
	`CREATE TABLE IF NOT EXISTS 'slides' ('scid' INTEGER, 'sid' INTEGER, 'name' TEXT, 'path' TEXT, 'porder' INTEGER, 'image' BLOB, 'imageSmall' BLOB, 'imagePreview' BLOB)`,
	`CREATE TABLE IF NOT EXISTS 'media' ('scid' INTEGER, 'name' TEXT, 'path' TEXT, 'aRatio' INTEGER)`,
	`CREATE TABLE IF NOT EXISTS 'announce' ('scid' INTEGER, 'aId' INTEGER, 'title' TEXT, 'aText' TEXT, 'usePrivate' BOOL, 'useAuto' BOOL, 'loop' BOOL, 'slideTimer' INTEGER, 'font' TEXT, 'color' INTEGER, 'useBack' BOOL, 'backImage' BLOB, 'backPath' TEXT, 'alignV' INTEGER, 'alignH' INTEGER)`,
}

// Blobber resolves image bytes for songs/slides stored in the main database.
type Blobber interface {
	SongBackground(songID int64) []byte
	SlideImages(slideID int64) (full, small, prev []byte)
}

// Save writes the schedule to path (always rewriting the file completely,
// which is simpler than the original's partial update and gives the same result).
func (s *Schedule) Save(path string, bl Blobber) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("could not overwrite existing file: %w", err)
		}
	}
	d, err := db.OpenExternal(path, true)
	if err != nil {
		return err
	}
	defer d.Close()
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	for _, q := range ddl {
		if _, err := tx.Exec(q); err != nil {
			tx.Rollback()
			return err
		}
	}
	for i := range s.Items {
		it := &s.Items[i]
		res, err := tx.Exec("INSERT INTO schedule (stype, name, sorder) VALUES (?,?,?)", it.Type, it.Name, i+1)
		if err != nil {
			tx.Rollback()
			return err
		}
		scid, _ := res.LastInsertId()
		it.Scid = scid
		switch it.Type {
		case "bible":
			if it.Bible != nil {
				_, err = tx.Exec("INSERT INTO bible (scid, verseIds, caption, captionLong) VALUES (?,?,?,?)", scid, it.Bible.VerseIDs, it.Bible.Caption, it.Bible.CaptionLong)
			}
		case "song":
			if sg := it.Song; sg != nil {
				var back []byte
				if sg.ID > 0 && bl != nil {
					back = bl.SongBackground(sg.ID)
				} else if k := sg.BackgroundName; false && k != "" {
					_ = k
				}
				if back == nil {
					if b, ok := s.Blobs["song:"+fmt.Sprint(sg.ID)+":"+sg.BackgroundName]; ok {
						back = b
					}
				}
				_, err = tx.Exec(`INSERT INTO song (scid,songid,sbid,sbName,number,title,category,tune,wordsBy,musicBy,songText,notes,usePrivate,alignV,alignH,color,font,
					infoColor,infoFont,endingColor,endingFont,useBack,backImage,backName) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
					scid, sg.ID, sg.SongbookID, sg.SongbookName, sg.Number, sg.Title, sg.Category, sg.Tune, sg.WordsBy, sg.MusicBy, sg.Text, sg.Notes,
					sg.UsePrivate, sg.AlignV, sg.AlignH, qt.HexToRGB(sg.Color), sg.Font.String(), qt.HexToRGB(sg.InfoColor), sg.InfoFont.String(),
					qt.HexToRGB(sg.EndingColor), sg.EndingFont.String(), sg.UseBackground, back, sg.BackgroundName)
			}
		case "slideshow":
			if ss := it.Slideshow; ss != nil {
				if _, err = tx.Exec("INSERT INTO slideshow (scid, ssid, name, info) VALUES (?,?,?,?)", scid, ss.ID, ss.Name, ss.Info); err == nil {
					for j, sl := range ss.Slides {
						var full, small, prev []byte
						if sl.ID > 0 && bl != nil {
							full, small, prev = bl.SlideImages(sl.ID)
						}
						if full == nil && sl.Temp != "" {
							full, small, prev = s.Blobs[sl.Temp+":full"], s.Blobs[sl.Temp+":small"], s.Blobs[sl.Temp+":prev"]
						}
						if _, err = tx.Exec("INSERT INTO slides (scid, sid, name, path, porder, image, imageSmall, imagePreview) VALUES (?,?,?,?,?,?,?,?)",
							scid, sl.ID, sl.Name, sl.Path, j, full, small, prev); err != nil {
							break
						}
					}
				}
			}
		case "media":
			if m := it.Media; m != nil {
				_, err = tx.Exec("INSERT INTO media (scid, name, path, aRatio) VALUES (?,?,?,?)", scid, m.Name, m.Path, 0)
			}
		case "announce":
			if a := it.Announce; a != nil {
				_, err = tx.Exec(`INSERT INTO announce (scid,aId,title,aText,usePrivate,useAuto,loop,slideTimer,font,color,useBack,backImage,backPath,alignV,alignH)
					VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, scid, a.ID, a.Title, a.Text, a.UsePrivate, a.UseAuto, a.Loop, a.SlideTime, a.Font.String(),
					qt.HexToRGB(a.Color), a.UseBackground, []byte{}, a.BackgroundPath, a.AlignV, a.AlignH)
			}
		}
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.FilePath = path
	s.Saved = true
	return nil
}

// Open loads a schedule file replacing the current schedule.
func (s *Schedule) Open(path string) error {
	d, err := db.OpenExternal(path, false)
	if err != nil {
		return err
	}
	defer d.Close()
	ver, err := db.UserVersion(d)
	if err != nil {
		return err
	}
	if ver != 2 {
		return fmt.Errorf("the schedule file version %d is not compatible (compatible version: 2)", ver)
	}
	rows, err := d.Query("SELECT id, stype, name FROM schedule ORDER BY sorder")
	if err != nil {
		return err
	}
	type head struct {
		id          int64
		stype, name string
	}
	var heads []head
	for rows.Next() {
		var h head
		var st, nm sql.NullString
		if err := rows.Scan(&h.id, &st, &nm); err != nil {
			rows.Close()
			return err
		}
		h.stype, h.name = st.String, nm.String
		heads = append(heads, h)
	}
	rows.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Items = nil
	s.Blobs = map[string][]byte{}
	for _, h := range heads {
		it := Item{Type: h.stype, Name: h.name, Scid: h.id}
		switch h.stype {
		case "bible":
			var b BibleRef
			var v, c, cl sql.NullString
			if err := d.QueryRow("SELECT verseIds, caption, captionLong FROM bible WHERE scid = ?", h.id).Scan(&v, &c, &cl); err == nil {
				b.VerseIDs, b.Caption, b.CaptionLong = v.String, c.String, cl.String
				if b.Caption == "" {
					b.Caption = h.name
				}
				it.Bible = &b
			}
		case "song":
			sg := songs.NewSong()
			var sbid, number, category, alignV, alignH, color, infoColor, endingColor sql.NullInt64
			var sbName, title, tune, wordsBy, musicBy, text, notes, font, infoFont, endingFont, backName sql.NullString
			var usePrivate, useBack any
			var back []byte
			err := d.QueryRow(`SELECT songid,sbid,sbName,number,title,category,tune,wordsBy,musicBy,songText,notes,usePrivate,alignV,alignH,color,font,infoColor,infoFont,
				endingColor,endingFont,useBack,backImage,backName FROM song WHERE scid = ?`, h.id).
				Scan(&sg.ID, &sbid, &sbName, &number, &title, &category, &tune, &wordsBy, &musicBy, &text, &notes, &usePrivate, &alignV, &alignH, &color, &font,
					&infoColor, &infoFont, &endingColor, &endingFont, &useBack, &back, &backName)
			if err == nil {
				sg.SongbookID, sg.SongbookName, sg.Number, sg.Title, sg.Category = sbid.Int64, sbName.String, int(number.Int64), title.String, int(category.Int64)
				sg.Tune, sg.WordsBy, sg.MusicBy, sg.Text, sg.Notes = tune.String, wordsBy.String, musicBy.String, text.String, notes.String
				sg.UsePrivate, sg.AlignV, sg.AlignH = qt.ParseBool(usePrivate), int(alignV.Int64), int(alignH.Int64)
				if color.Valid {
					sg.Color = qt.RGBToHex(color.Int64)
				}
				if f, ok := qt.ParseFont(font.String); ok {
					sg.Font = f
				}
				if infoColor.Valid {
					sg.InfoColor = qt.RGBToHex(infoColor.Int64)
				}
				if f, ok := qt.ParseFont(infoFont.String); ok {
					sg.InfoFont = f
				}
				if endingColor.Valid {
					sg.EndingColor = qt.RGBToHex(endingColor.Int64)
				}
				if f, ok := qt.ParseFont(endingFont.String); ok {
					sg.EndingFont = f
				}
				sg.UseBackground, sg.BackgroundName = qt.ParseBool(useBack), backName.String
				if len(back) > 0 {
					sg.HasBackground = true
					s.Blobs["song:"+fmt.Sprint(sg.ID)+":"+sg.BackgroundName] = back
				}
				it.Song = &sg
			}
		case "slideshow":
			ss := slides.ShowWithSlides{Slides: []slides.Slide{}}
			var name, info sql.NullString
			if err := d.QueryRow("SELECT ssid, name, info FROM slideshow WHERE scid = ?", h.id).Scan(&ss.ID, &name, &info); err == nil {
				ss.Name, ss.Info = name.String, info.String
				srows, err := d.Query("SELECT sid, name, path, porder, image, imageSmall, imagePreview FROM slides WHERE scid = ? ORDER BY porder", h.id)
				if err == nil {
					for srows.Next() {
						var sl slides.Slide
						var sname, spath sql.NullString
						var sid, order sql.NullInt64
						var full, small, prev []byte
						if err := srows.Scan(&sid, &sname, &spath, &order, &full, &small, &prev); err != nil {
							continue
						}
						// Slides from a file are referenced by temp key, even if the
						// id matches something in the local database.
						sl.Name, sl.Path, sl.Order = sname.String, spath.String, int(order.Int64)
						sl.Temp = newKey()
						s.Blobs[sl.Temp+":full"], s.Blobs[sl.Temp+":small"], s.Blobs[sl.Temp+":prev"] = full, small, prev
						if img, err := slides.DecodeBytes(full); err == nil {
							sl.Width, sl.Height = img.Bounds().Dx(), img.Bounds().Dy()
						}
						ss.Slides = append(ss.Slides, sl)
					}
					srows.Close()
				}
				it.Slideshow = &ss
			}
		case "media":
			var name, p sql.NullString
			if err := d.QueryRow("SELECT name, path FROM media WHERE scid = ?", h.id).Scan(&name, &p); err == nil {
				path := strings.TrimPrefix(p.String, "file://")
				it.Media = &media.Item{Name: name.String, Path: path}
			}
		case "announce":
			a := announce.New()
			var title, text, font, backPath sql.NullString
			var color, slideTimer, alignV, alignH sql.NullInt64
			var usePrivate, useAuto, loop, useBack any
			if err := d.QueryRow("SELECT aId, title, aText, usePrivate, useAuto, loop, slideTimer, font, color, useBack, backPath, alignV, alignH FROM announce WHERE scid = ?", h.id).
				Scan(&a.ID, &title, &text, &usePrivate, &useAuto, &loop, &slideTimer, &font, &color, &useBack, &backPath, &alignV, &alignH); err == nil {
				a.Title, a.Text = title.String, text.String
				a.UsePrivate, a.UseAuto, a.Loop, a.SlideTime = qt.ParseBool(usePrivate), qt.ParseBool(useAuto), qt.ParseBool(loop), int(slideTimer.Int64)
				if f, ok := qt.ParseFont(font.String); ok {
					a.Font = f
				}
				if color.Valid {
					a.Color = qt.RGBToHex(color.Int64)
				}
				a.UseBackground, a.BackgroundPath = qt.ParseBool(useBack), backPath.String
				a.AlignV, a.AlignH = int(alignV.Int64), int(alignH.Int64)
				it.Announce = &a
			}
		}
		s.Items = append(s.Items, it)
	}
	s.FilePath = path
	s.Saved = true
	return nil
}

// Label returns the display name for a new item of a given payload.
func Label(it Item) string {
	switch it.Type {
	case "bible":
		if it.Bible != nil {
			return it.Bible.Caption
		}
	case "song":
		if it.Song != nil {
			return it.Song.Label()
		}
	case "slideshow":
		if it.Slideshow != nil {
			return it.Slideshow.Name
		}
	case "media":
		if it.Media != nil {
			return it.Media.Name
		}
	case "announce":
		if it.Announce != nil {
			return it.Announce.Title
		}
	}
	return it.Name
}
