//go:build windows && winsetup

package sysutil

import (
	"syscall"
	"unsafe"
)

// DeleteOnReboot asks Windows to delete a file that cannot be deleted now (a program that is running) at the next start.
func DeleteOnReboot(path string) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	_, _, _ = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW").Call(uintptr(unsafe.Pointer(p)), 0, 0x4 /* MOVEFILE_DELAY_UNTIL_REBOOT */)
}
