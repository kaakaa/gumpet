//go:build !windows

package clipboard

import "os/exec"

// hide has nothing to do outside Windows: nothing there opens a window for a
// command that was not asked for one.
func hide(*exec.Cmd) {}
