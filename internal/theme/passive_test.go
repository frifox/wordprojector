package theme

import (
	"os"
	"path/filepath"
	"testing"

	"wordprojector/internal/db"
)

// Every theme has its own pool of pictures for the idle screen, and its own choice from it.
func TestPassivePicturesBelongToTheTheme(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "wordprojector.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	r := &Repo{DB: d, Dir: dir}
	a, err := r.Insert(Theme{Info: Info{Name: "A"}, BackgroundColor: "#000000"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.Insert(Theme{Info: Info{Name: "B"}, BackgroundColor: "#000000"})
	if err != nil {
		t.Fatal(err)
	}
	one, _ := r.AddImage(a, "passive", 1, "one", []byte("1"))
	two, _ := r.AddImage(a, "passive", 1, "two", []byte("22"))

	th, _ := r.Load(a)
	if th.Passive.ImageID != one {
		t.Fatalf("the first picture is chosen at once: %d, want %d", th.Passive.ImageID, one)
	}
	if other, _ := r.Load(b); other.Passive.ImageID != 0 {
		t.Fatalf("theme B has a picture of theme A: %d", other.Passive.ImageID)
	}
	if list, _ := r.Images(b, "passive", 1); len(list) != 0 {
		t.Fatalf("theme B's pool is %v", list)
	}

	th.Passive = Passive{ImageID: two, UseBackground: true, BackgroundFit: 2}
	if err := r.Update(th); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.Load(a); got.Passive != th.Passive {
		t.Fatalf("saved %+v, loaded %+v", th.Passive, got.Passive)
	}
	if img, _, _ := r.Background(a, "passive", 1); string(img) != "22" {
		t.Fatalf("the picture shown is %q", img)
	}

	// a copy has its own pictures and the same choice
	c, err := r.Duplicate(a, "C")
	if err != nil {
		t.Fatal(err)
	}
	cp, _ := r.Load(c)
	list, _ := r.Images(c, "passive", 1)
	if len(list) != 2 || cp.Passive.ImageID == 0 || cp.Passive.ImageID == two || !cp.Passive.UseBackground || cp.Passive.BackgroundFit != 2 {
		t.Fatalf("copy: %+v, pool %v", cp.Passive, list)
	}
	if img, _, _ := r.Background(c, "passive", 1); string(img) != "22" {
		t.Fatalf("the copy shows %q", img)
	}

	// no picture
	th.Passive = Passive{}
	if err := r.Update(th); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.Load(a); got.Passive.ImageID != 0 || got.Passive.UseBackground {
		t.Fatalf("after choosing none: %+v", got.Passive)
	}

	// deleting a theme removes its pictures, and only those
	before, _ := os.ReadDir(dir)
	if err := r.Delete(c); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadDir(dir)
	if len(after) != len(before)-2 {
		t.Fatalf("files %d -> %d", len(before), len(after))
	}
	if list, _ := r.Images(a, "passive", 1); len(list) != 2 {
		t.Fatalf("theme A lost pictures: %v", list)
	}
}
