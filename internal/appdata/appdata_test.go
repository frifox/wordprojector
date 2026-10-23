package appdata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAdopt(t *testing.T) {
	base := t.TempDir()
	oldDir, newDir := filepath.Join(base, "WordsProjector"), filepath.Join(base, "WordProjector")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "spData.sqlite"), []byte("db"), 0o644); err != nil {
		t.Fatal(err)
	}
	adopt(oldDir, newDir)
	if _, err := os.Stat(filepath.Join(newDir, "spData.sqlite")); err != nil {
		t.Fatalf("data was not moved: %v", err)
	}
	if _, err := os.Stat(oldDir); err == nil {
		t.Fatal("old folder is still there")
	}
	// a folder under the new name is never replaced
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	adopt(oldDir, newDir)
	if _, err := os.Stat(oldDir); err != nil {
		t.Fatal("old folder was moved over an existing new one")
	}
}
