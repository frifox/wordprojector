// Package appdata resolves where the application keeps its database and
// downloaded modules on each operating system.
package appdata

import (
	"os"
	"path/filepath"
	"runtime"
)

const AppName = "WordProjector"

// legacyName is what the program was called before; its folders are renamed to the current name on first use.
const legacyName = "WordsProjector"

// adopt renames the folder of the old name to the new one when only the old one exists, so the data follows the rename.
func adopt(oldDir, newDir string) {
	if _, err := os.Stat(newDir); err == nil {
		return
	}
	if fi, err := os.Stat(oldDir); err != nil || !fi.IsDir() {
		return
	}
	_ = os.Rename(oldDir, newDir)
}

// DBFileName is kept identical to the original application so that an
// existing SoftProjector database can be dropped in unchanged.
const DBFileName = "spData.sqlite"

var override string

// SetDir fixes the data directory (from the --data flag).
func SetDir(dir string) { override = dir }

// Dir returns the data directory, creating it when needed.
//
// Resolution order:
//  0. a directory set with SetDir
//  1. WORDPROJECTOR_DATA environment variable
//  2. a spData.sqlite next to the executable (portable mode)
//  3. the per-user application data directory for the platform
func Dir() (string, error) {
	if override != "" {
		return override, os.MkdirAll(override, 0o755)
	}
	if v := os.Getenv("WORDPROJECTOR_DATA"); v != "" {
		return v, os.MkdirAll(v, 0o755)
	}
	if v := os.Getenv("WORDSPROJECTOR_DATA"); v != "" { // the old name of the variable
		return v, os.MkdirAll(v, 0o755)
	}
	if exe, err := os.Executable(); err == nil {
		d := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(d, DBFileName)); err == nil {
			return d, nil
		}
	}
	var base string
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "Library", "Application Support")
	case "windows":
		base = os.Getenv("APPDATA")
		if base == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, "AppData", "Roaming")
		}
	default:
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, ".local", "share")
		}
	}
	d := filepath.Join(base, AppName)
	adopt(filepath.Join(base, legacyName), d)
	return d, os.MkdirAll(d, 0o755)
}

// Sub returns (and creates) a sub directory of the data dir, e.g. "BibleModules".
func Sub(name string) (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(d, name)
	return p, os.MkdirAll(p, 0o755)
}

// DocumentsDir is the folder for user-visible files, ~/Documents/WordProjector on every OS (the slide show
// pictures live in its SlideShows subfolder). It is created on demand.
func DocumentsDir(sub string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	adopt(filepath.Join(home, "Documents", legacyName), filepath.Join(home, "Documents", AppName))
	dir := filepath.Join(home, "Documents", AppName, sub)
	return dir, os.MkdirAll(dir, 0o755)
}
