//go:build !windows

package update

import "fmt"

func runElevated(exe string, args []string) error {
	return fmt.Errorf("administrator permission is only asked for on Windows")
}
