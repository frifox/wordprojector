package settings

import (
	"path/filepath"
	"testing"

	"wordprojector/internal/db"
)

func TestRoundTrip(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "spData.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	s := Default()
	s.General.CurrentThemeID = 3
	s.General.DisplayScreen2 = 1
	s.Bible[0].Primary = "2"
	s.App.Window = WindowGeometry{10, 20, 300, 400}
	if err := Save(d, s); err != nil {
		t.Fatal(err)
	}
	s2, err := Load(d)
	if err != nil {
		t.Fatal(err)
	}
	if s2.General.CurrentThemeID != 3 || s2.General.DisplayScreen2 != 1 || s2.Bible[0].Primary != "2" || s2.App.Window.Width != 300 || s2.Bible[1].Operator != "same" {
		t.Fatalf("%+v", s2)
	}
}
