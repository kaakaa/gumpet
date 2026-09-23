// Package quiet decides when the pet keeps its messages to itself.
//
// gumpet exists to interrupt, and so there are times it must not: a meeting, a
// shared screen, the middle of the night. Quitting it was the only way to get
// that, and quitting meant losing whatever arrived meanwhile. A quiet window
// keeps gumpet running and receiving, and only stops it saying anything.
//
// Everything here takes the time as an argument rather than reading a clock,
// so the awkward cases — a window that crosses midnight, midnight itself, a
// window that ends while gumpet is not looking — are tested without waiting
// for any of them.
package quiet

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Window is a stretch of the day, in local time, when the pet stays quiet.
// The zero value is no window at all.
type Window struct {
	// from and to are minutes past midnight. to is exclusive: a window
	// ending at 08:30 is over at 08:30.
	from, to int
	set      bool
}

// Parse reads a window from two "HH:MM" times. Both empty is no window. So is
// a window that starts and ends at the same minute, which would otherwise have
// to mean either never or always, and neither is something anyone would set
// on purpose.
func Parse(from, to string) (Window, error) {
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	if from == "" && to == "" {
		return Window{}, nil
	}
	if from == "" || to == "" {
		return Window{}, fmt.Errorf("set both from and to, or neither")
	}
	f, err := clock(from)
	if err != nil {
		return Window{}, fmt.Errorf("from: %w", err)
	}
	t, err := clock(to)
	if err != nil {
		return Window{}, fmt.Errorf("to: %w", err)
	}
	if f == t {
		return Window{}, nil
	}
	return Window{from: f, to: t, set: true}, nil
}

// clock reads "HH:MM" as minutes past midnight. A single-digit hour is
// accepted, because "8:30" is how plenty of people write it.
func clock(s string) (int, error) {
	h, m, ok := strings.Cut(s, ":")
	if !ok || len(m) != 2 || len(h) < 1 || len(h) > 2 {
		return 0, fmt.Errorf("%q is not a time like 22:00", s)
	}
	hh, err := strconv.Atoi(h)
	if err != nil || hh < 0 || hh > 23 {
		return 0, fmt.Errorf("%q is not a time like 22:00", s)
	}
	mm, err := strconv.Atoi(m)
	if err != nil || mm < 0 || mm > 59 {
		return 0, fmt.Errorf("%q is not a time like 22:00", s)
	}
	return hh*60 + mm, nil
}

// Set reports whether there is a window at all.
func (w Window) Set() bool { return w.set }

// Contains reports whether t falls in the window, reading t in its own
// location. The caller passes local time; the window is a time of day where
// the person is, not anywhere else.
func (w Window) Contains(t time.Time) bool {
	if !w.set {
		return false
	}
	m := t.Hour()*60 + t.Minute()
	if w.from < w.to {
		return m >= w.from && m < w.to
	}
	// The window crosses midnight: it is the part of the day after from, and
	// the part before to.
	return m >= w.from || m < w.to
}

// Hush follows the window from one moment to the next, and keeps count of
// what arrived while it was quiet so that the end of it can say how much.
type Hush struct {
	window Window
	quiet  bool
	held   int
}

// SetWindow adopts a new window. Nothing is decided until the next [Hush.Update]:
// a window taken away while messages are being held has to end the quiet the
// same way the clock would, summary and all.
func (h *Hush) SetWindow(w Window) { h.window = w }

// Hold counts a message that arrived while it was quiet.
func (h *Hush) Hold() { h.held++ }

// Update looks at the time and reports whether it is quiet now. When a quiet
// stretch has just ended with messages held, it also returns how many, once;
// the count starts again from nothing.
//
// Starting up inside the window is simply quiet from the first call. Nothing
// ended, so nothing is reported.
func (h *Hush) Update(now time.Time) (quiet bool, ended int) {
	was := h.quiet
	h.quiet = h.window.Contains(now)
	if was && !h.quiet {
		ended, h.held = h.held, 0
	}
	return h.quiet, ended
}

// Quiet reports what the last [Hush.Update] found.
func (h *Hush) Quiet() bool { return h.quiet }

// Summary is what the pet says when a quiet stretch ends: how many arrived,
// and where to read them. It is one sentence however many there were, because
// a dozen balloons at once is a dozen balloons nobody reads.
func Summary(n int, messagesPage string) string {
	noun := "messages"
	if n == 1 {
		noun = "message"
	}
	return fmt.Sprintf("%d %s arrived while it was quiet.\n%s", n, noun, messagesPage)
}
