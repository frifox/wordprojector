// Package slides manages picture slide shows. Every picture is kept as JPEG in three sizes, as files in the
// data directory (the original program kept them in the database).
package slides

import (
	"bytes"
	"database/sql"
	"image"
	"image/jpeg"
	_ "image/gif"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"wordprojector/internal/db"
)

// Settings mirrors SlideShowSettings ("pix" settings group).
type Settings struct {
	ExpandSmall bool `json:"expandSmall"`
	FitType     int  `json:"fitType"`
	Resize      bool `json:"resize"`
	BoundType   int  `json:"boundType"`
	BoundWidth  int  `json:"boundWidth"`
}

// DefaultSettings returns the original defaults.
func DefaultSettings() Settings {
	return Settings{ExpandSmall: true, FitType: 0, Resize: true, BoundType: 2, BoundWidth: 1280}
}

// Show is a slide show header.
type Show struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Info    string `json:"info"`
	Created string `json:"created"` // the day it was made, YYYY-MM-DD
}

// Slide is one picture of a slide show (image bytes are fetched separately).
type Slide struct {
	ID    int64  `json:"id"`
	Order int    `json:"order"`
	Name  string `json:"name"`
	Path  string `json:"path"`
	// Temp is set for images added but not saved yet (kept in memory).
	Temp string `json:"temp,omitempty"`
	// Linked slides show the picture where it is on disk (Path): only the small thumbnails are kept. The others
	// hold a copy of it.
	Linked bool `json:"linked,omitempty"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ShowWithSlides is a full slide show.
type ShowWithSlides struct {
	Show
	Slides []Slide `json:"slides"`
}

// Images holds the three encoded variants of a picture.
type Images struct {
	Full, Small, Preview []byte
	Width, Height        int
}

// Repo is the slide show repository.
//
// With Dir set, the pictures live as files there (full, small and preview variants) and the database only holds
// the slide rows; without it (tests) they are kept in the database.
type Repo struct {
	DB  *db.DB
	Dir string
}

func (r *Repo) filePath(file, size string) string {
	switch size {
	case "small":
		return filepath.Join(r.Dir, file+"_s.jpg")
	case "prev":
		return filepath.Join(r.Dir, file+"_p.jpg")
	}
	return filepath.Join(r.Dir, file+".jpg")
}

func (r *Repo) writeFiles(file string, img *Images, linked bool) error {
	if err := os.MkdirAll(r.Dir, 0o755); err != nil {
		return err
	}
	for _, v := range []struct {
		size string
		b    []byte
	}{{"full", img.Full}, {"small", img.Small}, {"prev", img.Preview}} {
		if linked && v.size == "full" {
			continue // the picture stays where it is
		}
		if err := os.WriteFile(r.filePath(file, v.size), v.b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) removeFiles(file string) {
	if r.Dir == "" || file == "" {
		return
	}
	for _, sz := range []string{"full", "small", "prev"} {
		_ = os.Remove(r.filePath(file, sz))
	}
}

func (r *Repo) List() ([]Show, error) {
	rows, err := r.DB.Query("SELECT id, name, info, COALESCE(created, '') FROM SlideShows ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Show{}
	for rows.Next() {
		var s Show
		var n, i sql.NullString
		if err := rows.Scan(&s.ID, &n, &i, &s.Created); err != nil {
			return nil, err
		}
		s.Name, s.Info = n.String, i.String
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repo) Get(id int64) (ShowWithSlides, error) {
	var s ShowWithSlides
	var n, i sql.NullString
	if err := r.DB.QueryRow("SELECT id, name, info, COALESCE(created, '') FROM SlideShows WHERE id = ?", id).Scan(&s.ID, &n, &i, &s.Created); err != nil {
		return s, err
	}
	s.Name, s.Info = n.String, i.String
	rows, err := r.DB.Query("SELECT id, p_order, name, path, pix, COALESCE(file,''), COALESCE(linked,0) FROM Slides WHERE ss_id = ? ORDER BY p_order, id", id)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	s.Slides = []Slide{}
	for rows.Next() {
		var sl Slide
		var ord sql.NullInt64
		var name, path sql.NullString
		var pix []byte
		var file string
		var linked int
		if err := rows.Scan(&sl.ID, &ord, &name, &path, &pix, &file, &linked); err != nil {
			return s, err
		}
		sl.Order, sl.Name, sl.Path, sl.Linked = int(ord.Int64), name.String, path.String, linked != 0
		if sl.Linked {
			// the size of the picture where it lies (nothing when it has been moved away)
			if f, err := os.Open(sl.Path); err == nil {
				if cfg, _, err := image.DecodeConfig(f); err == nil {
					sl.Width, sl.Height = cfg.Width, cfg.Height
				}
				f.Close()
			}
		} else if file != "" {
			if f, err := os.Open(r.filePath(file, "full")); err == nil {
				if cfg, _, err := image.DecodeConfig(f); err == nil {
					sl.Width, sl.Height = cfg.Width, cfg.Height
				}
				f.Close()
			}
		} else if cfg, _, err := image.DecodeConfig(bytes.NewReader(pix)); err == nil {
			sl.Width, sl.Height = cfg.Width, cfg.Height
		}
		s.Slides = append(s.Slides, sl)
	}
	sort.SliceStable(s.Slides, func(a, b int) bool { return s.Slides[a].Order < s.Slides[b].Order })
	return s, rows.Err()
}

// Image returns the stored image of a slide: size is "full", "small" or "prev". The full picture of a linked slide is
// read from where it lies.
func (r *Repo) Image(id int64, size string) ([]byte, error) {
	col := "pix"
	switch size {
	case "small":
		col = "pix_small"
	case "prev":
		col = "pix_prev"
	}
	var b []byte
	var file, path string
	var linked int
	err := r.DB.QueryRow("SELECT "+col+", COALESCE(file,''), COALESCE(path,''), COALESCE(linked,0) FROM Slides WHERE id = ?", id).Scan(&b, &file, &path, &linked)
	if err != nil {
		return b, err
	}
	if linked != 0 && size != "small" && size != "prev" {
		return readLinked(path)
	}
	if file != "" {
		return os.ReadFile(r.filePath(file, size))
	}
	return b, nil
}

// readLinked is the picture at path as the browser can show it: as it is when it is in a format every browser reads,
// else as a JPEG.
func readLinked(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return b, nil
	}
	img, err := DecodeBytes(b)
	if err != nil {
		return nil, err
	}
	return EncodeJPEG(img), nil
}

// Save stores a slide show. New slides (ID == 0) take their images from
// newImages keyed by Temp. Slides missing from the list are deleted.
func (r *Repo) Save(s ShowWithSlides, newImages map[string]*Images) (int64, error) {
	id := s.ID
	var written, gone []string // files created by this save / files of removed slides
	err := r.DB.Tx(func(tx *sql.Tx) error {
		if id == 0 {
			res, err := tx.Exec("INSERT INTO SlideShows (name, info, created) VALUES (?,?,?)", s.Name, s.Info, time.Now().Format("2006-01-02"))
			if err != nil {
				return err
			}
			id, err = res.LastInsertId()
			if err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec("UPDATE SlideShows SET name = ?, info = ? WHERE id = ?", s.Name, s.Info, id); err != nil {
				return err
			}
		}
		keep := map[int64]bool{}
		for i, sl := range s.Slides {
			if sl.ID > 0 {
				keep[sl.ID] = true
				if _, err := tx.Exec("UPDATE Slides SET p_order = ? WHERE id = ?", i, sl.ID); err != nil {
					return err
				}
				continue
			}
			img := newImages[sl.Temp]
			if img == nil {
				continue
			}
			var res sql.Result
			var err error
			if r.Dir == "" {
				res, err = tx.Exec("INSERT INTO Slides (ss_id, p_order, name, path, pix, pix_small, pix_prev) VALUES (?,?,?,?,?,?,?)",
					id, i, sl.Name, sl.Path, img.Full, img.Small, img.Preview)
			} else {
				res, err = tx.Exec("INSERT INTO Slides (ss_id, p_order, name, path, linked) VALUES (?,?,?,?,?)", id, i, sl.Name, sl.Path, b2i(sl.Linked))
			}
			if err != nil {
				return err
			}
			nid, _ := res.LastInsertId()
			if r.Dir != "" {
				file := strconv.FormatInt(nid, 10)
				written = append(written, file)
				if err := r.writeFiles(file, img, sl.Linked); err != nil {
					return err
				}
				if _, err := tx.Exec("UPDATE Slides SET file = ? WHERE id = ?", file, nid); err != nil {
					return err
				}
			}
			keep[nid] = true
		}
		rows, err := tx.Query("SELECT id, COALESCE(file,'') FROM Slides WHERE ss_id = ?", id)
		if err != nil {
			return err
		}
		var del []int64
		for rows.Next() {
			var sid int64
			var file string
			if err := rows.Scan(&sid, &file); err != nil {
				rows.Close()
				return err
			}
			if !keep[sid] {
				del = append(del, sid)
				if file != "" {
					gone = append(gone, file)
				}
			}
		}
		rows.Close()
		for _, sid := range del {
			if _, err := tx.Exec("DELETE FROM Slides WHERE id = ?", sid); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		for _, f := range written {
			r.removeFiles(f)
		}
		return id, err
	}
	for _, f := range gone {
		r.removeFiles(f)
	}
	return id, nil
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (r *Repo) Delete(id int64) error {
	var files []string
	err := r.DB.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM SlideShows WHERE id = ?", id); err != nil {
			return err
		}
		rows, err := tx.Query("SELECT COALESCE(file,'') FROM Slides WHERE ss_id = ?", id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var f string
			if err := rows.Scan(&f); err == nil && f != "" {
				files = append(files, f)
			}
		}
		rows.Close()
		_, err = tx.Exec("DELETE FROM Slides WHERE ss_id = ?", id)
		return err
	})
	if err == nil {
		for _, f := range files {
			r.removeFiles(f)
		}
	}
	return err
}

// ---- image processing -----------------------------------------------------

// Decode reads an image file from disk.
func Decode(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// DecodeBytes decodes an image held in memory.
func DecodeBytes(b []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(b))
	return img, err
}

// Scale scales img to fit inside maxW x maxH keeping the aspect ratio.
func Scale(img image.Image, maxW, maxH int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxW && h <= maxH {
		return img
	}
	rw := float64(maxW) / float64(w)
	rh := float64(maxH) / float64(h)
	ratio := rw
	if rh < rw {
		ratio = rh
	}
	nw, nh := int(float64(w)*ratio), int(float64(h)*ratio)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}

// EncodeJPEG encodes with quality 90 like QPixmap::save("JPG", 90).
func EncodeJPEG(img image.Image) []byte {
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	return buf.Bytes()
}

// Prepare builds the three image variants according to the slide settings
// (port of PictureWidget::on_pushButtonAddImages_clicked).
func Prepare(img image.Image, sets Settings) *Images {
	out := &Images{}
	full := img
	if sets.Resize && sets.BoundWidth > 0 {
		full = Scale(img, sets.BoundWidth, sets.BoundWidth)
	}
	out.Width, out.Height = full.Bounds().Dx(), full.Bounds().Dy()
	out.Full = EncodeJPEG(full)
	out.Preview = EncodeJPEG(Scale(img, 400, 400))
	out.Small = EncodeJPEG(Scale(img, 100, 100))
	return out
}

// SupportedExt lists image extensions accepted by the file dialogs.
var SupportedExt = []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tif", ".tiff"}

// IsImage reports whether the path has a supported image extension.
func IsImage(path string) bool {
	ext := filepath.Ext(path)
	for _, e := range SupportedExt {
		if e == ext || e == toLower(ext) {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
