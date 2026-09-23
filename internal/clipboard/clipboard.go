// Package clipboard puts text on the desktop's clipboard.
//
// It runs the command each desktop already has for the job rather than taking
// on a dependency, the same way [browser] opens links. That leaves one thing
// worth getting right, and it is testable without a screen: which command to
// run, and what to feed it.
package clipboard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"
)

// Timeout bounds a copy. Every command below reads its input and returns at
// once — xclip and wl-copy fork to keep serving the selection — so this only
// matters when something is badly wrong, and then the pet should not wait on
// it.
const Timeout = 2 * time.Second

// ErrUnavailable is returned when this machine has none of the commands.
// Plenty of Linux desktops have neither xclip nor wl-copy installed, and that
// is a reason to say so in the log, not to fail.
var ErrUnavailable = errors.New("no clipboard command is installed")

// command is one way of writing to the clipboard.
type command struct {
	name string
	args []string
	// windows means the text is handed over the way clip.exe reads Unicode:
	// UTF-16 with a byte-order mark, and CRLF line endings. Handed UTF-8, it
	// reads the bytes in the console's code page and Japanese comes out as
	// mojibake.
	windows bool
	// env is added to the command's environment.
	env []string
}

// candidates lists the commands worth trying on goos, best first.
//
// On Linux the order depends on the session. A Wayland session may still have
// xclip, talking to XWayland, but only wl-copy reaches native Wayland windows;
// under X it is the other way round. Both are tried in either case, because
// what is installed matters more than what would be ideal.
func candidates(goos string, getenv func(string) string) []command {
	switch goos {
	case "darwin":
		// pbcopy decodes its input by the locale, and an app started from
		// Finder or at login has none: LANG is only set by a terminal. In the
		// C locale anything outside ASCII is dropped and the clipboard is left
		// empty, with nothing to say it went wrong. gumpet always writes
		// UTF-8, so it says so. LC_ALL because it outranks whatever else the
		// user has set.
		return []command{{name: "pbcopy", env: []string{"LC_ALL=en_US.UTF-8"}}}
	case "windows":
		return []command{{name: "clip", windows: true}}
	}
	wl := command{name: "wl-copy"}
	x := []command{
		{name: "xclip", args: []string{"-selection", "clipboard"}},
		{name: "xsel", args: []string{"--clipboard", "--input"}},
	}
	if getenv("WAYLAND_DISPLAY") != "" {
		return append([]command{wl}, x...)
	}
	return append(x, wl)
}

// pick returns the first candidate that is installed.
func pick(goos string, getenv func(string) string, lookPath func(string) (string, error)) (command, error) {
	var tried []string
	for _, c := range candidates(goos, getenv) {
		if _, err := lookPath(c.name); err == nil {
			return c, nil
		}
		tried = append(tried, c.name)
	}
	return command{}, fmt.Errorf("%w (looked for %s)", ErrUnavailable, strings.Join(tried, ", "))
}

// encode is what c is fed for text.
func encode(c command, text string) []byte {
	if !c.windows {
		return []byte(text)
	}
	// Normalise first, so a message that already had CRLF does not end up
	// with CRCRLF.
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\n", "\r\n")
	units := utf16.Encode([]rune(text))
	out := make([]byte, 0, 2+2*len(units))
	out = append(out, 0xff, 0xfe)
	for _, u := range units {
		out = append(out, byte(u), byte(u>>8))
	}
	return out
}

// Write replaces the clipboard's contents with text.
func Write(text string) error {
	c, err := pick(runtime.GOOS, os.Getenv, exec.LookPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.name, c.args...)
	cmd.Stdin = bytes.NewReader(encode(c, text))
	if len(c.env) > 0 {
		cmd.Env = append(os.Environ(), c.env...)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	hide(cmd)
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%s: %w: %s", c.name, err, msg)
		}
		return fmt.Errorf("%s: %w", c.name, err)
	}
	return nil
}
