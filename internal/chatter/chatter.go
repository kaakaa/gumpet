// Package chatter decides when the pet says something of its own accord, and
// what it says.
//
// It holds no timers and calls no clock: the game loop already has a tick and
// a delta, and hands them in. That keeps the whole of "how often does this
// happen, and does it repeat itself" testable without waiting for real seconds
// to pass, and without a screen.
package chatter

import (
	"bufio"
	"bytes"
	_ "embed"
	"math/rand/v2"
	"strings"
	"time"
)

// jitter is how much the wait either side of the configured interval may vary,
// as a fraction of it. Exactly ten minutes apart, every time, reads as a
// machine talking; a little unevenness reads as something idly muttering.
const jitter = 0.3

//go:embed sayings.txt
var bundled []byte

// Bundled is the list gumpet ships with, used when no file is configured or
// the configured one cannot be read.
func Bundled() []string { return Parse(bundled) }

// Parse reads a sayings file: one per line, blank lines ignored, and lines
// starting with # treated as comments so a list can explain itself.
func Parse(data []byte) []string {
	var out []string
	s := bufio.NewScanner(bytes.NewReader(data))
	// A saying is one line, but a long one; the default 64KiB token is far
	// more than enough and the scanner's own limit would truncate silently.
	s.Buffer(make([]byte, 0, 8192), 1<<20)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// Sayer decides when the next idle remark is due and picks it.
//
// The zero value says nothing, which is what an empty list should do: a
// misconfigured file leaves the pet quiet rather than crashing it.
type Sayer struct {
	sayings  []string
	interval time.Duration
	rnd      *rand.Rand
	// wait counts down to the next remark.
	wait time.Duration
	// last is the index said most recently, so the same line is not picked
	// twice running. -1 before anything has been said.
	last int
}

// New returns a Sayer that offers one of sayings every interval or so. rnd
// makes both the timing and the choice reproducible from a test.
func New(sayings []string, interval time.Duration, rnd *rand.Rand) *Sayer {
	s := &Sayer{sayings: sayings, interval: interval, rnd: rnd, last: -1}
	s.wait = s.nextWait()
	return s
}

// Tick advances the countdown by dt and reports what to say, if anything.
//
// quiet is whether the pet has nothing better to do. While it is false the
// countdown is held at a full interval rather than merely paused, so a remark
// never lands the instant a real message finishes: the pet waits the same
// amount of time after being spoken to as it would after speaking.
func (s *Sayer) Tick(dt time.Duration, quiet bool) (string, bool) {
	if s == nil || len(s.sayings) == 0 || s.interval <= 0 {
		return "", false
	}
	if !quiet {
		s.wait = s.nextWait()
		return "", false
	}
	if s.wait -= dt; s.wait > 0 {
		return "", false
	}
	s.wait = s.nextWait()
	return s.pick(), true
}

// pick chooses a saying, avoiding the one said last. With a single saying
// there is no choice to make, and repeating it is the only option.
func (s *Sayer) pick() string {
	if len(s.sayings) == 1 {
		s.last = 0
		return s.sayings[0]
	}
	// Draw from the others by picking among them and stepping over the one
	// just said, which is uniform over the rest and always terminates.
	i := s.rnd.IntN(len(s.sayings) - 1)
	if s.last >= 0 && i >= s.last {
		i++
	}
	s.last = i
	return s.sayings[i]
}

func (s *Sayer) nextWait() time.Duration {
	spread := 2*s.rnd.Float64() - 1 // -1 to +1
	return time.Duration(float64(s.interval) * (1 + jitter*spread))
}
