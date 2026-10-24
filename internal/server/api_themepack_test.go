package server

import (
	"encoding/json"
	"image"
	"image/color"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"wordprojector/internal/db"
	"wordprojector/internal/live"
	"wordprojector/internal/slides"
)

// A theme is saved as a zip file and read back as a new theme with its pictures.
func TestThemePackThroughTheApi(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "wordprojector.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := live.New(d)
	if err != nil {
		t.Fatal(err)
	}
	s := New(e, nil, t.TempDir())
	call := func(path, body string) map[string]any {
		rec := httptest.NewRecorder()
		s.mux.ServeHTTP(rec, httptest.NewRequest("POST", path, strings.NewReader(body)))
		if rec.Code != 200 {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	list, _ := e.Themes.List()
	src := list[0].ID
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		img.Set(x, x, color.RGBA{200, 30, 30, 255})
	}
	if _, err := e.Themes.AddImage(src, "passive", 1, "red.png", slides.EncodeJPEG(img)); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "mytheme") // the .zip is added
	out := call("/api/theme-pack/export", `{"id":`+itoa(src)+`,"path":`+quote(file)+`}`)
	if out["ok"] != true || !strings.HasSuffix(out["path"].(string), ".zip") {
		t.Fatalf("export answered %v", out)
	}
	in := call("/api/theme-pack/import", `{"path":`+quote(out["path"].(string))+`}`)
	id := int64(in["id"].(float64))
	if id == src || in["name"] == list[0].Name {
		t.Fatalf("the import must be a new theme under a free name: %v", in)
	}
	if imgs, _ := e.Themes.Images(id, "passive", 1); len(imgs) != 1 || imgs[0].Name != "red.png" {
		t.Fatalf("pictures of the new theme: %+v", imgs)
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }
func quote(s string) string { b, _ := json.Marshal(s); return string(b) }
