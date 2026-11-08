//go:build !windows || !winsetup

package sysutil

// DeleteOnReboot is for the Windows installer only (build tag winsetup).
func DeleteOnReboot(path string) {}
