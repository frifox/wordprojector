//go:build windows

package shell

import (
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	procCreateIconFromR = user32.NewProc("CreateIconFromResourceEx")
	procSendMessage     = user32.NewProc("SendMessageW")
)

// setDisplayWindowIcon gives a display window the icon of the displays (Alt+Tab, the task bar), not the one of the operator window.
func setDisplayWindowIcon(w *application.WebviewWindow) {
	hwnd := uintptr(w.NativeWindow())
	if hwnd == 0 || len(displayIconPNG) == 0 {
		return
	}
	// ICON_SMALL = 0 (16 px), ICON_BIG = 1 (32 px); Windows reads the PNG and scales it to the size asked
	for _, c := range []struct {
		kind uintptr
		px   uintptr
	}{{0, 16}, {1, 32}} {
		hicon, _, _ := procCreateIconFromR.Call(uintptr(unsafe.Pointer(&displayIconPNG[0])), uintptr(len(displayIconPNG)), 1, 0x00030000, c.px, c.px, 0)
		if hicon != 0 {
			procSendMessage.Call(hwnd, 0x0080 /* WM_SETICON */, c.kind, hicon)
		}
	}
}
