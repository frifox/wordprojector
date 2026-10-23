package server

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"wordprojector/internal/db"
	"wordprojector/internal/live"
	"wordprojector/internal/settings"
)

// The Settings window works on a copy of the settings made when it was opened. Pressing OK there must not undo what
// other parts of the program saved by themselves meanwhile: the language, the order of buttons, what Books used.
func TestSettingsWindowDoesNotUndoOtherPreferences(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "wordprojector.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := live.New(d)
	if err != nil {
		t.Fatal(err)
	}
	s := New(e, nil, t.TempDir())

	call := func(method, path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		s.mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		if rec.Code != 200 {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		return rec
	}
	var stale settings.Settings // the copy the window was opened with
	json.Unmarshal(call("GET", "/api/settings", "").Body.Bytes(), &stale)

	call("PATCH", "/api/settings/app", `{"language":"de","bibleButtonOrder":["3","1"],"moduleSource":"custom","moduleUrls":{"bibles":"http://x/b.xml"},"splitters":{"main":0.4}}`)
	stale.General.DisplayOnStartUp = true // a change made in the window itself
	body, _ := json.Marshal(stale)
	call("PUT", "/api/settings", string(body))

	// as after a restart: what is in the database
	got, err := settings.Load(d)
	if err != nil {
		t.Fatal(err)
	}
	a := got.App
	if a.Language != "de" || got.Main.UITranslation != "de" {
		t.Errorf("language = %q / %q, want de", a.Language, got.Main.UITranslation)
	}
	if strings.Join(a.BibleButtonOrder, ",") != "3,1" || a.ModuleSource != "custom" || a.ModuleURLs["bibles"] != "http://x/b.xml" {
		t.Errorf("order %v, source %q, urls %v were put back", a.BibleButtonOrder, a.ModuleSource, a.ModuleURLs)
	}
	if a.Splitters["main"] != 0.4 {
		t.Errorf("splitters = %v", a.Splitters)
	}
	if !got.General.DisplayOnStartUp {
		t.Errorf("the window's own change was lost")
	}
}
