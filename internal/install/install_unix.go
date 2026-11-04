//go:build !windows && !darwin

package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func hasStart() bool { return true }

func defaultDir() (string, string) {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "opt", "WordProjector"), "~/.local/opt"
}

func platformInstallable(string) (bool, string) { return true, "" }

func appPath(m Marker) string { return m.Dir }

func copySelf(exe, dir string) (string, error) {
	dst := filepath.Join(dir, filepath.Base(exe))
	if samePath(dst, exe) {
		return exe, nil
	}
	if err := copyFile(exe, dst, 0o755); err != nil {
		return "", fmt.Errorf("cannot write to %s: %w", dir, err)
	}
	return dst, nil
}

func desktopDir() string {
	if out, err := exec.Command("xdg-user-dir", "DESKTOP").Output(); err == nil {
		if d := strings.TrimSpace(string(out)); d != "" {
			return d
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop")
}

func desktopEntry(exe, icon string) string {
	s := "[Desktop Entry]\nType=Application\nName=Word Projector\nComment=Bible, songs and slide shows for projection\nExec=\"" + exe + "\"\nTerminal=false\nCategories=AudioVideo;Presentation;\n"
	if icon != "" {
		s += "Icon=" + icon + "\n"
	}
	return s
}

// makeShortcuts writes .desktop files: one on the Desktop and one in the applications menu.
func makeShortcuts(exe, dir string, o Options) (desktop, start, icon string, err error) {
	if !o.Desktop && !o.Start {
		return
	}
	if len(Icon) > 0 {
		icon = filepath.Join(dir, "wordprojector.png")
		if werr := os.WriteFile(icon, Icon, 0o644); werr != nil {
			icon = ""
		}
	}
	var errs []string
	write := func(path string) string {
		if werr := os.MkdirAll(filepath.Dir(path), 0o755); werr != nil {
			errs = append(errs, werr.Error())
			return ""
		}
		if werr := os.WriteFile(path, []byte(desktopEntry(exe, icon)), 0o755); werr != nil {
			errs = append(errs, werr.Error())
			return ""
		}
		return path
	}
	if o.Desktop {
		desktop = write(filepath.Join(desktopDir(), "Word Projector.desktop"))
	}
	if o.Start {
		home, _ := os.UserHomeDir()
		start = write(filepath.Join(home, ".local", "share", "applications", "wordprojector.desktop"))
	}
	if len(errs) > 0 {
		err = fmt.Errorf("shortcuts: %s", strings.Join(errs, "; "))
	}
	return
}

func dropStaleEntry() {}

func deleteUninstallKey() {}

func registerUninstall(exe, dir, version string) error { return nil }

// oldCopyPlan takes away the program file the installer was started from.
func oldCopyPlan(exe string) plan { return plan{Files: []string{exe}} }

func app(p *plan, m Marker) {
	if safe(m.Exe) {
		p.Files = append(p.Files, m.Exe)
	}
	if m.Icon != "" && safe(m.Icon) {
		p.Files = append(p.Files, m.Icon)
	}
	if safe(m.Dir) {
		p.EmptyDirs = append(p.EmptyDirs, m.Dir)
	}
}

func runPlan(p plan) error { return runShellPlan(p) }
