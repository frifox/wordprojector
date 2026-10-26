//go:build linux

package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteLauncher(t *testing.T) {
	data := t.TempDir()
	exe := "/home/me/Word Projector/WordProjector"
	writeLauncher(exe, data, []byte("png"))
	desktop := filepath.Join(data, "applications", "wordprojector.desktop")
	b, err := os.ReadFile(desktop)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{`Exec="/home/me/Word Projector/WordProjector"`, "Icon=" + filepath.Join(data, "icons", "wordprojector.png"), "StartupWMClass=WordProjector", launcherMark} {
		if !strings.Contains(got, want) {
			t.Fatalf("launcher lacks %q:\n%s", want, got)
		}
	}
	if icon, _ := os.ReadFile(filepath.Join(data, "icons", "wordprojector.png")); string(icon) != "png" {
		t.Fatal("the icon was not written")
	}

	// moved: the launcher follows
	writeLauncher("/opt2/WordProjector", data, []byte("png"))
	if b, _ := os.ReadFile(desktop); !strings.Contains(string(b), `Exec="/opt2/WordProjector"`) {
		t.Fatalf("not renewed:\n%s", b)
	}

	// a launcher the user made (no mark) is left alone
	os.WriteFile(desktop, []byte("[Desktop Entry]\nName=Mine\n"), 0o644)
	writeLauncher("/elsewhere/WordProjector", data, []byte("png"))
	if b, _ := os.ReadFile(desktop); string(b) != "[Desktop Entry]\nName=Mine\n" {
		t.Fatalf("a launcher of the user was changed:\n%s", b)
	}
}

func TestExecQuote(t *testing.T) {
	if got := execQuote(`/a b/$c"d`); got != `"/a b/\$c\"d"` {
		t.Fatal(got)
	}
}
