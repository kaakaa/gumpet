// Package display is what gumpet knows about the monitors attached to the
// machine.
//
// Enumerating them needs Ebitengine; deciding which one a setting names, and
// describing them to the settings page, do not. Those live here so that both
// can be had without a screen.
package display

import "fmt"

// Monitor is one display, as the settings page and the log describe it.
type Monitor struct {
	// Number is what stage.display is set to in order to choose this one,
	// counting from 1 the way a display arrangement does rather than the way a
	// slice does.
	Number int    `json:"number"`
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Label is the monitor as a person should see it named.
func (m Monitor) Label() string {
	return fmt.Sprintf("%d. %s (%d×%d)", m.Number, m.Name, m.Width, m.Height)
}

// Pick turns the stage.display setting into an index into the monitors the
// system reports.
//
// A machine can lose a monitor between one run and the next, so a number that
// no longer names one falls back to the first rather than failing: a pet on the
// wrong screen is a nuisance, a pet that will not start is worse.
func Pick(want, available int) int {
	if available <= 0 {
		return 0
	}
	if want < 1 || want > available {
		return 0
	}
	return want - 1
}
