package slides

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"wordprojector/internal/db"
)

func writePNG(t *testing.T, path string, w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, x%h, color.RGBA{255, 0, 0, 255})
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	png.Encode(f, img)
	f.Close()
	b, _ := os.ReadFile(path)
	return b
}

// A linked slide keeps only its thumbnails: the picture is read from where it lies, and removing the slide show never
// touches that file. A slide that is not linked keeps a copy, as before.
func TestLinkedSlidesDoNotCopyThePicture(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "wordprojector.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	store := t.TempDir()
	r := &Repo{DB: d, Dir: store}
	pics := t.TempDir()
	linkedPath, copiedPath := filepath.Join(pics, "linked.png"), filepath.Join(pics, "copied.png")
	orig := writePNG(t, linkedPath, 300, 200)
	writePNG(t, copiedPath, 300, 200)
	prep := func(p string) *Images {
		img, err := Decode(p)
		if err != nil {
			t.Fatal(err)
		}
		return Prepare(img, DefaultSettings())
	}
	id, err := r.Save(ShowWithSlides{Show: Show{Name: "S"}, Slides: []Slide{
		{Name: "linked.png", Path: linkedPath, Temp: "a", Linked: true},
		{Name: "copied.png", Path: copiedPath, Temp: "b"},
	}}, map[string]*Images{"a": prep(linkedPath), "b": prep(copiedPath)})
	if err != nil {
		t.Fatal(err)
	}
	show, err := r.Get(id)
	if err != nil || len(show.Slides) != 2 {
		t.Fatalf("get: %v %+v", err, show.Slides)
	}
	lk, cp := show.Slides[0], show.Slides[1]
	if !lk.Linked || cp.Linked {
		t.Fatalf("linked flags: %v %v", lk.Linked, cp.Linked)
	}
	if lk.Width != 300 || lk.Height != 200 {
		t.Fatalf("the size of a linked picture is the size of the file: %dx%d", lk.Width, lk.Height)
	}
	entries, _ := os.ReadDir(store)
	have := map[string]bool{}
	for _, e := range entries {
		have[e.Name()] = true
	}
	for _, name := range []string{"1_s.jpg", "1_p.jpg"} {
		if !have[name] {
			t.Errorf("missing thumbnail %s in %v", name, have)
		}
	}
	if have["1.jpg"] {
		t.Errorf("the linked slide was copied")
	}
	if !have["2.jpg"] || !have["2_s.jpg"] || !have["2_p.jpg"] {
		t.Errorf("the other slide lost files: %v", have)
	}
	got, err := r.Image(lk.ID, "full")
	if err != nil || string(got) != string(orig) {
		t.Fatalf("the full picture is not read from the file: %v", err)
	}
	if small, err := r.Image(lk.ID, "small"); err != nil || len(small) == 0 {
		t.Fatalf("thumbnail: %v", err)
	}
	if err := r.Delete(id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(linkedPath); err != nil {
		t.Fatalf("removing the slide show deleted the original picture: %v", err)
	}
	if entries, _ := os.ReadDir(store); len(entries) != 0 {
		t.Errorf("files left in the store: %d", len(entries))
	}
	// a picture that has moved away shows nothing (no crash), and is reported with no size
	id2, _ := r.Save(ShowWithSlides{Show: Show{Name: "T"}, Slides: []Slide{{Name: "linked.png", Path: linkedPath, Temp: "a", Linked: true}}}, map[string]*Images{"a": prep(linkedPath)})
	os.Remove(linkedPath)
	moved, _ := r.Get(id2)
	if moved.Slides[0].Width != 0 {
		t.Errorf("a moved picture has a size: %+v", moved.Slides[0])
	}
	if b, err := r.Image(moved.Slides[0].ID, "full"); err == nil && len(b) > 0 {
		t.Errorf("a moved picture was read")
	}
}
