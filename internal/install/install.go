// Package install puts the program in the place programs belong on the operating system (Program Files, Applications,
// the user's local programs), makes shortcuts and takes it all away again.
package install

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"wordprojector/internal/appdata"
	"wordprojector/internal/sysutil"
)

// Icon is a PNG of the program's icon; where the system asks for a file (the Linux menu) it is written next to the program.
var Icon []byte

// ApplyFlag starts the arguments of the helper that runs with administrator permission and copies the program in.
const ApplyFlag = "--install-copy"

// CleanupFlag starts the helper that takes things away after the program has quit (Windows); its argument is a file
// that describes what to take away.
const CleanupFlag = "--cleanup"

// RemoveOldFlag makes a program that was started after an install delete the copy it was installed from.
const RemoveOldFlag = "--remove-old"

const markerName = "install.json"

// Marker is what the installer writes down: where the program was put and the shortcuts that were made, so that
// uninstalling takes away exactly those.
type Marker struct {
	Exe     string `json:"exe"`
	Dir     string `json:"dir"`
	Desktop string `json:"desktop,omitempty"`
	Start   string `json:"start,omitempty"`
	Icon    string `json:"icon,omitempty"`
	Version string `json:"version"`
}

// Info is what the windows and pages of installing show.
type Info struct {
	OS         string `json:"os"`
	CanInstall bool   `json:"canInstall"`
	Why        string `json:"why,omitempty"` // when it cannot be installed
	Installed  bool   `json:"installed"`
	Exe        string `json:"exe"`
	DefaultDir string `json:"defaultDir"`
	FolderName string `json:"folderName"` // "Program Files", "Applications"
	HasStart   bool   `json:"hasStart"`   // the system has a program menu to put a shortcut in
	// what an installed program would take away
	AppPath   string   `json:"appPath,omitempty"`
	Desktop   string   `json:"desktop,omitempty"`
	Start     string   `json:"start,omitempty"`
	Pinned    []string `json:"pinned,omitempty"` // shortcuts the user pinned to the taskbar or the Start menu (Windows)
	DataDir   string `json:"dataDir"`
	DocsDir   string `json:"docsDir"`
	Uninstall bool   `json:"uninstall"` // started from the uninstall entry of the system
}

// Options are the choices of the install form.
type Options struct {
	Dir     string `json:"dir"`
	Desktop bool   `json:"desktop"`
	Start   bool   `json:"start"`
}

// UninstallOptions say what to take away.
type UninstallOptions struct {
	App       bool `json:"app"`
	Shortcuts bool `json:"shortcuts"`
	Data      bool `json:"data"`
	Docs      bool `json:"docs"`
}

func selfExe() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return exe
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func markerPath() (string, error) {
	d, err := appdata.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, markerName), nil
}

func readMarker() (Marker, bool) {
	p, err := markerPath()
	if err != nil {
		return Marker{}, false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return Marker{}, false
	}
	var m Marker
	if json.Unmarshal(b, &m) != nil || m.Exe == "" {
		return Marker{}, false
	}
	return m, true
}

func writeMarker(m Marker) error {
	p, err := markerPath()
	if err != nil {
		return err
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	return os.WriteFile(p, b, 0o644)
}

func docsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Documents", appdata.AppName)
}

// installable says whether this running program may be moved: not a development build, not a portable copy.
func installable(exe string) (bool, string) {
	if exe == "" {
		return false, "The place of the running program is not known."
	}
	if strings.Contains(exe, string(filepath.Separator)+"go-build") || strings.HasPrefix(exe, os.TempDir()+string(filepath.Separator)) {
		return false, "This copy is run from a temporary folder (a development build)."
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(exe), appdata.DBFileName)); err == nil {
		return false, "This is a portable copy: its data is next to the program."
	}
	return platformInstallable(exe)
}

// current is the record of the install this running program belongs to. When the marker is lost (an uninstall that
// did not finish, a copied data folder) the system's own entry for the program (Windows) is the way back to it.
func current(exe string) (Marker, bool) {
	if m, ok := readMarker(); ok && samePath(m.Exe, exe) {
		return m, true
	}
	return recoverMarker(exe)
}

// Status describes the running program for the install and uninstall pages.
func Status(uninstallMode bool) Info {
	exe := selfExe()
	dropStaleEntry()
	in := Info{OS: runtime.GOOS, Exe: exe, Uninstall: uninstallMode, HasStart: hasStart()}
	in.DefaultDir, in.FolderName = defaultDir()
	in.CanInstall, in.Why = installable(exe)
	if d, err := appdata.Dir(); err == nil {
		in.DataDir = d
	}
	in.DocsDir = docsDir()
	if m, ok := current(exe); ok {
		in.Installed = true
		in.AppPath = appPath(m)
		in.Desktop, in.Start = existing(m.Desktop), existing(m.Start)
		in.Pinned = pinnedShortcuts(m.Exe)
	}
	return in
}

func existing(p string) string {
	if p == "" {
		return ""
	}
	if _, err := os.Lstat(p); err != nil {
		return ""
	}
	return p
}

// Result tells what an install did.
type Result struct {
	Exe     string `json:"exe"`     // the program to start from now on
	// RemoveOld is the copy the install started from, to be deleted by the program that starts from the new place
	// (Windows: a running program cannot delete itself, and a cmd that waits and deletes looks like malware)
	RemoveOld string `json:"-"`
	Warning string `json:"warning"` // what did not work and is not worth stopping for (a shortcut)
}

// Install copies the running program to the folder, makes the shortcuts and writes down what it did.
func Install(o Options, version string) (Result, error) {
	exe := selfExe()
	if ok, why := installable(exe); !ok {
		return Result{}, errors.New(why)
	}
	dir := strings.TrimSpace(o.Dir)
	if dir == "" {
		dir, _ = defaultDir()
	}
	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) {
		return Result{}, fmt.Errorf("the folder must be a full path: %s", dir)
	}
	newExe, err := copySelf(exe, dir)
	if err != nil {
		return Result{}, err
	}
	m := Marker{Exe: newExe, Dir: dir, Version: version}
	if old, ok := readMarker(); ok {
		removeShortcutFiles(old) // those of an earlier install
	}
	var warns []string
	d, s, icon, err := makeShortcuts(newExe, dir, o)
	if err != nil {
		warns = append(warns, err.Error())
	}
	m.Desktop, m.Start, m.Icon = d, s, icon
	if err := writeMarker(m); err != nil {
		warns = append(warns, err.Error())
	}
	if err := registerUninstall(newExe, dir, version); err != nil {
		warns = append(warns, err.Error())
	}
	// moved, not copied: the copy at the old place goes once it has quit
	res := Result{Exe: newExe}
	if !samePath(newExe, exe) {
		if p := oldCopyPlan(exe); safe(p.firstPath()) {
			if runtime.GOOS == "windows" {
				res.RemoveOld = exe
			} else if err := runPlan(p); err != nil {
				warns = append(warns, "the old copy was not removed: "+err.Error())
			}
		}
	}
	res.Warning = strings.Join(warns, "; ")
	return res, nil
}

func removeShortcutFiles(m Marker) {
	for _, p := range []string{m.Desktop, m.Start} {
		if p != "" {
			_ = os.Remove(p)
		}
	}
}

// plan is what an uninstall takes away once the program has quit.
type plan struct {
	Files     []string // single files and links
	Trees     []string // folders with everything in them
	EmptyDirs []string // folders that go only when nothing else is in them
	Registry  bool     // the uninstall entry of the system
	Elevate   bool     // a folder that only an administrator may change
	Delay     int      // seconds to wait for this program to quit first (0 is the usual two)
}

// firstPath is the one thing the plan takes away.
func (p plan) firstPath() string {
	if len(p.Trees) > 0 {
		return p.Trees[0]
	}
	if len(p.Files) > 0 {
		return p.Files[0]
	}
	return ""
}

func safe(p string) bool {
	p = filepath.Clean(p)
	if p == "" || p == "." || p == string(filepath.Separator) || filepath.Dir(p) == p {
		return false
	}
	if home, err := os.UserHomeDir(); err == nil && samePath(p, home) {
		return false
	}
	return true
}

// Uninstall takes away what the options say. The program's own files and the data it has open go after it has quit:
// the caller quits right after this returns.
func Uninstall(o UninstallOptions) error {
	m, ok := current(selfExe())
	if !ok {
		return errors.New("this program was not installed by the installer")
	}
	var p plan
	if o.Shortcuts {
		// the ones the installer made, and the ones the user pinned to the taskbar or the Start menu
		for _, f := range append([]string{m.Desktop, m.Start}, pinnedShortcuts(m.Exe)...) {
			if f != "" && safe(f) {
				p.Files = append(p.Files, f)
			}
		}
		m.Desktop, m.Start = "", ""
	}
	if o.App {
		app(&p, m)
		p.Registry = true
	}
	// only the folders the program made itself: its name is on them
	if o.Data {
		if d, err := appdata.Dir(); err == nil && filepath.Base(d) == appdata.AppName && safe(d) {
			p.Trees = append(p.Trees, d)
		}
	}
	if o.Docs {
		if d := docsDir(); d != "" && filepath.Base(d) == appdata.AppName && safe(d) {
			p.Trees = append(p.Trees, d)
		}
	}
	// what is written down changes only once the removal is under way: if it could not start (the administrator
	// question was declined) the program is still installed, and the record says so
	if err := runPlan(p); err != nil {
		return err
	}
	if mp, err := markerPath(); err == nil {
		if o.App {
			_ = os.Remove(mp)
		} else {
			_ = writeMarker(m)
		}
	}
	return nil
}

// ---- helpers shared by the system files -----------------------------------------------------------------------------

func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".wp-write-")
	if err != nil {
		return false
	}
	f.Close()
	_ = os.Remove(f.Name())
	return true
}

func sumOf(data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])
}

func copyFile(from, to string, mode os.FileMode) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return writeFile(to, data, mode)
}

// writeFile writes next to the target and renames over it, so a program that is running from there is replaced.
func writeFile(to string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	next := to + ".new"
	if err := os.WriteFile(next, data, mode); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		// a running program cannot be overwritten on Windows, but it can be renamed away
		if _, err := os.Stat(to); err == nil {
			_ = os.Remove(to + ".old")
			_ = os.Rename(to, to+".old")
		}
	}
	if err := os.Rename(next, to); err != nil {
		_ = os.Remove(next)
		return err
	}
	return nil
}

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

// lnkMentions says whether a shortcut file (.lnk) has the name in it, as plain bytes or as UTF-16 (a shortcut keeps the
// folders and the file of its target as separate names, so this only tells which shortcuts are worth asking Windows
// about).
func lnkMentions(data []byte, name string) bool {
	if name == "" {
		return false
	}
	want := []byte(strings.ToLower(name))
	wide := make([]byte, 0, len(want)*2)
	for _, c := range want {
		wide = append(wide, c, 0) // the names here are plain letters
	}
	low := make([]byte, len(data))
	for i, c := range data {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		low[i] = c
	}
	return bytes.Contains(low, want) || bytes.Contains(low, wide)
}
