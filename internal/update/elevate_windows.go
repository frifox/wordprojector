//go:build windows

package update

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

// runElevated runs the program with the arguments as an administrator (Windows asks the user first) and waits for it.
func runElevated(exe string, args []string) error {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = syscall.EscapeArg(a)
	}
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	params, _ := syscall.UTF16PtrFromString(strings.Join(quoted, " "))
	info := shellExecuteInfo{fMask: 0x40 /* SEE_MASK_NOCLOSEPROCESS */, lpVerb: verb, lpFile: file, lpParameters: params, nShow: 0 /* SW_HIDE */}
	info.cbSize = uint32(unsafe.Sizeof(info))
	r, _, callErr := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteExW").Call(uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == 1223 { // ERROR_CANCELLED
			return fmt.Errorf("administrator permission was not given, so the program in %s could not be replaced", exe)
		}
		return fmt.Errorf("cannot ask for administrator permission: %v", callErr)
	}
	defer syscall.CloseHandle(info.hProcess)
	if ev, err := syscall.WaitForSingleObject(info.hProcess, syscall.INFINITE); err != nil || ev != syscall.WAIT_OBJECT_0 {
		return fmt.Errorf("the administrator step did not finish")
	}
	var code uint32
	if err := syscall.GetExitCodeProcess(info.hProcess, &code); err != nil {
		return err
	}
	if code != 0 && code != 1 { // 1 comes with an error text in the file
		return fmt.Errorf("the administrator step stopped with code %d", code)
	}
	return nil
}
