package update

import (
	"os"
	"os/exec"
)

// Restart starts exe with the same arguments. Windows has no exec, so the
// caller exits once this returns; the new process was started after the old
// one let go of its address, so it can take it.
func Restart(exe string) error {
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Start()
}
