//go:build windows

package sysutil

import (
	"os/exec"
	"syscall"
	"time"
	"unsafe"
)

// Detach lets the started program go on when this one quits.
func Detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000 | 0x00000200} // CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP
}

// RawCommandLine sets the command line of the new process exactly as given (cmd does not understand the escaping Go
// would make). Call it after Detach.
func RawCommandLine(cmd *exec.Cmd, line string) { cmd.SysProcAttr.CmdLine = line }

// Hide keeps the window of a short helper program from showing.
func Hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

// WaitPID waits until the process has quit (or the time is up); a process that is already gone is not waited for.
func WaitPID(pid int, timeout time.Duration) {
	h, err := syscall.OpenProcess(0x00100000 /* SYNCHRONIZE */, false, uint32(pid))
	if err != nil {
		return
	}
	defer syscall.CloseHandle(h)
	_, _ = syscall.WaitForSingleObject(h, uint32(timeout.Milliseconds()))
}

// DeleteOnReboot asks Windows to delete a file that cannot be deleted now (a program that is running) at the next start.
func DeleteOnReboot(path string) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	_, _, _ = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW").Call(uintptr(unsafe.Pointer(p)), 0, 0x4 /* MOVEFILE_DELAY_UNTIL_REBOOT */)
}
