//go:build darwin

package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func hasStart() bool { return false }

func defaultDir() (string, string) { return "/Applications", "Applications" }

// bundleOf is the app bundle the program file is in ("" when it is not run from one).
func bundleOf(exe string) string {
	marker := ".app" + string(filepath.Separator) + "Contents" + string(filepath.Separator) + "MacOS" + string(filepath.Separator)
	if i := strings.Index(exe, marker); i >= 0 {
		return exe[:i+len(".app")]
	}
	return ""
}

func platformInstallable(exe string) (bool, string) {
	if bundleOf(exe) == "" {
		return false, "This is not the Word Projector app."
	}
	return true, ""
}

func appPath(m Marker) string { return bundleOf(m.Exe) }

// copySelf copies the app bundle into dir (with ditto, which keeps the signature) and returns the program inside it.
func copySelf(exe, dir string) (string, error) {
	bundle := bundleOf(exe)
	dst := filepath.Join(dir, filepath.Base(bundle))
	if samePath(dst, bundle) {
		return exe, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("cannot write to %s: %w", dir, err)
	}
	if err := os.RemoveAll(dst); err != nil {
		return "", fmt.Errorf("cannot replace %s: %w", dst, err)
	}
	if out, err := exec.Command("ditto", bundle, dst).CombinedOutput(); err != nil {
		return "", fmt.Errorf("cannot copy to %s: %v %s", dir, err, strings.TrimSpace(string(out)))
	}
	return dst + exe[len(bundle):], nil
}

// makeShortcuts links the app on the Desktop (there is no program menu to put it in; the Dock is the user's).
func makeShortcuts(exe, dir string, o Options) (desktop, start, icon string, err error) {
	if !o.Desktop {
		return
	}
	home, herr := os.UserHomeDir()
	if herr != nil {
		return "", "", "", herr
	}
	link := filepath.Join(home, "Desktop", "Word Projector")
	_ = os.Remove(link)
	if lerr := os.Symlink(bundleOf(exe), link); lerr != nil {
		return "", "", "", fmt.Errorf("shortcut: %w", lerr)
	}
	return link, "", "", nil
}

func dropStaleEntry() {}

func deleteUninstallKey() {}

func registerUninstall(exe, dir, version string) error { return nil }

// oldCopyPlan takes away the app bundle the installer was started from.
func oldCopyPlan(exe string) plan {
	if b := bundleOf(exe); b != "" {
		return plan{Trees: []string{b}}
	}
	return plan{}
}

func app(p *plan, m Marker) {
	if b := bundleOf(m.Exe); b != "" && safe(b) {
		p.Trees = append(p.Trees, b)
	}
}

// runPlan starts a shell that waits for this program to quit and then takes the things away.
func runPlan(p plan) error { return runShellPlan(p) }
