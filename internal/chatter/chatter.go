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

// Remark is one thing the pet can say.
type Remark struct {
	Text string
	// Title is the heading on the balloon, naming where the remark came from.
	// Empty for a saying out of a file, which came from nowhere in particular.
	Title string
	// At is when the feed says the entry appeared. Zero for a saying out of a
	// file, and for a feed that dated nothing — plenty do not.
	At time.Time
}

// Bundled is the list gumpet ships with, used when no file is configured or
// the configured one cannot be read.
func Bundled() []Remark { return Parse(bundled) }

// Parse reads a sayings file: one per line, blank lines ignored, and lines
// starting with # treated as comments so a list can explain itself.
func Parse(data []byte) []Remark {
	var out []Remark
	s := bufio.NewScanner(bytes.NewReader(data))
	// A saying is one line, but a long one; the default 64KiB token is far
	// more than enough and the scanner's own limit would truncate silently.
	s.Buffer(make([]byte, 0, 8192), 1<<20)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, Remark{Text: line})
	}
	return out
}

// Sayer decides when the next idle remark is due and picks it.
//
// The zero value says nothing, which is what an empty list should do: a
// misconfigured file leaves the pet quiet rather than crashing it.
type Sayer struct {
	// groups keeps each source's remarks apart, so that a source with thirty
	// headlines does not drown out one with three. A group is picked first and
	// a remark within it second, which makes every source equally likely
	// however much it has to say.
	groups   [][]Remark
	interval time.Duration
	rnd      *rand.Rand
	// wait counts down to the next remark.
	wait time.Duration
	// lastGroup and lastIndex are what was said most recently, so the same
	// line is not picked twice running. -1 before anything has been said.
	lastGroup, lastIndex int
}

// New returns a Sayer that offers one of remarks every interval or so. rnd
// makes both the timing and the choice reproducible from a test.
func New(remarks []Remark, interval time.Duration, rnd *rand.Rand) *Sayer {
	return NewGrouped([][]Remark{remarks}, interval, rnd)
}

// NewGrouped is [New] for remarks that come from several sources, each of
// which should be heard from as often as the others.
func NewGrouped(groups [][]Remark, interval time.Duration, rnd *rand.Rand) *Sayer {
	s := &Sayer{interval: interval, rnd: rnd, lastGroup: -1, lastIndex: -1}
	s.setGroups(groups)
	s.wait = s.nextWait()
	return s
}

// Tick advances the countdown by dt and reports what to say, if anything.
//
// quiet is whether the pet has nothing better to do. While it is false the
// countdown is held at a full interval rather than merely paused, so a remark
// never lands the instant a real message finishes: the pet waits the same
// amount of time after being spoken to as it would after speaking.
func (s *Sayer) Tick(dt time.Duration, quiet bool) (Remark, bool) {
	if s == nil || len(s.groups) == 0 || s.interval <= 0 {
		return Remark{}, false
	}
	if !quiet {
		s.wait = s.nextWait()
		return Remark{}, false
	}
	if s.wait -= dt; s.wait > 0 {
		return Remark{}, false
	}
	s.wait = s.nextWait()
	return s.pick(), true
}

// SetGroups swaps in new material, for sources that change under the pet —
// headlines from feeds, rather than a fixed file.
//
// The countdown is left alone: new material is not a reason to say something
// sooner. What was said last is forgotten, because it referred to a position
// in a list that no longer exists.
func (s *Sayer) SetGroups(groups [][]Remark) {
	if s == nil {
		return
	}
	var total int
	for _, g := range groups {
		total += len(g)
	}
	if total == 0 {
		return
	}
	s.setGroups(groups)
	s.lastGroup, s.lastIndex = -1, -1
}

// setGroups drops the empty ones, so that a feed which returned nothing does
// not take its turn and produce silence.
func (s *Sayer) setGroups(groups [][]Remark) {
	s.groups = s.groups[:0]
	for _, g := range groups {
		if len(g) > 0 {
			s.groups = append(s.groups, g)
		}
	}
}

// pick chooses a group, then a remark within it, avoiding the one said last.
// Every step is a single draw — nothing here retries until it likes the
// answer, so it cannot run long however the dice fall.
func (s *Sayer) pick() Remark {
	g := s.rnd.IntN(len(s.groups))
	// A group holding one remark can only repeat it. If that is what was just
	// said and there is somewhere else to go, go there instead.
	if g == s.lastGroup && len(s.groups[g]) == 1 && len(s.groups) > 1 {
		g = s.rnd.IntN(len(s.groups) - 1)
		if g >= s.lastGroup {
			g++
		}
	}

	group := s.groups[g]
	i := s.rnd.IntN(len(group))
	if g == s.lastGroup && len(group) > 1 {
		// Draw from the rest of this group by picking among them and stepping
		// over the one just said, which is uniform over the rest.
		i = s.rnd.IntN(len(group) - 1)
		if s.lastIndex >= 0 && i >= s.lastIndex {
			i++
		}
	}

	s.lastGroup, s.lastIndex = g, i
	return group[i]
}

func (s *Sayer) nextWait() time.Duration {
	spread := 2*s.rnd.Float64() - 1 // -1 to +1
	return time.Duration(float64(s.interval) * (1 + jitter*spread))
}
