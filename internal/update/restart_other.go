//go:build !windows

package update

import (
	"os"
	"syscall"
)

// Restart replaces this process with exe, run with the same arguments and
// environment. Exec rather than start-and-exit keeps the same process, so a
// gumpet started from a terminal is still attached to it afterwards, and
// whatever started it at login still sees it running.
//
// It only returns on failure.
func Restart(exe string) error {
	return syscall.Exec(exe, os.Args, os.Environ())
}
