package modules

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"wordprojector/internal/db"
)

var themeTables = []struct {
	name string
	cols []string
}{
	{"ThemeAnnounce", strings.Fields("disp use_shadow use_fading use_blur_shadow use_background background_name background text_font text_color text_align_v text_align_h use_disp_1")},
	{"ThemeBible", strings.Fields("disp use_shadow use_fading use_blur_shadow use_background background_name background text_font text_color text_align_v text_align_h caption_font caption_color caption_align caption_position use_abbr screen_use screen_position use_disp_1 add_background_color_to_text text_rec_background_color text_gen_background_color")},
	{"ThemePassive", strings.Fields("disp use_background background_name background use_disp_1")},
	{"ThemeSong", strings.Fields("disp use_shadow use_fading use_blur_shadow show_stanza_title show_key show_number info_color info_font info_align show_song_ending ending_color ending_font ending_type ending_position use_background background_name background text_font text_color text_align_v text_align_h screen_use screen_position use_disp_1 add_background_color_to_text text_rec_background_color text_gen_background_color")},
}

var themeDDL = []string{
	`CREATE TABLE IF NOT EXISTS 'ThemeAnnounce' ('theme_id' INTEGER, 'disp' INTEGER, 'use_shadow' BOOL, 'use_fading' BOOL, 'use_blur_shadow' BOOL, 'use_background' BOOL, 'background_name' TEXT, 'background' BLOB, 'text_font' TEXT, 'text_color' INTEGER, 'text_align_v' INTEGER, 'text_align_h' INTEGER, 'use_disp_1' BOOL)`,
	`CREATE TABLE IF NOT EXISTS 'ThemeBible' ('theme_id' INTEGER, 'disp' INTEGER, 'use_shadow' BOOL, 'use_fading' BOOL, 'use_blur_shadow' BOOL, 'use_background' BOOL, 'background_name' TEXT, 'background' BLOB, 'text_font' TEXT, 'text_color' INTEGER, 'text_align_v' INTEGER, 'text_align_h' INTEGER, 'caption_font' TEXT, 'caption_color' INTEGER, 'caption_align' INTEGER, 'caption_position' INTEGER, 'use_abbr' BOOL, 'screen_use' INTEGER, 'screen_position' INTEGER, 'use_disp_1' BOOL, 'add_background_color_to_text' BOOL, 'text_rec_background_color' INTEGER, 'text_gen_background_color' INTEGER)`,
	`CREATE TABLE IF NOT EXISTS 'ThemePassive' ('theme_id' INTEGER, 'disp' INTEGER, 'use_background' BOOL, 'background_name' TEXT, 'background' BLOB, 'use_disp_1' BOOL)`,
	`CREATE TABLE IF NOT EXISTS 'ThemeSong' ('theme_id' INTEGER, 'disp' INTEGER, 'use_shadow' BOOL, 'use_fading' BOOL, 'use_blur_shadow' BOOL, 'show_stanza_title' BOOL, 'show_key' BOOL, 'show_number' BOOL, 'info_color' INTEGER, 'info_font' TEXT, 'info_align' INTEGER, 'show_song_ending' BOOL, 'ending_color' INTEGER, 'ending_font' TEXT, 'ending_type' INTEGER, 'ending_position' INTEGER, 'use_background' BOOL, 'background_name' TEXT, 'background' BLOB, 'text_font' TEXT, 'text_color' INTEGER, 'text_align_v' INTEGER, 'text_align_h' INTEGER, 'screen_use' INTEGER, 'screen_position' INTEGER, 'use_disp_1' BOOL, 'add_background_color_to_text' BOOL, 'text_rec_background_color' INTEGER, 'text_gen_background_color' INTEGER)`,
	`CREATE TABLE IF NOT EXISTS 'Themes' ('id' INTEGER , 'name' TEXT, 'comment' TEXT)`,
}

// copyThemeRows copies the per-display rows of one table from src to dst.
func copyThemeRows(src interface{ Query(string, ...any) (*sql.Rows, error) }, dst *sql.Tx, table string, cols []string, fromID, toID int64) error {
	rows, err := src.Query(fmt.Sprintf("SELECT %s FROM %s WHERE theme_id = ?", strings.Join(cols, ","), table), fromID)
	if err != nil {
		// Source may lack a column (older files): fall back to the columns it has.
		return nil
	}
	defer rows.Close()
	qs := strings.Repeat("?,", len(cols)+1)
	stmt, err := dst.Prepare(fmt.Sprintf("INSERT INTO %s (theme_id,%s) VALUES (%s)", table, strings.Join(cols, ","), qs[:len(qs)-1]))
	if err != nil {
		return err
	}
	defer stmt.Close()
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		if _, err := stmt.Exec(append([]any{toID}, vals...)...); err != nil {
			return err
		}
	}
	return rows.Err()
}

// ImportTheme imports every theme contained in a .spt file and returns the new ids.
func (im *Importer) ImportTheme(path string, progress Progress) ([]int64, error) {
	if progress == nil {
		progress = noProgress
	}
	src, err := db.OpenExternal(path, false)
	if err != nil {
		return nil, err
	}
	defer src.Close()
	ver, err := db.UserVersion(src)
	if err != nil {
		return nil, err
	}
	if ver > 2 {
		return nil, fmt.Errorf("%w: theme file version %d is newer than supported version 2", ErrUnsupported, ver)
	}
	if ver < 2 {
		return nil, fmt.Errorf("%w: theme file version %d is not supported", ErrUnsupported, ver)
	}
	rows, err := src.Query("SELECT id, name, comment FROM Themes")
	if err != nil {
		return nil, err
	}
	type th struct {
		id            int64
		name, comment sql.NullString
	}
	var themes []th
	for rows.Next() {
		var t th
		if err := rows.Scan(&t.id, &t.name, &t.comment); err != nil {
			rows.Close()
			return nil, err
		}
		themes = append(themes, t)
	}
	rows.Close()
	var ids []int64
	err = im.DB.Tx(func(tx *sql.Tx) error {
		for i, t := range themes {
			res, err := tx.Exec("INSERT INTO Themes (name, comment) VALUES (?,?)", t.name.String, t.comment.String)
			if err != nil {
				return err
			}
			nid, _ := res.LastInsertId()
			for _, tb := range themeTables {
				if err := copyThemeRows(src, tx, tb.name, tb.cols, t.id, nid); err != nil {
					return err
				}
			}
			ids = append(ids, nid)
			progress(i+1, len(themes), "")
		}
		return nil
	})
	return ids, err
}

// ExportTheme writes the given themes (all when ids is empty) to a .spt file.
func (im *Importer) ExportTheme(path string, ids []int64, progress Progress) error {
	if progress == nil {
		progress = noProgress
	}
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("could not overwrite existing file: %w", err)
		}
	}
	if len(ids) == 0 {
		rows, err := im.DB.Query("SELECT id FROM Themes")
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
	}
	dst, err := db.OpenExternal(path, true)
	if err != nil {
		return err
	}
	defer dst.Close()
	tx, err := dst.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec("PRAGMA user_version = 2"); err != nil {
		tx.Rollback()
		return err
	}
	for _, s := range themeDDL {
		if _, err := tx.Exec(s); err != nil {
			tx.Rollback()
			return err
		}
	}
	for i, id := range ids {
		var name, comment sql.NullString
		if err := im.DB.QueryRow("SELECT name, comment FROM Themes WHERE id = ?", id).Scan(&name, &comment); err != nil {
			tx.Rollback()
			return err
		}
		if _, err := tx.Exec("INSERT INTO Themes (id, name, comment) VALUES (?,?,?)", id, name.String, comment.String); err != nil {
			tx.Rollback()
			return err
		}
		for _, tb := range themeTables {
			if err := copyThemeRows(im.DB, tx, tb.name, tb.cols, id, id); err != nil {
				tx.Rollback()
				return err
			}
		}
		progress(i+1, len(ids), "")
	}
	return tx.Commit()
}
