//go:build windows

package update

import (
	"os/exec"
	"syscall"
)

// detach lets the started program go on when this one quits.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000 | 0x00000200} // CREATE_NO_WINDOW (a detached cmd would open a console for ping) | CREATE_NEW_PROCESS_GROUP
}
