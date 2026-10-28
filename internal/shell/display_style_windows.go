//go:build windows

package shell

import (
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	dwmapi                = syscall.NewLazyDLL("dwmapi.dll")
	procDwmSetWindowAttr  = dwmapi.NewProc("DwmSetWindowAttribute")
	procMonitorFromWindow = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW   = user32.NewProc("GetMonitorInfoW")
	procSetWindowPos      = user32.NewProc("SetWindowPos")
)

type winRect struct{ left, top, right, bottom int32 }

type monitorInfo struct {
	size    uint32
	monitor winRect
	work    winRect
	flags   uint32
}

// squareDisplayWindow takes the rounded corners and the thin border Windows 11 gives a window of its own: a display window has to
// cover its screen completely, with nothing of what is behind it showing at the edge. (Windows 10 has neither; the calls then do nothing.)
func squareDisplayWindow(w *application.WebviewWindow) {
	hwnd := uintptr(w.NativeWindow())
	if hwnd == 0 {
		return
	}
	set := func(attr, value uint32) {
		procDwmSetWindowAttr.Call(hwnd, uintptr(attr), uintptr(unsafe.Pointer(&value)), 4)
	}
	set(33, 1)          // DWMWA_WINDOW_CORNER_PREFERENCE = DWMWCP_DONOTROUND
	set(34, 0xFFFFFFFE) // DWMWA_BORDER_COLOR = DWMWA_COLOR_NONE
}

// coverMonitor puts the display window exactly over the whole monitor it is on, in device pixels, above everything (the sizes that
// are worked out in scaled units can be a pixel short at some scale factors).
func coverMonitor(w *application.WebviewWindow) {
	hwnd := uintptr(w.NativeWindow())
	if hwnd == 0 {
		return
	}
	mon, _, _ := procMonitorFromWindow.Call(hwnd, 2) // MONITOR_DEFAULTTONEAREST
	if mon == 0 {
		return
	}
	mi := monitorInfo{size: uint32(unsafe.Sizeof(monitorInfo{}))}
	if ok, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi))); ok == 0 {
		return
	}
	r := mi.monitor
	const hwndTopmost = ^uintptr(0) // (HWND)-1
	// SWP_NOACTIVATE 0x10 | SWP_FRAMECHANGED 0x20 | SWP_SHOWWINDOW 0x40
	procSetWindowPos.Call(hwnd, hwndTopmost, uintptr(r.left), uintptr(r.top), uintptr(r.right-r.left), uintptr(r.bottom-r.top), 0x10|0x20|0x40)
}
