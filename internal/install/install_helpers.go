//go:build !windows || winsetup

package install

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"wordprojector/internal/sysutil"
)

// ApplyCopy is what the helper with administrator permission does: args are the program to copy, its checksum, the
// folder to copy it to and a file for an error text. It returns the exit code.
func ApplyCopy(args []string) int {
	if len(args) != 4 {
		return 2
	}
	src, want, dir, errFile := args[0], args[1], args[2], args[3]
	fail := func(err error) int {
		_ = os.WriteFile(errFile, []byte(err.Error()), 0o644)
		return 1
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return fail(err)
	}
	if sumOf(data) != want {
		return fail(errors.New("the program changed before it was installed"))
	}
	if err := writeFile(filepath.Join(dir, filepath.Base(src)), data, 0o755); err != nil {
		return fail(err)
	}
	return 0
}

// ---- what runs after the program has quit (Windows) -------------------------------------------------------------------

// cleanupJob is what the helper takes away once the program that started it has quit.
type cleanupJob struct {
	PID       int      `json:"pid"`
	Files     []string `json:"files"`
	Trees     []string `json:"trees"`
	EmptyDirs []string `json:"emptyDirs"`
	Registry  bool     `json:"registry"`
}

// RunCleanup is the helper: args[0] is the file that says what to take away. A running program cannot delete itself,
// so the helper is a copy of the program that waits for it to quit. It returns the exit code.
func RunCleanup(args []string) int {
	if len(args) != 1 {
		return 2
	}
	b, err := os.ReadFile(args[0])
	if err != nil {
		return 1
	}
	_ = os.Remove(args[0])
	var j cleanupJob
	if json.Unmarshal(b, &j) != nil {
		return 1
	}
	sysutil.WaitPID(j.PID, 30*time.Second)
	for _, f := range j.Files {
		removeRetry(f)
	}
	for _, t := range j.Trees {
		for i := 0; i < 20; i++ {
			if os.RemoveAll(t) == nil {
				break
			}
			time.Sleep(250 * time.Millisecond)
		}
	}
	for _, d := range j.EmptyDirs {
		_ = os.Remove(d) // only when nothing else is in it
	}
	if j.Registry {
		deleteUninstallKey()
	}
	sysutil.DeleteOnReboot(selfExe()) // this helper is a copy in the temporary folder
	return 0
}

// removeRetry deletes a file that may still be locked for a moment (the program that was running from it has just quit).
func removeRetry(path string) {
	for i := 0; i < 40; i++ {
		if err := os.Remove(path); err == nil || os.IsNotExist(err) {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// RemoveOld deletes the copy of the program an install started from; it is what the program started from the new
// place does, once the old one has quit.
func RemoveOld(path string) {
	if path == "" || !safe(path) || samePath(path, selfExe()) {
		return
	}
	removeRetry(path)
}

// CleanTemp removes the helpers an earlier uninstall left in the temporary folder (they could not delete themselves).
func CleanTemp() {
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "wordprojector-cleanup-*"))
	for _, m := range matches {
		if !samePath(m, selfExe()) {
			_ = os.Remove(m)
		}
	}
}
