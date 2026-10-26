//go:build linux

package shell

import (
	"os"
	"path/filepath"
	"strings"
)

// launcherMark is in the launcher this program writes: a launcher without it was made or edited by the user and is left alone.
const launcherMark = "X-WordProjector-Launcher=1"

// installLauncher puts a launcher (a .desktop file and the icon) in the user's applications folder, pointing at this program.
// The dock and the application menu of GNOME and others connect a window with its icon through such a file; a program that is
// only unpacked and run has none, so its windows get a generic icon. It is renewed when the program is moved.
func installLauncher(icon []byte) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	// a program from a package has its own launcher; one run from a build folder is not worth one
	if strings.HasPrefix(exe, "/usr/") || strings.HasPrefix(exe, "/opt/") || strings.Contains(exe, "go-build") || strings.HasPrefix(exe, os.TempDir()+"/") {
		return
	}
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		data = filepath.Join(home, ".local", "share")
	}
	writeLauncher(exe, data, icon)
}

// writeLauncher writes the launcher of the program exe into the data folder (~/.local/share).
func writeLauncher(exe, data string, icon []byte) {
	iconPath := filepath.Join(data, "icons", "wordprojector.png")
	desktopPath := filepath.Join(data, "applications", "wordprojector.desktop")
	if old, err := os.ReadFile(desktopPath); err == nil && !strings.Contains(string(old), launcherMark) {
		return
	}
	entry := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=Word Projector\n" +
		"Comment=Church projection software (songs, Bible, slide shows)\n" +
		"Exec=" + execQuote(exe) + "\n" +
		"Icon=" + iconPath + "\n" +
		"Terminal=false\n" +
		"Categories=Office;Presentation;\n" +
		"StartupWMClass=WordProjector\n" +
		launcherMark + "\n"
	if old, err := os.ReadFile(desktopPath); err == nil && string(old) == entry {
		if _, err := os.Stat(iconPath); err == nil {
			return // already as it should be
		}
	}
	if os.MkdirAll(filepath.Dir(iconPath), 0o755) != nil || os.MkdirAll(filepath.Dir(desktopPath), 0o755) != nil {
		return
	}
	if len(icon) > 0 {
		_ = os.WriteFile(iconPath, icon, 0o644)
	}
	_ = os.WriteFile(desktopPath, []byte(entry), 0o644)
}

// execQuote writes a path as the Exec of a launcher: in quotes, with the characters the format reserves escaped.
func execQuote(p string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", `\$`)
	return `"` + r.Replace(p) + `"`
}
