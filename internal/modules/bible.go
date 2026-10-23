package modules

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

)

// ImportBible imports a .spb text file (spDataVersion 1) and returns the new bible id.
func (im *Importer) ImportBible(path string, progress Progress) (int64, error) {
	if progress == nil {
		progress = noProgress
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)
	readLine := func() (string, bool) {
		line, err := r.ReadString('\n')
		if err != nil && line == "" {
			return "", false
		}
		return strings.TrimRight(line, "\r\n"), true
	}
	line, _ := readLine()
	line = strings.TrimPrefix(line, "\xef\xbb\xbf")
	if !strings.HasPrefix(line, "##spData") {
		return 0, fmt.Errorf("%w: the Bible file is of an unsupported (old) format", ErrUnsupported)
	}
	if parts := strings.Split(line, "\t"); len(parts) < 2 || strings.TrimSpace(parts[1]) != "1" {
		return 0, fmt.Errorf("%w: the Bible file is of a newer format", ErrUnsupported)
	}
	field := func() string {
		l, _ := readLine()
		parts := strings.SplitN(l, "\t", 2)
		if len(parts) < 2 {
			return ""
		}
		return strings.TrimSpace(parts[1])
	}
	title := field()
	abbr := field()
	info := strings.TrimSpace(strings.Join(strings.Split(field(), "@%"), "\n"))
	rtol := field()
	rtl := 0
	if rtol == "1" || strings.EqualFold(rtol, "true") {
		rtl = 1
	}
	var id int64
	err = im.DB.Tx(func(tx *sql.Tx) error {
		res, err := tx.Exec("INSERT INTO BibleVersions (bible_name, abbreviation, information, right_to_left) VALUES (?,?,?,?)", title, abbr, info, rtl)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		if err != nil {
			return err
		}
		bookStmt, err := tx.Prepare("INSERT INTO BibleBooks (bible_id, id, book_name, chapter_count) VALUES (?,?,?,?)")
		if err != nil {
			return err
		}
		defer bookStmt.Close()
		for {
			l, ok := readLine()
			if !ok {
				return fmt.Errorf("%w: unexpected end of file in book list", ErrUnsupported)
			}
			if strings.HasPrefix(l, "---") {
				break
			}
			parts := strings.Split(l, "\t")
			if len(parts) < 3 {
				continue
			}
			bid, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			cc, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
			if _, err := bookStmt.Exec(id, bid, strings.TrimSpace(parts[1]), cc); err != nil {
				return err
			}
		}
		verseStmt, err := tx.Prepare("INSERT INTO BibleVerse (verse_id, bible_id, book, chapter, verse, verse_text) VALUES (?,?,?,?,?,?)")
		if err != nil {
			return err
		}
		defer verseStmt.Close()
		row := 0
		idStr := strconv.FormatInt(id, 10)
		for {
			l, ok := readLine()
			if !ok {
				break
			}
			if strings.TrimSpace(l) == "" {
				continue
			}
			parts := strings.Split(l, "\t")
			if len(parts) < 5 {
				continue
			}
			ch, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
			vn, _ := strconv.Atoi(strings.TrimSpace(parts[3]))
			if _, err := verseStmt.Exec(strings.TrimSpace(parts[0]), idStr, strings.TrimSpace(parts[1]), ch, vn, parts[4]); err != nil {
				return err
			}
			row++
			if row%500 == 0 {
				progress(row, 31200, "")
			}
		}
		return nil
	})
	return id, err
}

// ExportBible writes a .spb text file for the bible and returns its title.
func (im *Importer) ExportBible(path string, bibleID int64, progress Progress) (string, error) {
	if progress == nil {
		progress = noProgress
	}
	var title, abbr, info sql.NullString
	var rtl any
	if err := im.DB.QueryRow("SELECT bible_name, abbreviation, information, right_to_left FROM BibleVersions WHERE id = ?", bibleID).Scan(&title, &abbr, &info, &rtl); err != nil {
		return "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	w := bufio.NewWriterSize(f, 1<<20)
	infoOne := strings.Join(strings.Split(strings.TrimSpace(info.String), "\n"), "@%")
	rtlStr := ""
	if v, ok := rtl.(int64); ok {
		rtlStr = strconv.FormatInt(v, 10)
	} else if s, ok := rtl.(string); ok {
		rtlStr = s
	}
	fmt.Fprintf(w, "##spDataVersion:\t1\n##Title:\t%s\n##Abbreviation:\t%s\n##Information:\t%s\n##RightToLeft:\t%s\n",
		strings.TrimSpace(title.String), strings.TrimSpace(abbr.String), strings.TrimSpace(infoOne), rtlStr)
	rows, err := im.DB.Query("SELECT id, book_name, chapter_count FROM BibleBooks WHERE bible_id = ? ORDER BY id", bibleID)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var id, cc int
		var name sql.NullString
		if err := rows.Scan(&id, &name, &cc); err != nil {
			rows.Close()
			return "", err
		}
		fmt.Fprintf(w, "%d\t%s\t%d\n", id, strings.TrimSpace(name.String), cc)
	}
	rows.Close()
	w.WriteString("-----")
	vrows, err := im.DB.Query("SELECT verse_id, book, chapter, verse, verse_text FROM BibleVerse WHERE bible_id = ? OR bible_id = ? ORDER BY rowid", strconv.FormatInt(bibleID, 10), bibleID)
	if err != nil {
		return "", err
	}
	defer vrows.Close()
	n := 0
	for vrows.Next() {
		var vid, book, text sql.NullString
		var ch, vn sql.NullInt64
		if err := vrows.Scan(&vid, &book, &ch, &vn, &text); err != nil {
			return "", err
		}
		fmt.Fprintf(w, "\n%s\t%s\t%d\t%d\t%s", strings.TrimSpace(vid.String), strings.TrimSpace(book.String), ch.Int64, vn.Int64, strings.TrimSpace(text.String))
		n++
		if n%1000 == 0 {
			progress(n, 31000, "")
		}
	}
	return strings.TrimSpace(title.String), w.Flush()
}
