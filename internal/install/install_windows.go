//go:build windows

package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"wordprojector/internal/sysutil"
)

const uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\WordProjector`

func hasStart() bool { return true }

func defaultDir() (string, string) {
	pf := os.Getenv("ProgramFiles")
	if pf == "" {
		pf = `C:\Program Files`
	}
	return filepath.Join(pf, "Word Projector"), "Program Files"
}

func platformInstallable(string) (bool, string) { return true, "" }

func appPath(m Marker) string { return m.Dir }

// copySelf puts the running program in dir; a folder like Program Files asks for administrator permission.
func copySelf(exe, dir string) (string, error) {
	dst := filepath.Join(dir, filepath.Base(exe))
	if samePath(dst, exe) {
		return exe, nil
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		return "", err
	}
	err = writeFile(dst, data, 0o755)
	if err != nil && os.IsPermission(err) {
		tmp, terr := os.MkdirTemp("", "wordprojector-install-")
		if terr != nil {
			return "", terr
		}
		defer os.RemoveAll(tmp)
		errFile := filepath.Join(tmp, "error.txt")
		if rerr := sysutil.Run(exe, []string{ApplyFlag, exe, sumOf(data), dir, errFile}); rerr != nil {
			return "", fmt.Errorf("%w, so the program could not be put in %s", rerr, dir)
		}
		if b, rerr := os.ReadFile(errFile); rerr == nil && len(b) > 0 {
			return "", fmt.Errorf("%s", strings.TrimSpace(string(b)))
		}
		err = nil
	}
	if err != nil {
		return "", fmt.Errorf("cannot write to %s: %w", dir, err)
	}
	return dst, nil
}

// makeShortcuts makes .lnk files with PowerShell (the folders come from Windows, so a redirected Desktop is found).
func makeShortcuts(exe, dir string, o Options) (desktop, start, icon string, err error) {
	if !o.Desktop && !o.Start {
		return
	}
	const script = `$ErrorActionPreference='Stop'; $w=New-Object -ComObject WScript.Shell; $exe=$env:WP_EXE
function mk($f){ $p=Join-Path ([Environment]::GetFolderPath($f)) 'Word Projector.lnk'; $s=$w.CreateShortcut($p); $s.TargetPath=$exe; $s.WorkingDirectory=(Split-Path $exe); $s.IconLocation=($exe+',0'); $s.Save(); $p }
if($env:WP_DESKTOP -eq '1'){ 'D:'+(mk 'Desktop') }
if($env:WP_START -eq '1'){ 'S:'+(mk 'Programs') }`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(), "WP_EXE="+exe, "WP_DESKTOP="+b2s(o.Desktop), "WP_START="+b2s(o.Start))
	sysutil.Hide(cmd)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	runErr := cmd.Run()
	for _, line := range strings.Split(out.String(), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "D:"):
			desktop = line[2:]
		case strings.HasPrefix(line, "S:"):
			start = line[2:]
		}
	}
	if runErr != nil {
		err = fmt.Errorf("shortcuts: %v %s", runErr, strings.TrimSpace(stderr.String()))
	}
	return
}

func b2s(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// registerUninstall adds the program to "Apps & features" for this user.
func registerUninstall(exe, dir, version string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, uninstallKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("uninstall entry: %w", err)
	}
	defer k.Close()
	for name, v := range map[string]string{
		"DisplayName":     "Word Projector",
		"DisplayVersion":  version,
		"Publisher":       "Word Projector",
		"InstallLocation": dir,
		"DisplayIcon":     exe,
		"UninstallString": `"` + exe + `" --uninstall`,
	} {
		if err := k.SetStringValue(name, v); err != nil {
			return fmt.Errorf("uninstall entry: %w", err)
		}
	}
	_ = k.SetDWordValue("NoModify", 1)
	_ = k.SetDWordValue("NoRepair", 1)
	return nil
}

// dropStaleEntry removes the uninstall entry when the program it points to is gone (deleted by hand), so that
// "Apps & features" does not keep an entry that can no longer be uninstalled. Another copy of the program running
// somewhere else cleans it up the next time it is started.
func dropStaleEntry() {
	k, err := registry.OpenKey(registry.CURRENT_USER, uninstallKey, registry.QUERY_VALUE)
	if err != nil {
		return
	}
	exe, _, err := k.GetStringValue("DisplayIcon")
	k.Close()
	if err != nil || exe == "" || samePath(exe, selfExe()) {
		return
	}
	if _, err := os.Stat(exe); os.IsNotExist(err) {
		_ = registry.DeleteKey(registry.CURRENT_USER, uninstallKey)
	}
}

// oldCopyPlan takes away the program file the installer was started from.
func oldCopyPlan(exe string) plan { return plan{Files: []string{exe}} }

// app adds the program and its folder (when nothing else is in it) to the plan.
func app(p *plan, m Marker) {
	if safe(m.Exe) {
		p.Files = append(p.Files, m.Exe)
	}
	if safe(m.Dir) {
		p.EmptyDirs = append(p.EmptyDirs, m.Dir)
		p.Elevate = !writable(m.Dir)
	}
}

// recoverMarker rebuilds the record of an install from the entry in "Apps & features" and the shortcuts in their usual
// places, for a program that is started from that entry.
func recoverMarker(exe string) (Marker, bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, uninstallKey, registry.QUERY_VALUE)
	if err != nil {
		return Marker{}, false
	}
	defer k.Close()
	icon, _, err := k.GetStringValue("DisplayIcon")
	if err != nil || !samePath(icon, exe) {
		return Marker{}, false
	}
	m := Marker{Exe: exe, Dir: filepath.Dir(exe)}
	if v, _, err := k.GetStringValue("DisplayVersion"); err == nil {
		m.Version = v
	}
	for _, f := range []struct {
		id  *windows.KNOWNFOLDERID
		dst *string
	}{{windows.FOLDERID_Desktop, &m.Desktop}, {windows.FOLDERID_Programs, &m.Start}} {
		if dir, err := windows.KnownFolderPath(f.id, 0); err == nil {
			if p := filepath.Join(dir, "Word Projector.lnk"); existing(p) != "" {
				*f.dst = p
			}
		}
	}
	return m, true
}

// pinnedShortcuts are the shortcuts of the program that the user pinned to the taskbar or the Start menu: Windows keeps
// them in folders of their own. A shortcut that has the program's file name in it is asked about its target.
func pinnedShortcuts(exe string) []string {
	base := os.Getenv("APPDATA")
	if base == "" || exe == "" {
		return nil
	}
	var candidates []string
	for _, sub := range []string{"TaskBar", "StartMenu"} {
		dir := filepath.Join(base, "Microsoft", "Internet Explorer", "Quick Launch", "User Pinned", sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".lnk") {
				continue
			}
			p := filepath.Join(dir, e.Name())
			if b, err := os.ReadFile(p); err == nil && lnkMentions(b, filepath.Base(exe)) {
				candidates = append(candidates, p)
			}
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	const script = `$w=New-Object -ComObject WScript.Shell; foreach($p in ($env:WP_LNKS -split '\|')){ try { $p + "` + "`" + `t" + $w.CreateShortcut($p).TargetPath } catch {} }`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(), "WP_LNKS="+strings.Join(candidates, "|"))
	sysutil.Hide(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var pinned []string
	for _, line := range strings.Split(string(out), "\n") {
		if p, target, ok := strings.Cut(strings.TrimSpace(line), "\t"); ok && samePath(target, exe) {
			pinned = append(pinned, p)
		}
	}
	return pinned
}

// deleteUninstallKey takes the program out of "Apps & features".
func deleteUninstallKey() { _ = registry.DeleteKey(registry.CURRENT_USER, uninstallKey) }

// runPlan starts a helper (a copy of this program in the temporary folder) that waits for this program to quit and then
// takes the things away. No cmd with a ping in front: that is how malware starts and deletes things.
func runPlan(p plan) error {
	tmp, err := os.CreateTemp("", "wordprojector-cleanup-*.exe")
	if err != nil {
		return err
	}
	helper := tmp.Name()
	tmp.Close()
	_ = os.Remove(helper) // the name only; the copy is written in its place
	if err := copyFile(selfExe(), helper, 0o755); err != nil {
		return err
	}
	job, _ := json.Marshal(cleanupJob{PID: os.Getpid(), Files: p.Files, Trees: p.Trees, EmptyDirs: p.EmptyDirs, Registry: p.Registry})
	jobFile := helper + ".json"
	if err := os.WriteFile(jobFile, job, 0o600); err != nil {
		return err
	}
	if p.Elevate {
		return sysutil.Start(helper, syscall.EscapeArg(CleanupFlag)+" "+syscall.EscapeArg(jobFile))
	}
	cmd := exec.Command(helper, CleanupFlag, jobFile)
	sysutil.Detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
