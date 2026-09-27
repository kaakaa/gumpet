// Package react decides how the pet moves when a message arrives, by how
// serious the message is.
//
// A balloon's colour says whether something went wrong, but only once someone
// looks at it. The pet sits at the edge of their sight, and a movement there
// is noticed before anything is read: a shiver for an error, a hop for a
// warning, a jump for joy at a success. Ordinary messages leave it be — a pet
// that bounced at everything would say nothing by it.
//
// The movement is an offset from where the pet would otherwise be, as a
// function of time. It is applied where the window is placed, so the pet is
// never clipped by its own window, and it never touches where the pet is
// walking: when it is over, the pet carries on exactly as before.
package react

import (
	"math"
	"time"

	"github.com/kaakaa/gumpet/internal/message"
)

// Kind is a way of reacting.
type Kind int

const (
	None Kind = iota
	// Shiver is a quick side-to-side tremble that dies away.
	Shiver
	// Hop is a small jump.
	Hop
	// Jump is a big one.
	Jump
)

// For is how the pet reacts to a message of the given level.
func For(level message.Level) Kind {
	switch level {
	case message.LevelError:
		return Shiver
	case message.LevelWarn:
		return Hop
	case message.LevelSuccess:
		return Jump
	}
	return None
}

// Stronger is whichever of a and b says more, for messages that arrive
// together: an error among them is what should be noticed.
func Stronger(a, b Kind) Kind {
	rank := map[Kind]int{None: 0, Hop: 1, Jump: 2, Shiver: 3}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

// The shapes, in logical pixels and time. Small, because the pet is a
// companion at the edge of the screen and not a notification that shouts.
const (
	shiverFor    = 600 * time.Millisecond
	shiverWidth  = 5.0
	shiverPerSec = 14.0
	hopFor       = 350 * time.Millisecond
	hopHeight    = 12.0
	jumpFor      = 550 * time.Millisecond
	jumpHeight   = 30.0
)

// Offset is how far the pet is from where it would be, t into a reaction of
// kind k. Up is negative, as on screen. done says the reaction is over, and
// the offset is then zero.
func Offset(k Kind, t time.Duration) (dx, dy float64, done bool) {
	switch k {
	case Shiver:
		if t >= shiverFor {
			return 0, 0, true
		}
		fade := 1 - float64(t)/float64(shiverFor)
		return shiverWidth * fade * math.Sin(2*math.Pi*shiverPerSec*t.Seconds()), 0, false
	case Hop:
		return arc(t, hopFor, hopHeight)
	case Jump:
		return arc(t, jumpFor, jumpHeight)
	}
	return 0, 0, true
}

// arc is a jump of the given height lasting d: up and back down, fastest at
// the ends and hanging at the top, the way something thrown does.
func arc(t, d time.Duration, height float64) (dx, dy float64, done bool) {
	if t >= d {
		return 0, 0, true
	}
	p := float64(t) / float64(d)
	return 0, -height * 4 * p * (1 - p), false
}
