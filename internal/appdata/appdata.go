// Package appdata resolves where the application keeps its database and
// downloaded modules on each operating system.
package appdata

import (
	"os"
	"path/filepath"
	"runtime"
)

const AppName = "WordProjector"

// DBFileName is the database file in the data folder.
const DBFileName = "wordprojector.sqlite"

var override string

// SetDir fixes the data directory (from the --data flag).
func SetDir(dir string) { override = dir }

// Dir returns the data directory, creating it when needed.
//
// Resolution order:
//  0. a directory set with SetDir
//  1. WORDPROJECTOR_DATA environment variable
//  2. a database file next to the executable (portable mode)
//  3. the per-user application data directory for the platform
func Dir() (string, error) {
	if override != "" {
		return override, os.MkdirAll(override, 0o755)
	}
	if v := os.Getenv("WORDPROJECTOR_DATA"); v != "" {
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
	dir := filepath.Join(home, "Documents", AppName, sub)
	return dir, os.MkdirAll(dir, 0o755)
}
