//go:build !windows

package shell

import "github.com/wailsapp/wails/v3/pkg/application"

// squareDisplayWindow and coverMonitor are for Windows 11: its windows get rounded corners and a thin border of their own.
func squareDisplayWindow(*application.WebviewWindow) {}
func coverMonitor(*application.WebviewWindow)        {}
