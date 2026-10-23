package theme

import (
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

func (r *Repo) filePath(file string) string { return filepath.Join(r.Dir, file+".jpg") }

func (r *Repo) removeFile(file string) {
	if r.Dir != "" && file != "" {
		_ = os.Remove(r.filePath(file))
	}
}

// imageFiles lists the files of the list entries matching a condition.
func (r *Repo) imageFiles(where string, args ...any) []string {
	rows, err := r.DB.Query("SELECT COALESCE(file,'') FROM ThemeBackgrounds WHERE "+where, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var f string
		if rows.Scan(&f) == nil && f != "" {
			out = append(out, f)
		}
	}
	return out
}

// insertImage adds a list entry and stores its picture (as a file when Dir is set).
func (r *Repo) insertImage(id int64, kind string, disp int, name string, jpeg []byte, selected int) (int64, error) {
	if r.Dir == "" {
		res, err := r.DB.Exec("INSERT INTO ThemeBackgrounds (theme_id, kind, disp, name, data, selected) VALUES (?,?,?,?,?,?)", id, kind, disp, name, jpeg, selected)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	res, err := r.DB.Exec("INSERT INTO ThemeBackgrounds (theme_id, kind, disp, name, selected) VALUES (?,?,?,?,?)", id, kind, disp, name, selected)
	if err != nil {
		return 0, err
	}
	imgID, _ := res.LastInsertId()
	file := strconv.FormatInt(imgID, 10)
	if err := os.MkdirAll(r.Dir, 0o755); err == nil {
		err = os.WriteFile(r.filePath(file), jpeg, 0o644)
	}
	if err == nil {
		_, err = r.DB.Exec("UPDATE ThemeBackgrounds SET file=? WHERE id=?", file, imgID)
	}
	if err != nil {
		_, _ = r.DB.Exec("DELETE FROM ThemeBackgrounds WHERE id=?", imgID)
		r.removeFile(file)
		return 0, err
	}
	return imgID, nil
}

// ImageData returns the JPEG of one list entry.
func (r *Repo) ImageData(imgID int64) ([]byte, error) {
	var data []byte
	var file string
	if err := r.DB.QueryRow("SELECT data, COALESCE(file,'') FROM ThemeBackgrounds WHERE id=?", imgID).Scan(&data, &file); err != nil {
		return nil, err
	}
	if file != "" {
		return os.ReadFile(r.filePath(file))
	}
	return data, nil
}

// Image is one background image in a theme's list.
type Image struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Selected bool   `json:"selected"`
}

func (r *Repo) kindOK(kind string) (string, bool) {
	k := strings.ToLower(kind)
	_, ok := kindTable[k]
	return k, ok
}

// Images lists the background images of a slide kind on a display. An image set before lists existed (stored only
// in the single-image column) is added to the list the first time it is asked for.
func (r *Repo) Images(id int64, kind string, disp int) ([]Image, error) {
	k, ok := r.kindOK(kind)
	if !ok {
		return nil, errUnknownKind(kind)
	}
	var n int
	if err := r.DB.QueryRow("SELECT count(*) FROM ThemeBackgrounds WHERE theme_id=? AND kind=? AND disp=?", id, k, disp).Scan(&n); err != nil {
		return nil, err
	}
	if n == 0 {
		if b, name, err := r.Background(id, k, disp); err == nil && len(b) > 0 {
			if _, err := r.insertImage(id, k, disp, name, b, 1); err != nil {
				return nil, err
			}
		}
	}
	rows, err := r.DB.Query("SELECT id, name, selected FROM ThemeBackgrounds WHERE theme_id=? AND kind=? AND disp=? ORDER BY id", id, k, disp)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Image{}
	for rows.Next() {
		var im Image
		var name sql.NullString
		var sel int
		if err := rows.Scan(&im.ID, &name, &sel); err != nil {
			return nil, err
		}
		im.Name, im.Selected = name.String, sel != 0
		out = append(out, im)
	}
	return out, rows.Err()
}

// AddImage adds a JPEG to the list; the first image of a list is selected right away.
func (r *Repo) AddImage(id int64, kind string, disp int, name string, jpeg []byte) (int64, error) {
	k, ok := r.kindOK(kind)
	if !ok {
		return 0, errUnknownKind(kind)
	}
	existing, err := r.Images(id, k, disp)
	if err != nil {
		return 0, err
	}
	imgID, err := r.insertImage(id, k, disp, name, jpeg, 0)
	if err != nil {
		return 0, err
	}
	if len(existing) == 0 {
		return imgID, r.SelectImage(id, k, disp, imgID)
	}
	return imgID, nil
}

// SelectImage makes an image the one the slides use.
func (r *Repo) SelectImage(id int64, kind string, disp int, imgID int64) error {
	k, ok := r.kindOK(kind)
	if !ok {
		return errUnknownKind(kind)
	}
	var name sql.NullString
	if err := r.DB.QueryRow("SELECT name FROM ThemeBackgrounds WHERE id=? AND theme_id=? AND kind=? AND disp=?", imgID, id, k, disp).Scan(&name); err != nil {
		return err
	}
	data, err := r.ImageData(imgID)
	if err != nil {
		return err
	}
	if _, err := r.DB.Exec("UPDATE ThemeBackgrounds SET selected = (id = ?) WHERE theme_id=? AND kind=? AND disp=?", imgID, id, k, disp); err != nil {
		return err
	}
	return r.SetBackground(id, k, disp, name.String, data)
}

// RemoveImage deletes an image from the list. If it was the selected one the first remaining image takes over,
// or the slides go back to no background image.
func (r *Repo) RemoveImage(id int64, kind string, disp int, imgID int64) error {
	k, ok := r.kindOK(kind)
	if !ok {
		return errUnknownKind(kind)
	}
	var sel int
	if err := r.DB.QueryRow("SELECT selected FROM ThemeBackgrounds WHERE id=? AND theme_id=? AND kind=? AND disp=?", imgID, id, k, disp).Scan(&sel); err != nil {
		return err
	}
	files := r.imageFiles("id = ?", imgID)
	if _, err := r.DB.Exec("DELETE FROM ThemeBackgrounds WHERE id=?", imgID); err != nil {
		return err
	}
	for _, f := range files {
		r.removeFile(f)
	}
	if sel == 0 {
		return nil
	}
	var next int64
	err := r.DB.QueryRow("SELECT id FROM ThemeBackgrounds WHERE theme_id=? AND kind=? AND disp=? ORDER BY id LIMIT 1", id, k, disp).Scan(&next)
	if err == sql.ErrNoRows {
		return r.SetBackground(id, k, disp, "", nil)
	}
	if err != nil {
		return err
	}
	return r.SelectImage(id, k, disp, next)
}

type errUnknownKind string

func (e errUnknownKind) Error() string { return "unknown kind " + string(e) }

// RenameImage changes the name shown for an image. The name of the selected image is also kept in the single-image
// column used by the slides.
func (r *Repo) RenameImage(id int64, kind string, disp int, imgID int64, name string) error {
	k, ok := r.kindOK(kind)
	if !ok {
		return errUnknownKind(kind)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errEmptyName{}
	}
	var sel int
	if err := r.DB.QueryRow("SELECT selected FROM ThemeBackgrounds WHERE id=? AND theme_id=? AND kind=? AND disp=?", imgID, id, k, disp).Scan(&sel); err != nil {
		return err
	}
	if _, err := r.DB.Exec("UPDATE ThemeBackgrounds SET name=? WHERE id=?", name, imgID); err != nil {
		return err
	}
	if sel != 0 {
		_, err := r.DB.Exec("UPDATE "+kindTable[k]+" SET background_name=? WHERE theme_id=? AND disp=?", name, id, disp)
		return err
	}
	return nil
}

type errEmptyName struct{}

func (errEmptyName) Error() string { return "the name can't be empty" }

// Duplicate copies a theme under a new name and returns the id of the copy.
func (r *Repo) Duplicate(id int64, name string) (int64, error) { return r.duplicate(id, name, nil) }

// duplicate copies a theme and the background images of the slide kinds listed.
func (r *Repo) duplicate(id int64, name string, kinds []string) (int64, error) {
	var newID int64
	err := r.DB.Tx(func(tx *sql.Tx) error {
		res, err := tx.Exec("INSERT INTO Themes (name, comment, bg_color) SELECT ?, '', COALESCE(bg_color, '#000000') FROM Themes WHERE id = ?", name, id)
		if err != nil {
			return err
		}
		if newID, err = res.LastInsertId(); err != nil {
			return err
		}
		for _, table := range []string{"ThemePassive", "ThemeBible", "ThemeSong", "ThemeAnnounce"} {
			rows, err := tx.Query("SELECT name FROM pragma_table_info('" + table + "') WHERE name <> 'theme_id'")
			if err != nil {
				return err
			}
			var cols []string
			for rows.Next() {
				var c string
				if err := rows.Scan(&c); err != nil {
					rows.Close()
					return err
				}
				cols = append(cols, c)
			}
			rows.Close()
			list := strings.Join(cols, ", ")
			if _, err := tx.Exec("INSERT INTO "+table+" (theme_id, "+list+") SELECT ?, "+list+" FROM "+table+" WHERE theme_id = ?", newID, id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	// the list of background images (the files are copied too)
	rows, err := r.DB.Query("SELECT id, kind, disp, COALESCE(name,''), selected FROM ThemeBackgrounds WHERE theme_id = ? ORDER BY id", id)
	if err != nil {
		return newID, err
	}
	type entry struct {
		id         int64
		kind, name string
		disp, sel  int
	}
	var list []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.kind, &e.disp, &e.name, &e.sel); err != nil {
			rows.Close()
			return newID, err
		}
		list = append(list, e)
	}
	rows.Close()
	for _, e := range list {
		if !slices.Contains(kinds, e.kind) {
			continue
		}
		data, err := r.ImageData(e.id)
		if err != nil {
			continue
		}
		if _, err := r.insertImage(newID, e.kind, e.disp, e.name, data, e.sel); err != nil {
			return newID, err
		}
	}
	return newID, nil
}

// BackgroundMarker is the comment of the hidden theme that holds the background of the presentation when nothing is
// shown (Settings > Background). It belongs to the program, not to a theme: switching themes keeps it.
const BackgroundMarker = "wp:background"

// EnsureBackground returns the id of the hidden background theme. The first time it is made from the passive
// background of theme from, so what was on the screen stays.
func (r *Repo) EnsureBackground(from int64) (int64, error) {
	var id int64
	err := r.DB.QueryRow("SELECT id FROM Themes WHERE comment = ? ORDER BY id LIMIT 1", BackgroundMarker).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	id, err = r.duplicate(from, "Background", []string{"passive"})
	if err != nil {
		return 0, err
	}
	_, err = r.DB.Exec("UPDATE Themes SET comment = ? WHERE id = ?", BackgroundMarker, id)
	return id, err
}
