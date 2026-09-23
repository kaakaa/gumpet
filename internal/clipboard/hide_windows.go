package clipboard

import (
	"os/exec"
	"syscall"
)

// createNoWindow keeps clip.exe, a console program, from flashing a console
// window: gumpet itself is built with -H=windowsgui and has none to lend it.
const createNoWindow = 0x08000000

func hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
