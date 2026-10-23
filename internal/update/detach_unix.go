//go:build !windows

package update

import (
	"os/exec"
	"syscall"
)

// detach lets the started program go on when this one quits.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
