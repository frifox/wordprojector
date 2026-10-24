package server

import (
	"encoding/json"
	"image"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wordprojector/internal/db"
	"wordprojector/internal/live"
)

// A folder's pictures can be taken with or without those of its subfolders (with them is how it always worked).
func TestFolderImagesWithAndWithoutSubfolders(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "wordprojector.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := live.New(d)
	if err != nil {
		t.Fatal(err)
	}
	s := New(e, nil, t.TempDir())
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub", "deeper"), 0o755)
	os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755)
	for _, p := range []string{"a.png", "b.png", "sub/c.png", "sub/deeper/d.png", ".hidden/e.png"} {
		f, _ := os.Create(filepath.Join(dir, p))
		png.Encode(f, image.NewRGBA(image.Rect(0, 0, 4, 4)))
		f.Close()
	}
	names := func(body string) string {
		rec := httptest.NewRecorder()
		s.mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/slideshows/folder-images", strings.NewReader(body)))
		if rec.Code != 200 {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
		var out []struct{ Name string }
		json.Unmarshal(rec.Body.Bytes(), &out)
		var got []string
		for _, o := range out {
			got = append(got, o.Name)
		}
		return strings.Join(got, ",")
	}
	q, _ := json.Marshal(dir)
	if got := names(`{"path":` + string(q) + `}`); got != "a.png,b.png,c.png,d.png" {
		t.Errorf("by default (with subfolders): %s", got)
	}
	if got := names(`{"path":` + string(q) + `,"recursive":true}`); got != "a.png,b.png,c.png,d.png" {
		t.Errorf("with subfolders: %s", got)
	}
	if got := names(`{"path":` + string(q) + `,"recursive":false}`); got != "a.png,b.png" {
		t.Errorf("without subfolders: %s", got)
	}
}
