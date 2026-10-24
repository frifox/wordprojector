package theme

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

// A theme travels as a zip file: "theme.json" holds the look, "backgrounds/" the pictures of the theme.
const (
	packFormat     = 1
	packConfig     = "theme.json"
	packFolder     = "backgrounds/"
	maxPackConfig  = 1 << 20   // the settings of one theme are a few kilobytes
	maxPackPicture = 32 << 20  // one picture, as it comes out of the zip
	maxPackTotal   = 256 << 20 // all of them
	maxPackImages  = 200
)

type packImage struct {
	Name     string `json:"name"`
	File     string `json:"file"` // the name of the file in backgrounds/
	Selected bool   `json:"selected,omitempty"`
}

type packPassive struct {
	BackgroundFit int         `json:"backgroundFit"`
	UseBackground bool        `json:"useBackground"`
	Images        []packImage `json:"images"`
}

// packFile is theme.json.
type packFile struct {
	Format            int         `json:"format"`
	Name              string      `json:"name"`
	BackgroundColor   string      `json:"backgroundColor"`
	BackgroundOpacity int         `json:"backgroundOpacity"`
	Bible             Bible       `json:"bible"`
	Song              Song        `json:"song"`
	Passive           packPassive `json:"passive"`
}

// safeFile is a name for a picture inside the zip: no folders, no odd characters.
func safeFile(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if s == "" {
		s = "image"
	}
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

// Export packs a theme and the pictures it has for the idle screen into a zip file.
func (r *Repo) Export(id int64) (data []byte, name string, err error) {
	t, err := r.Load(id)
	if err != nil {
		return nil, "", err
	}
	images, err := r.Images(id, "passive", 1)
	if err != nil {
		return nil, "", err
	}
	pf := packFile{
		Format: packFormat, Name: t.Name, BackgroundColor: t.BackgroundColor, BackgroundOpacity: t.BackgroundOpacity,
		Bible: t.Bible, Song: t.Song,
		Passive: packPassive{BackgroundFit: t.Passive.BackgroundFit, UseBackground: t.Passive.UseBackground, Images: []packImage{}},
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i, img := range images {
		b, err := r.ImageData(img.ID)
		if err != nil {
			return nil, "", fmt.Errorf("the picture %q: %w", img.Name, err)
		}
		file := fmt.Sprintf("%02d-%s.jpg", i+1, safeFile(strings.TrimSuffix(img.Name, path.Ext(img.Name))))
		w, err := zw.Create(packFolder + file)
		if err != nil {
			return nil, "", err
		}
		if _, err := w.Write(b); err != nil {
			return nil, "", err
		}
		pf.Passive.Images = append(pf.Passive.Images, packImage{Name: img.Name, File: file, Selected: img.ID == t.Passive.ImageID})
	}
	cfg, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return nil, "", err
	}
	w, err := zw.Create(packConfig)
	if err != nil {
		return nil, "", err
	}
	if _, err := w.Write(cfg); err != nil {
		return nil, "", err
	}
	if err := zw.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), t.Name, nil
}

// uniqueName is name, or name with a number when a theme has it already.
func (r *Repo) uniqueName(name string) (string, error) {
	list, err := r.List()
	if err != nil {
		return "", err
	}
	taken := func(n string) bool {
		for _, t := range list {
			if strings.EqualFold(t.Name, n) {
				return true
			}
		}
		return false
	}
	if !taken(name) {
		return name, nil
	}
	for i := 2; ; i++ {
		if n := fmt.Sprintf("%s %d", name, i); !taken(n) {
			return n, nil
		}
	}
}

func readZipFile(f *zip.File, limit int64) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("%q is too big", f.Name)
	}
	return b, nil
}

// Import makes a new theme from a zip file made by Export. jpeg turns the bytes of a picture (in any format the
// program can read) into the JPEG kept for the theme. A theme with the name of an existing one gets a number.
func (r *Repo) Import(data []byte, jpeg func([]byte) ([]byte, error)) (int64, string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0, "", errors.New("this is not a theme file (a zip file with theme.json)")
	}
	files := map[string]*zip.File{}
	var total uint64
	for _, f := range zr.File {
		files[strings.ToLower(f.Name)] = f
		total += f.UncompressedSize64
	}
	if total > maxPackTotal {
		return 0, "", errors.New("the theme file is too big")
	}
	cf := files[packConfig]
	if cf == nil {
		return 0, "", errors.New("this is not a theme file: theme.json is missing")
	}
	raw, err := readZipFile(cf, maxPackConfig)
	if err != nil {
		return 0, "", err
	}
	pf := packFile{Format: packFormat, BackgroundColor: "#000000", BackgroundOpacity: 100}
	t := Default()
	pf.Bible, pf.Song = t.Bible, t.Song // what the file does not say is the default
	if err := json.Unmarshal(raw, &pf); err != nil {
		return 0, "", fmt.Errorf("theme.json is not valid: %w", err)
	}
	if pf.Format > packFormat {
		return 0, "", errors.New("this theme file is from a newer version of the program")
	}
	if len(pf.Passive.Images) > maxPackImages {
		return 0, "", errors.New("the theme file has too many pictures")
	}
	name := strings.TrimSpace(pf.Name)
	if name == "" {
		name = "Imported theme"
	}
	if name, err = r.uniqueName(name); err != nil {
		return 0, "", err
	}
	t.Name, t.BackgroundColor, t.BackgroundOpacity = name, colorOrBlack(pf.BackgroundColor), percent(pf.BackgroundOpacity)
	t.Bible, t.Song = pf.Bible, pf.Song
	id, err := r.Insert(t)
	if err != nil {
		return 0, "", err
	}
	fail := func(err error) (int64, string, error) {
		_ = r.Delete(id)
		return 0, "", err
	}
	var selected int64
	for _, pi := range pf.Passive.Images {
		f := files[strings.ToLower(packFolder+pi.File)]
		if f == nil || strings.Contains(pi.File, "/") || strings.Contains(pi.File, `\`) {
			continue // a picture the file lists but does not have is left out
		}
		b, err := readZipFile(f, maxPackPicture)
		if err != nil {
			return fail(err)
		}
		if b, err = jpeg(b); err != nil {
			return fail(fmt.Errorf("the picture %q cannot be read: %w", pi.Name, err))
		}
		imgID, err := r.AddImage(id, "passive", 1, pi.Name, b)
		if err != nil {
			return fail(err)
		}
		if pi.Selected {
			selected = imgID
		}
	}
	loaded, err := r.Load(id)
	if err != nil {
		return fail(err)
	}
	loaded.Passive.BackgroundFit = pf.Passive.BackgroundFit
	loaded.Passive.UseBackground = pf.Passive.UseBackground
	if selected > 0 {
		loaded.Passive.ImageID = selected
	}
	if err := r.Update(loaded); err != nil {
		return fail(err)
	}
	return id, name, nil
}
