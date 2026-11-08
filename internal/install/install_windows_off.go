//go:build windows && !winsetup

package install

import "errors"

// The Windows builds cannot install or uninstall themselves for now: the code that does (copying the program, shortcuts, an
// uninstall entry, a helper that deletes things after the program has quit) is what some antivirus programs took for
// malware. install_windows.go has it, compiled in only with -tags winsetup. Everything below only says "not available".

const setupEnabled = false

const setupOffReason = "Installing is not available in the Windows build for now."

func hasStart() bool                                      { return false }
func defaultDir() (string, string)                        { return "", "" }
func platformInstallable(string) (bool, string)           { return false, setupOffReason }
func appPath(m Marker) string                             { return m.Dir }
func copySelf(exe, dir string) (string, error)            { return "", errors.New(setupOffReason) }
func dropStaleEntry()                                     {}
func deleteUninstallKey()                                 {}
func recoverMarker(string) (Marker, bool)                 { return Marker{}, false }
func pinnedShortcuts(string) []string                     { return nil }
func registerUninstall(exe, dir, version string) error    { return nil }
func oldCopyPlan(exe string) plan                         { return plan{} }
func app(p *plan, m Marker)                               {}
func runPlan(p plan) error                                { return errors.New(setupOffReason) }
func makeShortcuts(exe, dir string, o Options) (desktop, start, icon string, err error) { return }

// ApplyCopy, RunCleanup, RemoveOld and CleanTemp are the helpers of the install; the flags that start them do nothing here.
func ApplyCopy(args []string) int  { return 2 }
func RunCleanup(args []string) int { return 2 }
func RemoveOld(path string)        {}
func CleanTemp()                   {}
