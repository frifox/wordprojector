//go:build darwin

package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The app bundle is copied whole and the Desktop link points at the copy.
func TestCopySelfAndShortcut(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "src", "Word Projector.app", "Contents", "MacOS", "WordProjector")
	os.MkdirAll(filepath.Dir(exe), 0o755)
	os.WriteFile(exe, []byte("program"), 0o755)
	os.WriteFile(filepath.Join(root, "src", "Word Projector.app", "Contents", "Info.plist"), []byte("plist"), 0o644)
	// as after a download: the quarantine flag is on the app
	if out, err := exec.Command("xattr", "-w", "com.apple.quarantine", "0081;00000000;Safari;", exe).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	dir := filepath.Join(root, "Applications")
	got, err := copySelf(exe, dir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "Word Projector.app", "Contents", "MacOS", "WordProjector")
	if got != want {
		t.Fatalf("exe = %s, want %s", got, want)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "Word Projector.app", "Contents", "Info.plist")); string(b) != "plist" {
		t.Fatalf("the bundle was not copied whole: %q", b)
	}
	if out, _ := exec.Command("xattr", got).Output(); strings.Contains(string(out), "com.apple.quarantine") {
		t.Fatal("the copy kept the quarantine flag, so macOS would translocate it")
	}
	t.Setenv("HOME", filepath.Join(root, "home"))
	os.MkdirAll(filepath.Join(root, "home", "Desktop"), 0o755)
	d, _, _, err := makeShortcuts(got, dir, Options{Desktop: true})
	if err != nil {
		t.Fatal(err)
	}
	if to, _ := os.Readlink(d); to != filepath.Join(dir, "Word Projector.app") {
		t.Fatalf("link goes to %q", to)
	}
}
