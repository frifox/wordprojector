//go:build !windows

package install

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The uninstall takes away files and folders after the program quit, and a folder only when nothing else is in it.
func TestRunShellPlan(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.txt")
	tree := filepath.Join(root, "tree")
	empty := filepath.Join(root, "empty")
	kept := filepath.Join(root, "kept")
	os.WriteFile(file, []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(tree, "sub"), 0o755)
	os.WriteFile(filepath.Join(tree, "sub", "b"), []byte("x"), 0o644)
	os.Mkdir(empty, 0o755)
	os.Mkdir(kept, 0o755)
	os.WriteFile(filepath.Join(kept, "mine.txt"), []byte("x"), 0o644)
	if err := runShellPlan(plan{Files: []string{file}, Trees: []string{tree}, EmptyDirs: []string{empty, kept}}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(tree); os.IsNotExist(err) {
			if _, err := os.Stat(empty); os.IsNotExist(err) {
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	for _, p := range []string{file, tree, empty} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Errorf("%s is still there", p)
		}
	}
	if _, err := os.Stat(filepath.Join(kept, "mine.txt")); err != nil {
		t.Errorf("a file in a folder that was only to go when empty was taken: %v", err)
	}
}
