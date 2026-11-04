//go:build !windows

// Package sysutil holds the few operating system calls the updater and the installer share.
package sysutil

import "fmt"

// Run is for Windows only.
func Run(exe string, args []string) error {
	return fmt.Errorf("administrator permission is only asked for on Windows")
}

// Start is for Windows only.
func Start(exe, params string) error {
	return fmt.Errorf("administrator permission is only asked for on Windows")
}
