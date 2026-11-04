//go:build windows

// Package sysutil holds the few operating system calls the updater and the installer share.
package sysutil

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

type shellExecuteInfo struct {
	cbSize       uint32
	fMask        uint32
	hwnd         uintptr
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     uintptr
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    uintptr
	dwHotKey     uint32
	hIcon        uintptr
	hProcess     syscall.Handle
}

// Run runs the program with the arguments as an administrator (Windows asks the user first) and waits for it.
func Run(exe string, args []string) error { return shellExec(exe, quote(args), true) }

// Start runs the program as an administrator without waiting; params is the command line after the program, as it is.
func Start(exe, params string) error { return shellExec(exe, params, false) }

func quote(args []string) string {
	q := make([]string, len(args))
	for i, a := range args {
		q[i] = syscall.EscapeArg(a)
	}
	return strings.Join(q, " ")
}

func shellExec(exe, params string, wait bool) error {
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	p, _ := syscall.UTF16PtrFromString(params)
	info := shellExecuteInfo{fMask: 0x40 /* SEE_MASK_NOCLOSEPROCESS */, lpVerb: verb, lpFile: file, lpParameters: p, nShow: 0 /* SW_HIDE */}
	info.cbSize = uint32(unsafe.Sizeof(info))
	r, _, callErr := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteExW").Call(uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == 1223 { // ERROR_CANCELLED
			return fmt.Errorf("administrator permission was not given")
		}
		return fmt.Errorf("cannot ask for administrator permission: %v", callErr)
	}
	defer syscall.CloseHandle(info.hProcess)
	if !wait {
		return nil
	}
	if ev, err := syscall.WaitForSingleObject(info.hProcess, syscall.INFINITE); err != nil || ev != syscall.WAIT_OBJECT_0 {
		return fmt.Errorf("the administrator step did not finish")
	}
	var code uint32
	if err := syscall.GetExitCodeProcess(info.hProcess, &code); err != nil {
		return err
	}
	if code != 0 && code != 1 { // 1 comes with an error text in a file the caller reads
		return fmt.Errorf("the administrator step stopped with code %d", code)
	}
	return nil
}
