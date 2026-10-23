// Package modules implements import/export of the SoftProjector exchange
// formats (.sps songbooks, .spb bibles, .spt themes) and downloads from the
// module servers.
package modules

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"wordprojector/internal/db"
	"wordprojector/internal/qt"
	"wordprojector/internal/songs"
)

// Progress receives progress updates during long operations. max may be 0 when unknown.
type Progress func(cur, max int, msg string)

func noProgress(int, int, string) {}

// ErrUnsupported is returned for unknown/too old/too new file versions.
var ErrUnsupported = errors.New("unsupported file format")

// Importer bundles the database handle.
type Importer struct{ DB *db.DB }

// ImportSongbook imports a .sps file of any supported version and returns the
// new songbook id.
func (im *Importer) ImportSongbook(path string, progress Progress) (int64, error) {
	if progress == nil {
		progress = noProgress
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	head := make([]byte, 16)
	n, _ := io.ReadFull(f, head)
	f.Close()
	head = head[:n]
	switch {
	case bytes.HasPrefix(head, []byte("SQLite")):
		return im.importSongbookSQLite(path, progress)
	case bytes.HasPrefix(head, []byte("##")):
		return im.importSongbookLegacy(path, progress)
	case bytes.HasPrefix(bytes.TrimLeft(head, "\xef\xbb\xbf \t\r\n"), []byte("<?xml")):
		return im.importSongbookXML(path, progress)
	}
	return 0, fmt.Errorf("%w: this songbook file is in a very old format no longer supported", ErrUnsupported)
}

func (im *Importer) insertSongbook(tx *sql.Tx, title, info string) (int64, error) {
	res, err := tx.Exec("INSERT INTO Songbooks (name, info) VALUES (?,?)", strings.TrimSpace(title), strings.TrimSpace(info))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (im *Importer) importSongbookSQLite(path string, progress Progress) (int64, error) {
	src, err := db.OpenExternal(path, false)
	if err != nil {
		return 0, err
	}
	defer src.Close()
	ver, err := db.UserVersion(src)
	if err != nil {
		return 0, err
	}
	if ver > 2 {
		return 0, fmt.Errorf("%w: songbook file version %d is newer than supported version 2", ErrUnsupported, ver)
	}
	if ver < 2 {
		return 0, fmt.Errorf("%w: songbook file version %d is not supported", ErrUnsupported, ver)
	}
	var title, info sql.NullString
	if err := src.QueryRow("SELECT title, info FROM SongBook").Scan(&title, &info); err != nil {
		return 0, fmt.Errorf("reading SongBook table: %w", err)
	}
	var max int
	_ = src.QueryRow("SELECT count(*) FROM Songs").Scan(&max)
	rows, err := src.Query(`SELECT number,title,category,tune,words,music,song_text,notes,use_private,alignment_v,alignment_h,color,font,
		info_color,info_font,ending_color,ending_font,use_background,background_name,background,count,date FROM Songs`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var sbid int64
	err = im.DB.Tx(func(tx *sql.Tx) error {
		var err error
		sbid, err = im.insertSongbook(tx, title.String, info.String)
		if err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT INTO Songs (songbook_id,number,title,category,tune,words,music,song_text,notes,use_private,alignment_v,alignment_h,
			color,font,info_color,info_font,ending_color,ending_font,use_background,background_name,background,count,date) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		row := 0
		for rows.Next() {
			var vals [22]any
			ptrs := make([]any, 22)
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				return err
			}
			st := qt.ParseString(vals[6])
			if songs.HasOldLineMarkers(st) {
				st = songs.CleanSongLines(st)
			}
			vals[6] = st
			args := append([]any{sbid}, vals[:]...)
			if _, err := stmt.Exec(args...); err != nil {
				return err
			}
			row++
			if row%50 == 0 {
				progress(row, max, "")
			}
		}
		return rows.Err()
	})
	progress(max, max, "")
	return sbid, err
}

func (im *Importer) importSongbookLegacy(path string, progress Progress) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	readLine := func() (string, bool) {
		line, err := r.ReadString('\n')
		if err != nil && line == "" {
			return "", false
		}
		return strings.TrimRight(line, "\r\n"), true
	}
	if _, ok := readLine(); !ok { // version line "##..."
		return 0, ErrUnsupported
	}
	titleLine, _ := readLine()
	infoLine, _ := readLine()
	title := strings.TrimSpace(strings.ReplaceAll(titleLine, "#", ""))
	info := songs.ToMultiLine(strings.TrimSpace(strings.ReplaceAll(infoLine, "#", "")))
	var sbid int64
	err = im.DB.Tx(func(tx *sql.Tx) error {
		var err error
		sbid, err = im.insertSongbook(tx, title, info)
		if err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT INTO Songs (songbook_id, number, title, category, tune, words, music, song_text, font, background_name, notes) VALUES (?,?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		row := 0
		for {
			line, ok := readLine()
			if !ok {
				break
			}
			if strings.TrimSpace(line) == "" {
				continue
			}
			split := strings.Split(line, "#$#")
			for len(split) < 7 {
				split = append(split, "")
			}
			num, _ := strconv.Atoi(strings.TrimSpace(split[0]))
			cat, _ := strconv.Atoi(strings.TrimSpace(split[2]))
			st := split[6]
			if songs.HasOldLineMarkers(st) {
				st = songs.CleanSongLines(st)
			}
			font, back, notes := "", "", ""
			if len(split) > 7 {
				font = split[7]
				if len(split) > 9 {
					back = split[9]
				}
				if len(split) > 10 {
					notes = songs.ToMultiLine(split[10])
				}
			}
			if _, err := stmt.Exec(sbid, num, split[1], cat, split[3], split[4], split[5], st, font, back, notes); err != nil {
				return err
			}
			row++
			if row%50 == 0 {
				progress(row, 0, "")
			}
		}
		return nil
	})
	return sbid, err
}

type xmlSongBook struct {
	XMLName  xml.Name `xml:"spSongBook"`
	Version  string   `xml:"version,attr"`
	SongBook struct {
		Title string `xml:"title"`
		Info  string `xml:"info"`
	} `xml:"SongBook"`
	Songs []xmlSong `xml:"Song"`
}

type xmlSong struct {
	Number     string `xml:"number,attr"`
	Title      string `xml:"title"`
	Category   string `xml:"category"`
	Tune       string `xml:"tune"`
	Words      string `xml:"words"`
	Music      string `xml:"music"`
	SongText   string `xml:"song_text"`
	Notes      string `xml:"notes"`
	UsePrivate string `xml:"use_private"`
	Alignment  string `xml:"alignment"`
	Color      string `xml:"color"`
	Font       string `xml:"font"`
	Background string `xml:"background"`
	Count      string `xml:"count"`
	Date       string `xml:"date"`
}

func (im *Importer) importSongbookXML(path string, progress Progress) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var sb xmlSongBook
	if err := xml.Unmarshal(data, &sb); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrUnsupported, err)
	}
	if v, _ := strconv.ParseFloat(sb.Version, 64); v != 2.0 {
		return 0, fmt.Errorf("%w: XML songbook version %q", ErrUnsupported, sb.Version)
	}
	var sbid int64
	err = im.DB.Tx(func(tx *sql.Tx) error {
		var err error
		sbid, err = im.insertSongbook(tx, sb.SongBook.Title, sb.SongBook.Info)
		if err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT INTO Songs (songbook_id, number, title, category, tune, words, music, song_text, notes, use_private, alignment_v, alignment_h,
			color, font, background_name, count, date) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for i, s := range sb.Songs {
			num, _ := strconv.Atoi(strings.TrimSpace(s.Number))
			cat, _ := strconv.Atoi(strings.TrimSpace(s.Category))
			st := s.SongText
			if songs.HasOldLineMarkers(st) {
				st = songs.CleanSongLines(st)
			}
			av, ah := 1, 1
			if parts := strings.Split(s.Alignment, ","); len(parts) == 2 {
				av, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
				ah, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
			}
			var color any
			if c, ok := qt.ParseInt(s.Color); ok {
				color = c
			}
			count, _ := strconv.Atoi(strings.TrimSpace(s.Count))
			if _, err := stmt.Exec(sbid, num, s.Title, cat, s.Tune, s.Words, s.Music, st, s.Notes, qt.ParseBool(s.UsePrivate), av, ah,
				color, s.Font, s.Background, count, s.Date); err != nil {
				return err
			}
			if i%50 == 0 {
				progress(i, len(sb.Songs), "")
			}
		}
		return nil
	})
	return sbid, err
}

// ExportSongbook writes a version 2 .sps SQLite file for the songbook.
func (im *Importer) ExportSongbook(path string, songbookID int64, progress Progress) (string, error) {
	if progress == nil {
		progress = noProgress
	}
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return "", fmt.Errorf("could not overwrite existing file: %w", err)
		}
	}
	var name, info sql.NullString
	if err := im.DB.QueryRow("SELECT name, info FROM Songbooks WHERE id = ?", songbookID).Scan(&name, &info); err != nil {
		return "", err
	}
	dst, err := db.OpenExternal(path, true)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	tx, err := dst.Begin()
	if err != nil {
		return "", err
	}
	stmts := []string{
		"PRAGMA user_version = 2",
		`CREATE TABLE 'SongBook' ('title' TEXT, 'info' TEXT)`,
		`CREATE TABLE 'Songs' ('number' INTEGER, 'title' TEXT, 'category' INTEGER DEFAULT 0, 'tune' TEXT, 'words' TEXT, 'music' TEXT, 'song_text' TEXT, 'notes' TEXT, 'use_private' BOOL, 'alignment_v' INTEGER, 'alignment_h' INTEGER, 'color' INTEGER, 'font' TEXT, 'info_color' INTEGER, 'info_font' TEXT, 'ending_color' INTEGER, 'ending_font' TEXT, 'use_background' BOOL, 'background_name' TEXT, 'background' BLOB, 'count' INTEGER DEFAULT 0, 'date' TEXT)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			tx.Rollback()
			return "", err
		}
	}
	if _, err := tx.Exec("INSERT INTO SongBook (title, info) VALUES (?,?)", name.String, info.String); err != nil {
		tx.Rollback()
		return "", err
	}
	rows, err := im.DB.Query(`SELECT number,title,category,tune,words,music,song_text,notes,use_private,alignment_v,alignment_h,color,font,info_color,info_font,
		ending_color,ending_font,use_background,background_name,background,count,date FROM Songs WHERE songbook_id = ? ORDER BY number, id`, songbookID)
	if err != nil {
		tx.Rollback()
		return "", err
	}
	defer rows.Close()
	stmt, err := tx.Prepare(`INSERT INTO Songs (number,title,category,tune,words,music,song_text,notes,use_private,alignment_v,alignment_h,color,font,info_color,info_font,
		ending_color,ending_font,use_background,background_name,background,count,date) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		return "", err
	}
	n := 0
	for rows.Next() {
		var vals [22]any
		ptrs := make([]any, 22)
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			tx.Rollback()
			return "", err
		}
		if _, err := stmt.Exec(vals[:]...); err != nil {
			tx.Rollback()
			return "", err
		}
		n++
		if n%50 == 0 {
			progress(n, 0, "")
		}
	}
	stmt.Close()
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return name.String, nil
}
