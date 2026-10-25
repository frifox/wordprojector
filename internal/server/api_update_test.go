package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wordprojector/internal/update"
)

// the whole path of an update: look at the releases, install one, see that it waits for a restart
func TestUpdateAPI(t *testing.T) {
	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	h := &zip.FileHeader{Name: "WordProjector.exe", Method: zip.Deflate}
	h.SetMode(0o755)
	w, _ := zw.CreateHeader(h)
	w.Write([]byte("new"))
	zw.Close()
	var gh *httptest.Server
	gh = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases":
			json.NewEncoder(w).Encode([]map[string]any{{"tag_name": "2026-03-04", "published_at": "2026-03-04T00:00:00Z",
				"assets": []map[string]any{{"name": "WordProjector-2026-03-04-windows-x64.zip", "browser_download_url": gh.URL + "/dl.zip"}}}})
		case "/dl.zip":
			w.Write(zbuf.Bytes())
		}
	}))
	defer gh.Close()

	exe := filepath.Join(t.TempDir(), "WordProjector.exe")
	os.WriteFile(exe, []byte("old"), 0o755)
	u := &update.Updater{Repo: "x/y", API: gh.URL, Current: "2026-01-01", Client: gh.Client(), GOOS: "windows", GOARCH: "amd64", Exe: exe, AllowTemp: true}
	s := &Server{mux: http.NewServeMux(), hub: newHub(), upd: &updateState{u: u}}
	s.updateRoutes()

	call := func(method, path, body string) (int, map[string]any) {
		rec := httptest.NewRecorder()
		s.mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	code, st := call("GET", "/api/update?refresh=1", "")
	if code != 200 || st["latest"] != "2026-03-04" || st["newer"] != true || st["supported"] != true {
		t.Fatalf("status %d %v", code, st)
	}
	if code, out := call("POST", "/api/update/install", `{"tag":"2027-01-01"}`); code == 200 {
		t.Fatalf("an unknown release was installed: %v", out)
	}
	if code, out := call("POST", "/api/update/install", `{"tag":"2026-03-04"}`); code != 200 {
		t.Fatalf("install: %d %v", code, out)
	}
	if got, _ := os.ReadFile(exe); string(got) != "new" {
		t.Fatalf("program = %q", got)
	}
	_, st = call("GET", "/api/update", "")
	if st["pending"] != "2026-03-04" || st["newer"] != false {
		t.Fatalf("after install: %v", st)
	}
}
