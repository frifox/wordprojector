//go:build !linux

package shell

// installLauncher is for Linux: elsewhere the system connects a window with its icon by itself.
func installLauncher([]byte) {}
