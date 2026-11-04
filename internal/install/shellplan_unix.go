//go:build !windows

package install

import (
	"os/exec"
	"strconv"

	"wordprojector/internal/sysutil"
)

// runShellPlan starts a shell that waits for this program to quit and then takes the things away; a folder that is
// only to go when empty is tried last, with rmdir.
func runShellPlan(p plan) error {
	n := len(p.Files) + len(p.Trees)
	delay := 2
	if p.Delay > 0 {
		delay = p.Delay
	}
	args := []string{"-c", `sleep ` + strconv.Itoa(delay) + `; n=$1; shift; i=0; for p in "$@"; do if [ $i -lt $n ]; then rm -rf -- "$p"; else rmdir -- "$p" 2>/dev/null; fi; i=$((i+1)); done`, "sh", strconv.Itoa(n)}
	args = append(args, p.Files...)
	args = append(args, p.Trees...)
	args = append(args, p.EmptyDirs...)
	cmd := exec.Command("sh", args...)
	sysutil.Detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
