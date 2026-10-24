package theme

import (
	"archive/zip"
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"wordprojector/internal/db"
)

func packRepo(t *testing.T) *Repo {
	d, err := db.Open(filepath.Join(t.TempDir(), "wordprojector.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	return &Repo{DB: d, Dir: t.TempDir()}
}

func asIs(b []byte) ([]byte, error) { return b, nil }

// A theme exported to a zip file and imported again is the same theme with the same pictures, under a free name.
func TestThemePackRoundTrip(t *testing.T) {
	r := packRepo(t)
	th := Default()
	th.Name = "Warm"
	th.BackgroundColor = "#336699"
	th.BackgroundOpacity = 60
	th.Song.TextColor = "#ffee00"
	id, err := r.Insert(th)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = r.AddImage(id, "passive", 1, "one.png", []byte("111"))
	two, _ := r.AddImage(id, "passive", 1, "two.png", []byte("2222"))
	cur, _ := r.Load(id)
	cur.Passive.ImageID, cur.Passive.UseBackground, cur.Passive.BackgroundFit = two, true, 2
	if err := r.Update(cur); err != nil {
		t.Fatal(err)
	}

	data, name, err := r.Export(id)
	if err != nil || name != "Warm" {
		t.Fatalf("export: %v, %q", err, name)
	}
	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	var entries []string
	for _, f := range zr.File {
		entries = append(entries, f.Name)
	}
	if len(entries) != 3 || !strings.Contains(strings.Join(entries, ","), "theme.json") || !strings.Contains(strings.Join(entries, ","), "backgrounds/01-one.jpg") {
		t.Fatalf("the zip holds %v", entries)
	}

	nid, nname, err := r.Import(data, asIs)
	if err != nil {
		t.Fatal(err)
	}
	if nname != "Warm 2" {
		t.Fatalf("a theme with that name exists: it should be numbered, got %q", nname)
	}
	got, _ := r.Load(nid)
	if got.BackgroundColor != "#336699" || got.BackgroundOpacity != 60 || got.Song.TextColor != "#ffee00" {
		t.Fatalf("the look was not kept: %+v %d %q", got.BackgroundColor, got.BackgroundOpacity, got.Song.TextColor)
	}
	if !got.Passive.UseBackground || got.Passive.BackgroundFit != 2 {
		t.Fatalf("the picture settings were not kept: %+v", got.Passive)
	}
	imgs, _ := r.Images(nid, "passive", 1)
	if len(imgs) != 2 || imgs[0].Name != "one.png" || imgs[1].Name != "two.png" {
		t.Fatalf("pictures: %+v", imgs)
	}
	if !imgs[1].Selected || got.Passive.ImageID != imgs[1].ID {
		t.Fatalf("the chosen picture is not the second: %+v, chosen %d", imgs, got.Passive.ImageID)
	}
	if b, _ := r.ImageData(imgs[1].ID); string(b) != "2222" {
		t.Fatalf("picture bytes %q", b)
	}
	// the original is untouched and the copy has pictures of its own
	if orig, _ := r.Images(id, "passive", 1); len(orig) != 2 || orig[1].ID == imgs[1].ID {
		t.Fatalf("original pictures: %+v", orig)
	}
}

func zipOf(files map[string]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for n, c := range files {
		w, _ := zw.Create(n)
		w.Write([]byte(c))
	}
	zw.Close()
	return buf.Bytes()
}

// Files that are not theme files are refused with a clear message, and nothing is left behind.
func TestThemePackRefusesOtherFiles(t *testing.T) {
	r := packRepo(t)
	before, _ := r.List()
	for name, data := range map[string][]byte{
		"not a zip":           []byte("hello"),
		"no theme.json":       zipOf(map[string]string{"readme.txt": "x"}),
		"broken json":         zipOf(map[string]string{"theme.json": "{"}),
		"newer format":        zipOf(map[string]string{"theme.json": `{"format": 99, "name": "x"}`}),
		"unreadable picture":  zipOf(map[string]string{"theme.json": `{"name":"x","passive":{"images":[{"name":"a","file":"a.jpg"}]}}`, "backgrounds/a.jpg": "junk"}),
	} {
		jpeg := asIs
		if name == "unreadable picture" {
			jpeg = func([]byte) ([]byte, error) { return nil, errUnknownKind("junk") }
		}
		if _, _, err := r.Import(data, jpeg); err == nil {
			t.Errorf("%s: imported", name)
		}
	}
	if after, _ := r.List(); len(after) != len(before) {
		t.Fatalf("themes were left behind: %d, was %d", len(after), len(before))
	}
	// a picture the file lists but does not have, and a file without a name, are fine
	if _, name, err := r.Import(zipOf(map[string]string{"THEME.JSON": `{"passive":{"images":[{"name":"gone","file":"gone.jpg"}]}}`}), asIs); err != nil || name != "Imported theme" {
		t.Fatalf("a bare theme file: %v %q", err, name)
	}
}
