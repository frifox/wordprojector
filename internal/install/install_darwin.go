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

// recoverMarker finds the install of a program that macOS runs from a temporary place (App Translocation: an app that
// was copied with the "downloaded" flag still on it). The record names the app in /Applications; when that is the same
// app, the flag is taken off it so that it starts from there the next time, and the program counts as installed.
func recoverMarker(exe string) (Marker, bool) {
	if !strings.Contains(exe, "/AppTranslocation/") {
		return Marker{}, false
	}
	m, ok := readMarker()
	if !ok {
		return Marker{}, false
	}
	installed, here := bundleOf(m.Exe), bundleOf(exe)
	if installed == "" || here == "" || filepath.Base(installed) != filepath.Base(here) {
		return Marker{}, false
	}
	if _, err := os.Stat(installed); err != nil {
		return Marker{}, false
	}
	_ = exec.Command("xattr", "-dr", "com.apple.quarantine", installed).Run()
	return m, true
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
	// a copy that kept the "downloaded" flag is run by macOS from a temporary read-only place (App Translocation),
	// which is not where it was installed (ditto copies the flag whatever its options say)
	_ = exec.Command("xattr", "-dr", "com.apple.quarantine", dst).Run()
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

func pinnedShortcuts(string) []string { return nil }


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
