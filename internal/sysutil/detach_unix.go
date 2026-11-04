//go:build !windows

package sysutil

import (
	"os/exec"
	"syscall"
	"time"
)

// Detach lets the started program go on when this one quits.
func Detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

// RawCommandLine is for Windows only.
func RawCommandLine(cmd *exec.Cmd, line string) {}

// Hide is for Windows only.
func Hide(cmd *exec.Cmd) {}

// WaitPID is for Windows only.
func WaitPID(pid int, timeout time.Duration) {}

// DeleteOnReboot is for Windows only.
func DeleteOnReboot(path string) {}
