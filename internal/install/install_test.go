package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyCopy(t *testing.T) {
	src := filepath.Join(t.TempDir(), "WordProjector.exe")
	os.WriteFile(src, []byte("program"), 0o755)
	dir := filepath.Join(t.TempDir(), "Word Projector")
	errFile := filepath.Join(t.TempDir(), "err.txt")
	if code := ApplyCopy([]string{src, sumOf([]byte("other")), dir, errFile}); code == 0 {
		t.Fatal("a wrong checksum was accepted")
	}
	if b, _ := os.ReadFile(errFile); !strings.Contains(string(b), "changed") {
		t.Fatalf("error text = %q", b)
	}
	if code := ApplyCopy([]string{src, sumOf([]byte("program")), dir, errFile}); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "WordProjector.exe")); string(b) != "program" {
		t.Fatalf("copy = %q", b)
	}
}

func TestSafe(t *testing.T) {
	home, _ := os.UserHomeDir()
	for _, p := range []string{"", "/", ".", home} {
		if safe(p) {
			t.Errorf("%q counted as safe to delete", p)
		}
	}
	if !safe(filepath.Join(home, "Documents", "WordProjector")) {
		t.Error("a folder of the program is not safe")
	}
}

func TestLnkMentions(t *testing.T) {
	wide := []byte{}
	for _, c := range []byte("WordProjector.exe") {
		wide = append(wide, c, 0)
	}
	if !lnkMentions(append([]byte("Program Files|Word Projector|WordProjector.exe"), 0), "wordprojector.exe") || !lnkMentions(append([]byte("xx"), wide...), "WordProjector.exe") {
		t.Error("a shortcut with the name as plain text or as UTF-16 was not recognised")
	}
	if lnkMentions([]byte(`C:\Windows\notepad.exe`), "WordProjector.exe") || lnkMentions(nil, "") {
		t.Error("another program's shortcut was recognised")
	}
}
