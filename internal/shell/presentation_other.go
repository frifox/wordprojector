//go:build !darwin

package shell

// hideDockAndMenuBar is a macOS feature; elsewhere the display window already covers the taskbar.
func hideDockAndMenuBar(bool) {}
