// Package media stores the media (audio/video) library: plain file references.
package media

import (
	"database/sql"
	"path/filepath"
	"strings"

	"wordprojector/internal/db"
)

// Item is one library entry.
type Item struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// AudioExt and VideoExt list the supported extensions (as in the original).
var AudioExt = []string{".mp3", ".aac", ".acc", ".ogg", ".oga", ".wma", ".wav", ".asf", ".mka", ".m4a", ".flac"}
var VideoExt = []string{".wmv", ".avi", ".mkv", ".flv", ".mp4", ".mpg", ".mpeg", ".mov", ".ogv", ".ts", ".webm", ".m4v"}

// IsSupported reports whether a file extension is a known media type.
func IsSupported(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range append(append([]string{}, AudioExt...), VideoExt...) {
		if e == ext {
			return true
		}
	}
	return false
}

type Repo struct{ DB *db.DB }

func (r *Repo) List() ([]Item, error) {
	rows, err := r.DB.Query("SELECT long_path, short_path FROM Media ORDER BY rowid")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var p, n sql.NullString
		if err := rows.Scan(&p, &n); err != nil {
			return nil, err
		}
		out = append(out, Item{Path: p.String, Name: n.String})
	}
	return out, rows.Err()
}

func (r *Repo) Add(paths []string) error {
	return r.DB.Tx(func(tx *sql.Tx) error {
		for _, p := range paths {
			if _, err := tx.Exec("INSERT INTO Media (long_path, short_path) VALUES (?,?)", p, filepath.Base(p)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repo) Remove(path string) error {
	return r.DB.Tx(func(tx *sql.Tx) error {
		_, err := tx.Exec("DELETE FROM Media WHERE long_path = ?", path)
		return err
	})
}
