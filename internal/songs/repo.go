package songs

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"wordprojector/internal/db"
	"wordprojector/internal/qt"
	"wordprojector/internal/translit"
)

// Songbook is a collection of songs.
type Songbook struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Info string `json:"info"`
}

// Song is one song. Background image bytes are not included; use Background().
type Song struct {
	ID            int64   `json:"id"`
	SongbookID    int64   `json:"songbookId"`
	SongbookName  string  `json:"songbookName"`
	Number        int     `json:"number"`
	Title         string  `json:"title"`
	Category      int     `json:"category"`
	Tune          string  `json:"tune"`
	WordsBy       string  `json:"wordsBy"`
	MusicBy       string  `json:"musicBy"`
	Text          string  `json:"text,omitempty"`
	Notes         string  `json:"notes"`
	UsePrivate    bool    `json:"usePrivate"`
	AlignV        int     `json:"alignV"`
	AlignH        int     `json:"alignH"`
	Color         string  `json:"color"`
	Font          qt.Font `json:"font"`
	InfoColor     string  `json:"infoColor"`
	InfoFont      qt.Font `json:"infoFont"`
	EndingColor   string  `json:"endingColor"`
	EndingFont    qt.Font `json:"endingFont"`
	UseBackground bool    `json:"useBackground"`
	BackgroundName string `json:"backgroundName"`
	HasBackground bool    `json:"hasBackground"`
	Count         int     `json:"count"`
	Date          string  `json:"date"`
}

// NewSong returns a song with the original program's defaults.
func NewSong() Song {
	return Song{Number: 1, AlignV: 1, AlignH: 1, Color: "#ffffff", Font: qt.DefaultFont(28),
		InfoColor: "#ffffff", InfoFont: qt.DefaultFont(28), EndingColor: "#ffffff", EndingFont: qt.DefaultFont(28)}
}

// Repo is the songs repository.
type Repo struct{ DB *db.DB }

// ---- songbooks ------------------------------------------------------------

func (r *Repo) Songbooks() ([]Songbook, error) {
	rows, err := r.DB.Query("SELECT id, name, info FROM Songbooks ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Songbook{}
	for rows.Next() {
		var sb Songbook
		var name, info sql.NullString
		if err := rows.Scan(&sb.ID, &name, &info); err != nil {
			return nil, err
		}
		sb.Name, sb.Info = name.String, info.String
		out = append(out, sb)
	}
	return out, rows.Err()
}

func (r *Repo) Songbook(id int64) (Songbook, error) {
	var sb Songbook
	var name, info sql.NullString
	err := r.DB.QueryRow("SELECT id, name, info FROM Songbooks WHERE id = ?", id).Scan(&sb.ID, &name, &info)
	sb.Name, sb.Info = name.String, info.String
	return sb, err
}

func (r *Repo) AddSongbook(name, info string) (int64, error) {
	var id int64
	err := r.DB.Tx(func(tx *sql.Tx) error {
		res, err := tx.Exec("INSERT INTO Songbooks (name, info) VALUES (?,?)", strings.TrimSpace(name), info)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	return id, err
}

func (r *Repo) UpdateSongbook(id int64, name, info string) error {
	return r.DB.Tx(func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE Songbooks SET name = ?, info = ? WHERE id = ?", strings.TrimSpace(name), strings.TrimSpace(info), id)
		return err
	})
}

func (r *Repo) DeleteSongbook(id int64) error {
	return r.DB.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM Songbooks WHERE id = ?", id); err != nil {
			return err
		}
		_, err := tx.Exec("DELETE FROM Songs WHERE songbook_id = ?", id)
		return err
	})
}

// ---- songs ----------------------------------------------------------------

const songCols = `s.id, s.songbook_id, s.number, s.title, s.category, s.tune, s.words, s.music, s.song_text, s.notes,
 s.use_private, s.alignment_v, s.alignment_h, s.color, s.font, s.info_color, s.info_font, s.ending_color, s.ending_font,
 s.use_background, s.background_name, (s.background IS NOT NULL AND length(s.background) > 0), s.count, s.date, COALESCE(b.name,'')`

// lenientInt is a nullable integer column that also accepts what other programs leave in it: an empty or
// non-numeric text counts as NULL instead of failing the scan (and with it the whole song list).
type lenientInt struct {
	Int64 int64
	Valid bool
}

func (n *lenientInt) Scan(v any) error {
	n.Int64, n.Valid = 0, false
	switch t := v.(type) {
	case int64:
		n.Int64, n.Valid = t, true
	case float64:
		n.Int64, n.Valid = int64(t), true
	case bool:
		n.Valid = true
		if t {
			n.Int64 = 1
		}
	case []byte:
		return n.Scan(string(t))
	case string:
		if i, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64); err == nil {
			n.Int64, n.Valid = i, true
		}
	}
	return nil
}

func scanSong(sc interface{ Scan(...any) error }, withText bool) (Song, error) {
	s := NewSong()
	var (
		sbid, number, category                       lenientInt
		title, tune, words, music, text, notes, date sql.NullString
		usePrivate, useBack, hasBack                 any
		alignV, alignH                               lenientInt
		color, infoColor, endingColor                lenientInt
		font, infoFont, endingFont, backName         sql.NullString
		count                                        lenientInt
		sbname                                       sql.NullString
	)
	err := sc.Scan(&s.ID, &sbid, &number, &title, &category, &tune, &words, &music, &text, &notes,
		&usePrivate, &alignV, &alignH, &color, &font, &infoColor, &infoFont, &endingColor, &endingFont,
		&useBack, &backName, &hasBack, &count, &date, &sbname)
	if err != nil {
		return s, err
	}
	s.SongbookID = sbid.Int64
	s.Number = int(number.Int64)
	s.Title = title.String
	s.Category = int(category.Int64)
	s.Tune, s.WordsBy, s.MusicBy = tune.String, words.String, music.String
	if withText {
		s.Text = text.String
	}
	s.Notes = notes.String
	s.UsePrivate = qt.ParseBool(usePrivate)
	if alignV.Valid {
		s.AlignV = int(alignV.Int64)
	}
	if alignH.Valid {
		s.AlignH = int(alignH.Int64)
	}
	if color.Valid {
		s.Color = qt.RGBToHex(color.Int64)
	}
	if f, ok := qt.ParseFont(font.String); ok {
		s.Font = f
	}
	if infoColor.Valid {
		s.InfoColor = qt.RGBToHex(infoColor.Int64)
	}
	if f, ok := qt.ParseFont(infoFont.String); ok {
		s.InfoFont = f
	}
	if endingColor.Valid {
		s.EndingColor = qt.RGBToHex(endingColor.Int64)
	}
	if f, ok := qt.ParseFont(endingFont.String); ok {
		s.EndingFont = f
	}
	s.UseBackground = qt.ParseBool(useBack)
	s.BackgroundName = backName.String
	s.HasBackground = qt.ParseBool(hasBack)
	s.Count = int(count.Int64)
	s.Date = date.String
	s.SongbookName = sbname.String
	return s, nil
}

// Songs returns every song without its text (for the song table).
func (r *Repo) Songs() ([]Song, error) {
	rows, err := r.DB.Query("SELECT " + songCols + " FROM Songs s LEFT JOIN Songbooks b ON b.id = s.songbook_id ORDER BY s.id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Song{}
	for rows.Next() {
		s, err := scanSong(rows, false)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Song returns one song including its text.
func (r *Repo) Song(id int64) (Song, error) {
	row := r.DB.QueryRow("SELECT "+songCols+" FROM Songs s LEFT JOIN Songbooks b ON b.id = s.songbook_id WHERE s.id = ?", id)
	return scanSong(row, true)
}

// SongsOfBook returns all songs of a songbook including text (export).
func (r *Repo) SongsOfBook(songbookID int64) ([]Song, error) {
	rows, err := r.DB.Query("SELECT "+songCols+" FROM Songs s LEFT JOIN Songbooks b ON b.id = s.songbook_id WHERE s.songbook_id = ? ORDER BY s.number, s.id", songbookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Song{}
	for rows.Next() {
		s, err := scanSong(rows, true)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Background returns the private background image (JPEG bytes) of a song.
func (r *Repo) Background(id int64) ([]byte, error) {
	var b []byte
	err := r.DB.QueryRow("SELECT background FROM Songs WHERE id = ?", id).Scan(&b)
	return b, err
}

// SetBackground stores a private background image for a song.
func (r *Repo) SetBackground(id int64, name string, jpeg []byte) error {
	return r.DB.Tx(func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE Songs SET background_name = ?, background = ? WHERE id = ?", name, jpeg, id)
		return err
	})
}

func songArgs(s Song) []any {
	return []any{s.SongbookID, s.Number, strings.TrimSpace(s.Title), s.Category, s.Tune, s.WordsBy, s.MusicBy,
		strings.TrimSpace(strings.ReplaceAll(s.Text, "\r\n", "\n")), s.Notes, s.UsePrivate, s.AlignV, s.AlignH,
		qt.HexToRGB(s.Color), s.Font.String(), qt.HexToRGB(s.InfoColor), s.InfoFont.String(),
		qt.HexToRGB(s.EndingColor), s.EndingFont.String(), s.UseBackground, s.BackgroundName}
}

// Insert adds a new song and returns its id. background may be nil.
func (r *Repo) Insert(s Song, background []byte) (int64, error) {
	var id int64
	err := r.DB.Tx(func(tx *sql.Tx) error {
		args := append(songArgs(s), background)
		res, err := tx.Exec(`INSERT INTO Songs (songbook_id,number,title,category,tune,words,music,song_text,notes,
			use_private,alignment_v,alignment_h,color,font,info_color,info_font,ending_color,ending_font,use_background,background_name,background)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, args...)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	return id, err
}

// Update saves an existing song. When keepBackground is true the stored image is left alone.
func (r *Repo) Update(s Song, background []byte, keepBackground bool) error {
	return r.DB.Tx(func(tx *sql.Tx) error {
		if keepBackground {
			args := append(songArgs(s), s.ID)
			_, err := tx.Exec(`UPDATE Songs SET songbook_id=?,number=?,title=?,category=?,tune=?,words=?,music=?,song_text=?,notes=?,
				use_private=?,alignment_v=?,alignment_h=?,color=?,font=?,info_color=?,info_font=?,ending_color=?,ending_font=?,
				use_background=?,background_name=? WHERE id = ?`, args...)
			return err
		}
		args := append(songArgs(s), background, s.ID)
		_, err := tx.Exec(`UPDATE Songs SET songbook_id=?,number=?,title=?,category=?,tune=?,words=?,music=?,song_text=?,notes=?,
			use_private=?,alignment_v=?,alignment_h=?,color=?,font=?,info_color=?,info_font=?,ending_color=?,ending_font=?,
			use_background=?,background_name=?,background=? WHERE id = ?`, args...)
		return err
	})
}

func (r *Repo) Delete(id int64) error {
	return r.DB.Tx(func(tx *sql.Tx) error {
		_, err := tx.Exec("DELETE FROM Songs WHERE id = ?", id)
		return err
	})
}

// NextNumber returns the next free song number in a songbook (port of lastUser).
func (r *Repo) NextNumber(songbookID int64) (int, error) {
	var n sql.NullInt64
	err := r.DB.QueryRow("SELECT MAX(number) FROM Songs WHERE songbook_id = ?", songbookID).Scan(&n)
	if err != nil {
		return 1, err
	}
	if !n.Valid {
		return 1, nil
	}
	return int(n.Int64) + 1, nil
}

// FindMatching returns the id of the song in the database that matches the
// given songbook name, number and title (port of EditWidget::isInDatabase).
func (r *Repo) FindMatching(songbookName string, number int, title string) (int64, bool) {
	var id int64
	err := r.DB.QueryRow(`SELECT s.id FROM Songs s JOIN Songbooks b ON b.id = s.songbook_id
		WHERE b.name = ? AND s.number = ? AND trim(s.title) = ? ORDER BY s.id DESC LIMIT 1`,
		songbookName, number, strings.TrimSpace(title)).Scan(&id)
	return id, err == nil
}

// ---- song counter ---------------------------------------------------------

// AddCount increments the usage counter of a song and stamps today's date.
func (r *Repo) AddCount(id int64) error {
	if id <= 0 {
		return nil
	}
	d := time.Now().Format("01:02:2006")
	return r.DB.Tx(func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE Songs SET count = COALESCE(count,0) + 1, date = ? WHERE id = ?", d, id)
		return err
	})
}

// CountRow is one line of the song counter report.
type CountRow struct {
	ID       int64  `json:"id"`
	Songbook string `json:"songbook"`
	Number   int    `json:"number"`
	Title    string `json:"title"`
	Count    int    `json:"count"`
	Date     string `json:"date"`
}

func (r *Repo) Counts() ([]CountRow, error) {
	rows, err := r.DB.Query(`SELECT s.id, COALESCE(b.name,''), s.number, s.title, s.count, COALESCE(s.date,'')
		FROM Songs s LEFT JOIN Songbooks b ON b.id = s.songbook_id WHERE s.count > 0 ORDER BY s.count DESC, s.title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CountRow{}
	for rows.Next() {
		var c CountRow
		if err := rows.Scan(&c.ID, &c.Songbook, &c.Number, &c.Title, &c.Count, &c.Date); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repo) ResetCounts() error {
	return r.DB.Tx(func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE Songs SET count = 0, date = '' WHERE count > 0")
		return err
	})
}

func (r *Repo) ResetCount(id int64) error {
	return r.DB.Tx(func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE Songs SET count = 0, date = '' WHERE id = ?", id)
		return err
	})
}

// ---- full text search -----------------------------------------------------

// SearchType mirrors the original "Search Type" combo box.
type SearchType int

const (
	SearchContainsPhrase SearchType = iota
	SearchContainsWordPhrase
	SearchLineBegins
	SearchAnyWord
	SearchAllWords
)

// SearchResult is a song id plus the highlight regexp pattern used by the UI.
type SearchResult struct {
	IDs       []int64 `json:"ids"`
	Highlight string  `json:"highlight"`
}

// wordLeft/wordRight are Unicode aware word boundaries. Go's \b only knows
// ASCII letters, which breaks whole word search for Cyrillic, Armenian etc.
const (
	wordLeft  = `(?:^|[^\p{L}\p{N}_])`
	wordRight = `(?:$|[^\p{L}\p{N}_])`
	jsLeft    = `(?<![\p{L}\p{N}_])`
	jsRight   = `(?![\p{L}\p{N}_])`
)

// BuildSearchRegex builds the regular expression used by both song and bible
// full text search. The returned highlight pattern uses JavaScript syntax
// (flags prefix "(?i)" is handled by the UI).
func BuildSearchRegex(text string, t SearchType, lineAnchor string) (re *regexp.Regexp, highlight string, words []string, err error) {
	text = Clean(text)
	if text == "" {
		return nil, "", nil, errors.New("empty search")
	}
	words = strings.Fields(text)
	esc := make([]string, len(words))
	for i, w := range words {
		esc[i] = regexp.QuoteMeta(w)
	}
	phrase := strings.Join(esc, `\W*`)
	alt := strings.Join(esc, "|")
	var pat string
	switch t {
	case SearchContainsWordPhrase:
		pat = wordLeft + `(` + phrase + `)` + wordRight
		highlight = jsLeft + phrase + jsRight
	case SearchLineBegins:
		pat = lineAnchor + phrase
		highlight = phrase
	case SearchAnyWord, SearchAllWords:
		pat = wordLeft + `(` + alt + `)` + wordRight
		highlight = jsLeft + `(?:` + alt + `)` + jsRight
	default:
		pat = phrase
		highlight = phrase
	}
	re, err = regexp.Compile("(?i)" + pat)
	if err != nil {
		return nil, "", nil, err
	}
	return re, "(?i)" + highlight, words, nil
}

// ContainsAllWords reports whether text contains each word as a whole word.
func ContainsAllWords(text string, words []string) bool {
	for _, w := range words {
		re := regexp.MustCompile(`(?i)` + wordLeft + regexp.QuoteMeta(w) + wordRight)
		if !re.MatchString(text) {
			return false
		}
	}
	return true
}

// Search performs a full text search over all song texts.
func (r *Repo) Search(text string, t SearchType) (SearchResult, error) {
	re, hl, words, err := BuildSearchRegex(text, t, "\n")
	if err != nil {
		return SearchResult{IDs: []int64{}}, err
	}
	rows, err := r.DB.Query("SELECT id, song_text FROM Songs")
	if err != nil {
		return SearchResult{}, err
	}
	defer rows.Close()
	res := SearchResult{IDs: []int64{}, Highlight: hl}
	for rows.Next() {
		var id int64
		var txt sql.NullString
		if err := rows.Scan(&id, &txt); err != nil {
			return res, err
		}
		body := txt.String
		if t == SearchLineBegins {
			body = "\n" + body
		}
		if t == SearchAllWords {
			if ContainsAllWords(body, words) {
				res.IDs = append(res.IDs, id)
			}
		} else if re.MatchString(body) {
			res.IDs = append(res.IDs, id)
		}
	}
	return res, rows.Err()
}

// SortByNumber sorts songs by number then title.
func SortByNumber(list []Song) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Number != list[j].Number {
			return list[i].Number < list[j].Number
		}
		return list[i].Title < list[j].Title
	})
}

// Label returns "number title" used in schedules.
func (s Song) Label() string { return fmt.Sprintf("%d %s", s.Number, s.Title) }

// WordMatcher matches verses against a typed query.
//
//   - exact: the words must appear next to each other in the typed order;
//     otherwise they only all have to be present, in any order and place.
//   - fullWord: every word must match as a whole word (no partial words).
//
// Matching is always case insensitive.
type WordMatcher struct {
	Match     func(string) bool
	Highlight string // JavaScript regexp source with a "(?i)" prefix
}

// NewWordMatcher builds a WordMatcher for a query.
func NewWordMatcher(query string, exact, fullWord bool) (*WordMatcher, error) {
	words := strings.Fields(Clean(query))
	if len(words) == 0 {
		return nil, errors.New("empty search")
	}
	esc := make([]string, len(words))
	for i, w := range words {
		esc[i] = regexp.QuoteMeta(w)
	}
	return newMatcher(esc, exact, fullWord)
}

// NewPhoneticMatcher is NewWordMatcher for Latin typing against Cyrillic text:
// every typed word becomes a pattern for its Cyrillic spellings.
func NewPhoneticMatcher(query string, exact, fullWord bool) (*WordMatcher, error) {
	words := strings.Fields(Clean(query))
	if len(words) == 0 {
		return nil, errors.New("empty search")
	}
	pats := make([]string, 0, len(words))
	for _, w := range words {
		if p := translit.Pattern(w); p != "" {
			pats = append(pats, p)
		}
	}
	if len(pats) == 0 {
		return nil, errors.New("empty search")
	}
	return newMatcher(pats, exact, fullWord)
}

// newMatcher builds a matcher from already escaped word patterns.
func newMatcher(esc []string, exact, fullWord bool) (*WordMatcher, error) {
	wrap := func(p string) string {
		if fullWord {
			return wordLeft + `(?:` + p + `)` + wordRight
		}
		return p
	}
	jsWrap := func(p string) string {
		if fullWord {
			return jsLeft + `(?:` + p + `)` + jsRight
		}
		return p
	}
	m := &WordMatcher{}
	if exact {
		phrase := strings.Join(esc, `[^\p{L}\p{N}]+`)
		re, err := regexp.Compile("(?i)" + wrap(phrase))
		if err != nil {
			return nil, err
		}
		m.Match = re.MatchString
		m.Highlight = "(?i)" + jsWrap(phrase)
		return m, nil
	}
	res := make([]*regexp.Regexp, len(esc))
	for i, w := range esc {
		re, err := regexp.Compile("(?i)" + wrap(w))
		if err != nil {
			return nil, err
		}
		res[i] = re
	}
	m.Match = func(text string) bool {
		for _, re := range res {
			if !re.MatchString(text) {
				return false
			}
		}
		return true
	}
	m.Highlight = "(?i)" + jsWrap(strings.Join(esc, "|"))
	return m, nil
}
