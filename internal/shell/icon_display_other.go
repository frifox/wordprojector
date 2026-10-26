//go:build !windows && !(linux && cgo)

package shell

import "github.com/wailsapp/wails/v3/pkg/application"

// setDisplayWindowIcon: on macOS the Dock shows one icon for the program and a window has none of its own.
func setDisplayWindowIcon(*application.WebviewWindow) {}
