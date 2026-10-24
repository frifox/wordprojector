//go:build !darwin

package shell

import "unsafe"

// hideDockAndMenuBar is a macOS feature; elsewhere the display window already covers the taskbar.
func hideDockAndMenuBar(bool) {}

// bringToFront is needed on macOS only: the file dialogs of a background program open behind other windows there.
func bringToFront() {}

// nativeOpen is a macOS panel; elsewhere the dialogs of the toolkit are used.
func nativeOpen(unsafe.Pointer, string, []string, bool, bool) ([]string, bool) { return nil, true }

const haveNativeOpen = false
