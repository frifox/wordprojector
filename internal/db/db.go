// Package db opens the SoftProjector-compatible SQLite database and creates
// the schema (identical to the spData.sqlite version 2 of the original program).
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// Version is the PRAGMA user_version the original application requires.
const Version = 2

// Schema is the exact DDL used by SoftProjector 2.x for a fresh database.
var Schema = []string{
	`CREATE TABLE IF NOT EXISTS 'Announcements' ('id' INTEGER PRIMARY KEY  AUTOINCREMENT  NOT NULL  UNIQUE , 'title' TEXT, 'text' TEXT, 'usePrivate' BOOL, 'useAuto' BOOL, 'loop' BOOL, 'slideTime' INTEGER, 'useBackground' BOOL, 'backgoundPath' TEXT, 'font' TEXT, 'color' TEXT, 'alignment' TEXT)`,
	`CREATE TABLE IF NOT EXISTS 'BibleBooks' ('bible_id' INTEGER, 'id' INTEGER, 'book_name' TEXT, 'chapter_count' INTEGER DEFAULT 0)`,
	`CREATE TABLE IF NOT EXISTS 'BibleVerse' ('verse_id' TEXT, 'bible_id' TEXT, 'book' TEXT, 'chapter' INTEGER, 'verse' INTEGER, 'verse_text' TEXT)`,
	`CREATE TABLE IF NOT EXISTS 'BibleVersions' ('id' INTEGER PRIMARY KEY  AUTOINCREMENT  NOT NULL, 'bible_name' TEXT, 'abbreviation' TEXT, 'information' TEXT, 'right_to_left' INTEGER DEFAULT 0)`,
	`CREATE TABLE IF NOT EXISTS 'Media' ('long_path' TEXT, 'short_path' TEXT)`,
	`CREATE TABLE IF NOT EXISTS 'Settings' ('type' TEXT, 'sets' TEXT)`,
	`CREATE TABLE IF NOT EXISTS 'SlideShows' ('id' INTEGER PRIMARY KEY  AUTOINCREMENT  NOT NULL , 'name' TEXT, 'info' TEXT)`,
	`CREATE TABLE IF NOT EXISTS 'Slides' ('id' INTEGER PRIMARY KEY  AUTOINCREMENT  NOT NULL , 'ss_id' INTEGER, 'p_order' INTEGER, 'name' TEXT, 'path' TEXT, 'pix' BLOB, 'pix_small' BLOB, 'pix_prev' BLOB)`,
	`CREATE TABLE IF NOT EXISTS 'Songbooks' ('id' INTEGER PRIMARY KEY  AUTOINCREMENT  NOT NULL , 'name' TEXT, 'info' TEXT)`,
	`CREATE TABLE IF NOT EXISTS 'Songs' ('id' INTEGER PRIMARY KEY  AUTOINCREMENT  NOT NULL , 'songbook_id' INTEGER, 'number' INTEGER, 'title' TEXT, 'category' INTEGER DEFAULT 0, 'tune' TEXT, 'words' TEXT, 'music' TEXT, 'song_text' TEXT, 'notes' TEXT, 'use_private' BOOL, 'alignment_v' INTEGER, 'alignment_h' INTEGER, 'color' INTEGER, 'font' TEXT, 'info_color' INTEGER, 'info_font' TEXT, 'ending_color' INTEGER, 'ending_font' TEXT, 'use_background' BOOL, 'background_name' TEXT, 'background' BLOB, 'count' INTEGER DEFAULT 0, 'date' TEXT)`,
	`CREATE TABLE IF NOT EXISTS 'ThemeAnnounce' ('theme_id' INTEGER, 'disp' INTEGER, 'use_shadow' BOOL, 'use_fading' BOOL, 'use_blur_shadow' BOOL, 'use_background' BOOL, 'background_name' TEXT, 'background' BLOB, 'text_font' TEXT, 'text_color' INTEGER, 'text_align_v' INTEGER, 'text_align_h' INTEGER, 'use_disp_1' BOOL)`,
	`CREATE TABLE IF NOT EXISTS 'ThemeBible' ('theme_id' INTEGER, 'disp' INTEGER, 'use_shadow' BOOL, 'use_fading' BOOL, 'use_blur_shadow' BOOL, 'use_background' BOOL, 'background_name' TEXT, 'background' BLOB, 'text_font' TEXT, 'text_color' INTEGER, 'text_align_v' INTEGER, 'text_align_h' INTEGER, 'caption_font' TEXT, 'caption_color' INTEGER, 'caption_align' INTEGER, 'caption_position' INTEGER, 'use_abbr' BOOL, 'screen_use' INTEGER, 'screen_position' INTEGER, 'use_disp_1' BOOL, 'add_background_color_to_text' BOOL, 'text_rec_background_color' INTEGER, 'text_gen_background_color' INTEGER)`,
	`CREATE TABLE IF NOT EXISTS 'ThemePassive' ('theme_id' INTEGER, 'disp' INTEGER, 'use_background' BOOL, 'background_name' TEXT, 'background' BLOB, 'use_disp_1' BOOL)`,
	`CREATE TABLE IF NOT EXISTS 'ThemeSong' ('theme_id' INTEGER, 'disp' INTEGER, 'use_shadow' BOOL, 'use_fading' BOOL, 'use_blur_shadow' BOOL, 'show_stanza_title' BOOL, 'show_key' BOOL, 'show_number' BOOL, 'info_color' INTEGER, 'info_font' TEXT, 'info_align' INTEGER, 'show_song_ending' BOOL, 'ending_color' INTEGER, 'ending_font' TEXT, 'ending_type' INTEGER, 'ending_position' INTEGER, 'use_background' BOOL, 'background_name' TEXT, 'background' BLOB, 'text_font' TEXT, 'text_color' INTEGER, 'text_align_v' INTEGER, 'text_align_h' INTEGER, 'screen_use' INTEGER, 'screen_position' INTEGER, 'use_disp_1' BOOL, 'add_background_color_to_text' BOOL, 'text_rec_background_color' INTEGER, 'text_gen_background_color' INTEGER)`,
	`CREATE TABLE IF NOT EXISTS 'Themes' ('id' INTEGER PRIMARY KEY  AUTOINCREMENT  NOT NULL , 'name' TEXT, 'comment' TEXT)`,
}

// DB wraps the sql.DB. SQLite is single-writer, so writes are serialised with
// a mutex to avoid SQLITE_BUSY under concurrent HTTP handlers.
type DB struct {
	*sql.DB
	Path string
	mu   sync.Mutex
}

// Open opens (or creates) the database file at path.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	_, statErr := os.Stat(path)
	existed := statErr == nil
	sdb, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(0)")
	if err != nil {
		return nil, err
	}
	sdb.SetMaxOpenConns(1)
	d := &DB{DB: sdb, Path: path}
	if err := d.Ping(); err != nil {
		return nil, err
	}
	if !existed {
		if err := d.createSchema(); err != nil {
			return nil, err
		}
	} else {
		var v int
		if err := d.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
			return nil, err
		}
		if v != Version && v != 0 {
			return nil, fmt.Errorf("database %s has version %d, this program requires version %d", path, v, Version)
		}
		// Make sure every table exists (tolerates partially created files).
		if err := d.createSchema(); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func (d *DB) createSchema() error {
	for _, s := range Schema {
		if _, err := d.Exec(s); err != nil {
			return fmt.Errorf("schema: %w", err)
		}
	}
	if err := d.addColumns(); err != nil {
		return err
	}
	_, err := d.Exec(fmt.Sprintf("PRAGMA user_version = %d", Version))
	return err
}

// addColumns adds the few columns this program has on top of the original schema (old files get them here).
func (d *DB) addColumns() error {
	has := func(col string) (bool, error) {
		var n int
		err := d.QueryRow("SELECT count(*) FROM pragma_table_info('ThemePassive') WHERE name = ?", col).Scan(&n)
		return n > 0, err
	}
	// an earlier draft used another encoding under this name
	if old, err := has("background_fit"); err != nil {
		return err
	} else if old {
		if _, err := d.Exec("ALTER TABLE ThemePassive DROP COLUMN background_fit"); err != nil {
			return fmt.Errorf("schema: %w", err)
		}
	}
	if ok, err := has("image_scale"); err != nil {
		return err
	} else if !ok {
		// how the passive image fills the screen: 0 scale (stretch, the original behaviour), 1 fill (cover), 2 fit (contain)
		if _, err := d.Exec("ALTER TABLE ThemePassive ADD COLUMN image_scale INTEGER NOT NULL DEFAULT 0"); err != nil {
			return fmt.Errorf("schema: %w", err)
		}
	}
	// the list of background images a theme keeps for a slide kind and display; the selected one is also stored
	// in the original single-image column, which is what the slides and the exported themes use
	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS ThemeBackgrounds (
		id INTEGER PRIMARY KEY AUTOINCREMENT, theme_id INTEGER, kind TEXT, disp INTEGER,
		name TEXT, data BLOB, selected INTEGER NOT NULL DEFAULT 0)`); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	if n, err := d.colCount("ThemeBackgrounds", "file"); err != nil {
		return err
	} else if n == 0 {
		if _, err := d.Exec("ALTER TABLE ThemeBackgrounds ADD COLUMN file TEXT"); err != nil {
			return fmt.Errorf("schema: %w", err)
		}
	}
	if n, err := d.colCount("Themes", "bg_color"); err != nil {
		return err
	} else if n == 0 {
		if _, err := d.Exec("ALTER TABLE Themes ADD COLUMN bg_color TEXT"); err != nil {
			return fmt.Errorf("schema: %w", err)
		}
	}
	if n, err := d.colCount("Themes", "bg_opacity"); err != nil {
		return err
	} else if n == 0 {
		if _, err := d.Exec("ALTER TABLE Themes ADD COLUMN bg_opacity INTEGER"); err != nil {
			return fmt.Errorf("schema: %w", err)
		}
	}
	// the color behind the text: the space above and below the text, and how opaque it is (percent); and how wide the
	// characters are (percent of normal)
	for _, table := range []string{"ThemeBible", "ThemeSong"} {
		for _, col := range []struct{ name, def string }{{"bk_padding", "10"}, {"bk_opacity", "100"}, {"text_scale_h", "100"}} {
			if n, err := d.colCount(table, col.name); err != nil {
				return err
			} else if n == 0 {
				if _, err := d.Exec("ALTER TABLE " + table + " ADD COLUMN " + col.name + " INTEGER NOT NULL DEFAULT " + col.def); err != nil {
					return fmt.Errorf("schema: %w", err)
				}
			}
		}
	}
	// how wide the characters of the caption (Bible), the song information and the song ending are (percent of normal)
	for table, cols := range map[string][]string{"ThemeBible": {"caption_scale_h"}, "ThemeSong": {"info_scale_h", "ending_scale_h"}} {
		for _, col := range cols {
			if n, err := d.colCount(table, col); err != nil {
				return err
			} else if n == 0 {
				if _, err := d.Exec("ALTER TABLE " + table + " ADD COLUMN " + col + " INTEGER NOT NULL DEFAULT 100"); err != nil {
					return fmt.Errorf("schema: %w", err)
				}
			}
		}
	}
	// the day a slide show was made (YYYY-MM-DD)
	if n, err := d.colCount("SlideShows", "created"); err != nil {
		return err
	} else if n == 0 {
		if _, err := d.Exec("ALTER TABLE SlideShows ADD COLUMN created TEXT"); err != nil {
			return fmt.Errorf("schema: %w", err)
		}
	}
	// slide show pictures live as files in the data directory; "file" names them
	var n int
	if err := d.QueryRow("SELECT count(*) FROM pragma_table_info('Slides') WHERE name = 'file'").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := d.Exec("ALTER TABLE Slides ADD COLUMN file TEXT"); err != nil {
			return fmt.Errorf("schema: %w", err)
		}
	}
	// a slide can be linked to its picture on disk instead of holding a copy of it
	if err := d.QueryRow("SELECT count(*) FROM pragma_table_info('Slides') WHERE name = 'linked'").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := d.Exec("ALTER TABLE Slides ADD COLUMN linked INTEGER"); err != nil {
			return fmt.Errorf("schema: %w", err)
		}
	}
	return nil
}

func (d *DB) colCount(table, col string) (n int, err error) {
	err = d.QueryRow("SELECT count(*) FROM pragma_table_info('"+table+"') WHERE name = ?", col).Scan(&n)
	return
}

// Lock serialises writers. Callers must defer Unlock.
func (d *DB) Lock()   { d.mu.Lock() }
func (d *DB) Unlock() { d.mu.Unlock() }

// Tx runs fn inside a write transaction while holding the writer lock.
func (d *DB) Tx(fn func(tx *sql.Tx) error) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	tx, err := d.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// OpenExternal opens an arbitrary SQLite file (module/schedule files). The
// caller must close it.
func OpenExternal(path string, create bool) (*sql.DB, error) {
	if !create {
		if _, err := os.Stat(path); err != nil {
			return nil, err
		}
	}
	sdb, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	sdb.SetMaxOpenConns(1)
	if err := sdb.Ping(); err != nil {
		sdb.Close()
		return nil, err
	}
	return sdb, nil
}

// UserVersion reads PRAGMA user_version from any sqlite handle.
func UserVersion(sdb *sql.DB) (int, error) {
	var v int
	err := sdb.QueryRow("PRAGMA user_version").Scan(&v)
	return v, err
}

// LastInsertID returns the last autoincrement value of a table, the way the
// original program did (sqlite_sequence).
func LastInsertID(q interface {
	QueryRow(query string, args ...any) *sql.Row
}, table string) (int64, error) {
	var id int64
	err := q.QueryRow("SELECT seq FROM sqlite_sequence WHERE name = ?", table).Scan(&id)
	return id, err
}
