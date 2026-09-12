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
// The window follows the pet around the screen, and the cursor position is
// read back through that moving frame, so a perfectly still cursor reports a
// pixel of jitter as the window shifts under it. Anything smaller than this is
// that jitter rather than a hand.
const MoveThreshold = 2

// Tracker watches the cursor across ticks. Its zero value is ready to use and
// treats the first position it sees as a movement, so a cursor already resting
// on the pet when gumpet starts still gets its moment.
type Tracker struct {
	x, y  float64
	known bool
	idle  time.Duration
}

// Update takes the cursor's position on the *screen* — not within the window,
// which moves with the pet — along with whether it is over something worth
// stopping for, and how long has passed. It reports whether the pet should
// hold still.
func (t *Tracker) Update(x, y float64, over bool, dt time.Duration) bool {
	if !t.known || abs(x-t.x) >= MoveThreshold || abs(y-t.y) >= MoveThreshold {
		t.x, t.y = x, y
		t.known = true
		t.idle = 0
	} else {
		t.idle += dt
	}
	return over && t.idle < IdleLimit
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
