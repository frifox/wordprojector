// Package bible provides the Bible model, chapter/verse lookups, search and
// the verse/caption composition used for projection (ported from bible.cpp).
package bible

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"wordprojector/internal/db"
	"wordprojector/internal/qt"
	"wordprojector/internal/songs"
	"wordprojector/internal/translit"
)

// Info describes one Bible translation.
type Info struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Abbr string `json:"abbr"`
	Info string `json:"info"`
	RTL  bool   `json:"rtl"`
}

// Book is a Bible book.
type Book struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Chapters int    `json:"chapters"`
}

// Verse is a verse row as stored in the database.
type Verse struct {
	ID      string `json:"id"`
	Book    int    `json:"book"`
	Chapter int    `json:"chapter"`
	Verse   int    `json:"verse"`
	Text    string `json:"text"`
}

// ChapterVerse is one line of the chapter preview list. IDs may hold several
// comma separated verse ids when the source splits one verse over rows.
type ChapterVerse struct {
	Num  int    `json:"num"`
	Text string `json:"text"`
	IDs  string `json:"ids"`
}

// SearchHit is one search result.
type SearchHit struct {
	Book     string `json:"book"`
	BookID   int    `json:"bookId"`
	Chapter  int    `json:"chapter"`
	Verse    int    `json:"verse"`
	Text     string `json:"text"`
	Display  string `json:"display"`
}

// Repo is the bible repository with an in-memory verse cache per translation.
type Repo struct {
	DB    *db.DB
	mu    sync.Mutex
	cache map[int64]*loaded
}

type loaded struct {
	books  []Book
	verses []Verse
}

func NewRepo(d *db.DB) *Repo { return &Repo{DB: d, cache: map[int64]*loaded{}} }

// Invalidate drops cached verses (after import/delete).
func (r *Repo) Invalidate() {
	r.mu.Lock()
	r.cache = map[int64]*loaded{}
	r.mu.Unlock()
}

func (r *Repo) List() ([]Info, error) {
	rows, err := r.DB.Query("SELECT id, bible_name, abbreviation, information, right_to_left FROM BibleVersions ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Info{}
	for rows.Next() {
		var b Info
		var name, abbr, info sql.NullString
		var rtl any
		if err := rows.Scan(&b.ID, &name, &abbr, &info, &rtl); err != nil {
			return nil, err
		}
		b.Name, b.Abbr, b.Info = strings.TrimSpace(name.String), strings.TrimSpace(abbr.String), info.String
		b.RTL = qt.ParseBool(rtl)
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *Repo) Get(id int64) (Info, error) {
	list, err := r.List()
	if err != nil {
		return Info{}, err
	}
	for _, b := range list {
		if b.ID == id {
			return b, nil
		}
	}
	return Info{}, sql.ErrNoRows
}

func (r *Repo) UpdateInfo(b Info) error {
	rtl := 0
	if b.RTL {
		rtl = 1
	}
	return r.DB.Tx(func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE BibleVersions SET bible_name = ?, abbreviation = ?, information = ?, right_to_left = ? WHERE id = ?",
			strings.TrimSpace(b.Name), strings.TrimSpace(b.Abbr), strings.TrimSpace(b.Info), rtl, b.ID)
		return err
	})
}

func (r *Repo) Delete(id int64) error {
	defer r.Invalidate()
	return r.DB.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM BibleBooks WHERE bible_id = ?", id); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM BibleVerse WHERE bible_id = ?", strconv.FormatInt(id, 10)); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM BibleVerse WHERE bible_id = ?", id); err != nil {
			return err
		}
		_, err := tx.Exec("DELETE FROM BibleVersions WHERE id = ?", id)
		return err
	})
}

func (r *Repo) load(id int64) (*loaded, error) {
	r.mu.Lock()
	if l, ok := r.cache[id]; ok {
		r.mu.Unlock()
		return l, nil
	}
	r.mu.Unlock()
	l := &loaded{}
	rows, err := r.DB.Query("SELECT id, book_name, chapter_count FROM BibleBooks WHERE bible_id = ? ORDER BY id", id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var b Book
		var name sql.NullString
		if err := rows.Scan(&b.ID, &name, &b.Chapters); err != nil {
			rows.Close()
			return nil, err
		}
		b.Name = strings.TrimSpace(name.String)
		l.books = append(l.books, b)
	}
	rows.Close()
	// bible_id is TEXT in BibleVerse; match both representations.
	vrows, err := r.DB.Query("SELECT verse_id, book, chapter, verse, verse_text FROM BibleVerse WHERE bible_id = ? OR bible_id = ? ORDER BY rowid", strconv.FormatInt(id, 10), id)
	if err != nil {
		return nil, err
	}
	defer vrows.Close()
	for vrows.Next() {
		var v Verse
		var vid, book, text sql.NullString
		var ch, vn sql.NullInt64
		if err := vrows.Scan(&vid, &book, &ch, &vn, &text); err != nil {
			return nil, err
		}
		v.ID = strings.TrimSpace(vid.String)
		v.Book, _ = strconv.Atoi(strings.TrimSpace(book.String))
		v.Chapter, v.Verse = int(ch.Int64), int(vn.Int64)
		v.Text = strings.TrimSpace(text.String)
		l.verses = append(l.verses, v)
	}
	if err := vrows.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.cache[id] = l
	r.mu.Unlock()
	return l, nil
}

func (r *Repo) Books(id int64) ([]Book, error) {
	l, err := r.load(id)
	if err != nil {
		return nil, err
	}
	if l.books == nil {
		return []Book{}, nil
	}
	return l.books, nil
}

func (r *Repo) bookName(l *loaded, bookID int) string {
	for _, b := range l.books {
		if b.ID == bookID {
			return b.Name
		}
	}
	return ""
}

// Chapter returns the verses of a chapter, merging rows that share a verse
// number exactly like Bible::getChapter does.
func (r *Repo) Chapter(id int64, book, chapter int) ([]ChapterVerse, error) {
	l, err := r.load(id)
	if err != nil {
		return nil, err
	}
	out := []ChapterVerse{}
	verseOld := 0
	for _, v := range l.verses {
		if v.Book != book || v.Chapter != chapter {
			continue
		}
		if v.Verse == verseOld && len(out) > 0 {
			last := &out[len(out)-1]
			last.Text = strings.Join(strings.Fields(last.Text), " ") + " " + v.Text
			last.IDs += "," + v.ID
		} else {
			out = append(out, ChapterVerse{Num: v.Verse, Text: v.Text, IDs: v.ID})
		}
		verseOld = v.Verse
	}
	return out, nil
}

// Ref resolves a (possibly comma separated) verse id list into book id,
// chapter, first verse and last verse numbers within the given bible.
func (r *Repo) Ref(id int64, verseIDs string) (bookID, chapter, first, last int, bookName string, err error) {
	l, err := r.load(id)
	if err != nil {
		return
	}
	ids := strings.Split(verseIDs, ",")
	firstID := strings.TrimSpace(ids[0])
	lastID := strings.TrimSpace(ids[len(ids)-1])
	for _, v := range l.verses {
		if v.ID == firstID {
			bookID, chapter, first = v.Book, v.Chapter, v.Verse
			bookName = r.bookName(l, v.Book)
		}
		if v.ID == lastID {
			last = v.Verse
		}
	}
	if first == 0 {
		// fall back to decoding the id itself: B001C001V001
		bookID, chapter, first = decodeVerseID(firstID)
		_, _, last = decodeVerseID(lastID)
		bookName = r.bookName(l, bookID)
	}
	return
}

func decodeVerseID(id string) (b, c, v int) {
	re := regexp.MustCompile(`B(\d+)C(\d+)V(\d+)`)
	m := re.FindStringSubmatch(id)
	if len(m) == 4 {
		b, _ = strconv.Atoi(m[1])
		c, _ = strconv.Atoi(m[2])
		v, _ = strconv.Atoi(m[3])
	}
	return
}

// VerseID builds the canonical verse id "B001C001V001".
func VerseID(book, chapter, verse int) string {
	return fmt.Sprintf("B%03dC%03dV%03d", book, chapter, verse)
}

// Passage is the projected text and caption for one translation.
type Passage struct {
	Text    string `json:"text"`
	Caption string `json:"caption"`
}

// VerseAndCaption composes the projection text and caption for the given verse
// ids in the given translation (port of Bible::getVerseAndCaption).
func (r *Repo) VerseAndCaption(id int64, verseIDs string, useAbbr bool) (Passage, error) {
	l, err := r.load(id)
	if err != nil {
		return Passage{}, err
	}
	ids := strings.Split(verseIDs, ",")
	set := map[string]bool{}
	for _, s := range ids {
		set[strings.TrimSpace(s)] = true
	}
	var found []Verse
	for _, v := range l.verses {
		if set[v.ID] {
			found = append(found, v)
		}
	}
	if len(found) == 0 {
		return Passage{}, nil
	}
	var p Passage
	book := found[0].Book
	if len(ids) > 1 {
		var show strings.Builder
		first := found[0].Verse
		nOld := -1
		caption := ""
		for _, v := range found {
			if v.Verse == nOld {
				if v.Verse == first {
					// Same verse split over rows: no verse numbers at all.
					s := show.String()
					if i := strings.Index(s, ")"); i >= 0 && strings.HasPrefix(strings.TrimSpace(s), "(") {
						s = s[i+1:]
					}
					show.Reset()
					show.WriteString(s)
					caption = fmt.Sprintf(" %d:%d", v.Chapter, v.Verse)
				} else {
					caption = fmt.Sprintf(" %d:%d-%d", v.Chapter, first, v.Verse)
				}
				show.WriteString(" " + v.Text)
			} else {
				caption = fmt.Sprintf(" %d:%d-%d", v.Chapter, first, v.Verse)
				if show.Len() == 0 {
					show.WriteString(fmt.Sprintf(" (%d) %s", v.Verse, v.Text))
				} else {
					show.WriteString(fmt.Sprintf(" (%d) %s", v.Verse, v.Text))
				}
			}
			nOld = v.Verse
		}
		p.Text = strings.Join(strings.Fields(show.String()), " ")
		p.Caption = caption
	} else {
		v := found[0]
		p.Text = v.Text
		p.Caption = fmt.Sprintf(" %d:%d", v.Chapter, v.Verse)
	}
	p.Caption = r.bookName(l, book) + p.Caption
	if useAbbr {
		if info, err := r.Get(id); err == nil && info.Abbr != "" {
			p.Caption = fmt.Sprintf("%s (%s)", p.Caption, info.Abbr)
		}
	}
	p.Text = strings.Join(strings.Fields(p.Text), " ")
	p.Caption = strings.Join(strings.Fields(p.Caption), " ")
	return p, nil
}

// isCyrillicBible samples the first verses to see whether the text is Cyrillic.
func isCyrillicBible(verses []Verse) bool {
	for i, v := range verses {
		if i >= 200 {
			break
		}
		if translit.HasCyrillic(v.Text) {
			return true
		}
	}
	return false
}

// Search searches verse texts. book/chapter of 0 mean "any". See
// songs.NewWordMatcher for the meaning of exact and fullWord.
func (r *Repo) Search(id int64, text string, exact, fullWord bool, book, chapter int) ([]SearchHit, string, error) {
	l, err := r.load(id)
	if err != nil {
		return nil, "", err
	}
	// Latin typing against a Cyrillic Bible is matched by sound ("moi" finds "мой")
	newMatcher := songs.NewWordMatcher
	if translit.IsLatinQuery(text) && isCyrillicBible(l.verses) {
		newMatcher = songs.NewPhoneticMatcher
	}
	m, err := newMatcher(text, exact, fullWord)
	if err != nil {
		return []SearchHit{}, "", err
	}
	hits := []SearchHit{}
	for _, v := range l.verses {
		if book != 0 && v.Book != book {
			continue
		}
		if chapter != 0 && v.Chapter != chapter {
			continue
		}
		if !m.Match(v.Text) {
			continue
		}
		name := r.bookName(l, v.Book)
		hits = append(hits, SearchHit{Book: name, BookID: v.Book, Chapter: v.Chapter, Verse: v.Verse, Text: v.Text,
			Display: fmt.Sprintf("%s %d:%d %s", name, v.Chapter, v.Verse, v.Text)})
		if len(hits) >= 5000 {
			break
		}
	}
	return hits, m.Highlight, nil
}

// VerseCount returns the number of verse rows (used for export progress).
func (r *Repo) Verses(id int64) ([]Verse, error) {
	l, err := r.load(id)
	if err != nil {
		return nil, err
	}
	return l.verses, nil
}
