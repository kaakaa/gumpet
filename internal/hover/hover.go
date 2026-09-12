// Package hover decides whether the cursor is holding the pet still.
//
// Hovering stops the pet so that it, or one of its balloons, is something you
// can aim at rather than a moving target. But a cursor left lying wherever it
// was last used is not aiming at anything, and a pet that wanders under one
// would otherwise stop there and never start again.
//
// So the hold is not about where the cursor is. It is about whether the cursor
// is being moved: a hold lasts only while the cursor has stirred recently, and
// lapses once it has sat still long enough that nobody is plainly reaching for
// the pet.
package hover

import "time"

// IdleLimit is how long a cursor may sit motionless before it stops holding
// the pet. Long enough to reach the pet and click it, short enough that a
// forgotten cursor does not pin the pet down for the afternoon.
//
// Opening the menu is what a deliberate hover leads to, and an open menu stops
// the pet by itself, so this only has to outlast the aiming.
const IdleLimit = 3 * time.Second

// MoveThreshold is how far the cursor must travel to count as having moved, in
// logical pixels.
//
// The cursor is reported relative to a window that moves with the pet, so its
// position on the screen is a sum of two numbers and carries the rounding of
// both. Anything smaller than this is that rounding rather than a hand.
const MoveThreshold = 2

// Tracker watches the cursor across ticks. Its zero value is ready to use.
type Tracker struct {
	x, y  float64
	known bool
	idle  time.Duration
}

// Update reports whether the pet should hold still. It takes the cursor's
// position on the *screen* — not within the window, which moves with the pet —
// whether the cursor is inside that window at all, whether it is over
// something worth stopping for, and how long has passed.
//
// The window matters because the system reports the cursor relative to it and
// stops updating that once the cursor leaves. What comes back from outside is
// wherever the cursor was when it left, which added to a window that has since
// moved looks exactly like a cursor being swept across the screen. So a
// reading is only believed while the cursor is inside.
func (t *Tracker) Update(x, y float64, inside, over bool, dt time.Duration) bool {
	switch {
	case !inside:
		// Nothing can be read out there, and nothing needs to be: a cursor
		// outside the window is not on the pet. Counting the time as stillness
		// is what makes a cursor that has been lying somewhere for a while fail
		// to catch the pet when the pet finally walks over it.
		t.known = false
		t.idle += dt

	case !t.known:
		// First reading since the cursor came back into view. However far it is
		// from where it was last seen, crossing that gap was not something this
		// tick watched happen, so it is not movement.
		t.x, t.y = x, y
		t.known = true
		t.idle += dt

	case abs(x-t.x) >= MoveThreshold || abs(y-t.y) >= MoveThreshold:
		t.x, t.y = x, y
		t.idle = 0

	default:
		t.idle += dt
	}
	return inside && over && t.idle < IdleLimit
}

// Reset forgets what the cursor was doing, for when it stops being watched at
// all — while the window is click-through, say, and reports nothing useful.
func (t *Tracker) Reset() {
	*t = Tracker{}
}

// Idle is how long the cursor has sat still.
func (t *Tracker) Idle() time.Duration { return t.idle }

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
