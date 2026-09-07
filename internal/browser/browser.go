// Package browser opens a URL in whatever browser the desktop prefers.
package browser

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Open launches url. It returns as soon as the browser has been started, not
// when the page has loaded.
func Open(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open %s: %w", url, err)
	}
	// Nothing waits for the browser, so reap it rather than leaving a zombie.
	go func() { _ = cmd.Wait() }()
	return nil
}
