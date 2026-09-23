// Package drag turns pressing, moving and releasing the mouse into a new
// position for the pet's stage.
//
// It is separate from the drawing so that the awkward part is testable: a
// press is not a drag until the cursor has moved far enough, and until then it
// might still turn out to be a click. Getting that wrong means either a menu
// that opens whenever you try to move the pet, or a pet that will not open its
// menu because your hand is not perfectly still.
package drag

import "math"

// Threshold is how far the cursor must travel before a press counts as a drag
// rather than a click. Small enough that moving the pet feels immediate, large
// enough that a hand shaking by a pixel still opens the menu.
const Threshold = 4.0

// Rect is a rectangle in monitor pixels. It matches layout.Rect, which this
// package deliberately does not import: the geometry here is arithmetic, and
// tying it to the drawing packages would be the only reason to.
type Rect struct {
	X, Y, W, H float64
}

// Tracker follows one press-to-release gesture.
//
// The zero value is ready, and means nothing is being dragged.
type Tracker struct {
	// pressed is whether the button is currently down on something draggable.
	pressed bool
	// dragging is whether it has moved far enough to be a drag. Once true it
	// stays true until the button is released: a drag that returns to where it
	// started is still a drag, and must not turn back into a click.
	dragging bool
	// startX and startY are where the press landed, in monitor pixels.
	startX, startY float64
	// originX and originY are where the stage was when the press landed, so
	// the stage moves with the cursor rather than jumping under it.
	originX, originY float64
	// fixed is a press on something that cannot be moved. See [Tracker.PressFixed].
	fixed bool
}

// Press notes a button going down at a point, with the stage where it is. The
// point is in monitor pixels, since the window moves with the pet and a point
// inside it does not stay still.
func (t *Tracker) Press(x, y float64, stage Rect) {
	t.pressed = true
	t.dragging = false
	t.fixed = false
	t.startX, t.startY = x, y
	t.originX, t.originY = stage.X, stage.Y
}

// PressFixed notes a button going down on something that cannot be moved: a
// pet roaming the whole screen has no stage to drag anywhere. The gesture is
// followed all the same, and never becomes a drag however far the cursor
// goes, so letting go is always a click.
//
// It exists because whether a press can drag must only decide what moving
// does, never whether the press is heard. The first version of dragging
// started no gesture at all on a fixed stage, and since the menu opens when a
// gesture ends, a pet roaming the whole screen could not be clicked.
func (t *Tracker) PressFixed(x, y float64) {
	t.pressed = true
	t.dragging = false
	t.fixed = true
	t.startX, t.startY = x, y
}

// Move reports where the stage should be now, and whether a drag is under way.
//
// Until the cursor has travelled [Threshold] it reports false and the stage is
// left alone, because the gesture may still turn out to be a click.
func (t *Tracker) Move(x, y float64, stage Rect, monitor Rect) (nx, ny float64, dragging bool) {
	if !t.pressed || t.fixed {
		return stage.X, stage.Y, false
	}
	if !t.dragging {
		if math.Abs(x-t.startX) < Threshold && math.Abs(y-t.startY) < Threshold {
			return stage.X, stage.Y, false
		}
		t.dragging = true
	}

	nx = t.originX + (x - t.startX)
	ny = t.originY + (y - t.startY)
	nx, ny = confine(nx, ny, stage.W, stage.H, monitor)
	return nx, ny, true
}

// Release ends the gesture and reports whether it was a drag. A press that
// never moved far enough returns false, which is the caller's signal that it
// was a click after all.
func (t *Tracker) Release() (wasDrag bool) {
	wasDrag = t.dragging
	t.pressed, t.dragging, t.fixed = false, false, false
	return wasDrag
}

// Pressed reports whether a gesture is in progress: the button went down on
// something draggable and has not come up yet. It is true from the press,
// before the cursor has moved far enough for [Tracker.Dragging] to be.
//
// Callers drive the gesture from this rather than from Dragging, which cannot
// become true until Move has been called — asking Dragging first is a loop
// that never starts.
func (t *Tracker) Pressed() bool { return t.pressed }

// Dragging reports whether the press has travelled far enough to count.
func (t *Tracker) Dragging() bool { return t.pressed && t.dragging }

// Cancel abandons the gesture without treating it as either a drag or a click.
func (t *Tracker) Cancel() { t.pressed, t.dragging, t.fixed = false, false, false }

// confine keeps the stage somewhere it can be got at again. The whole stage is
// kept on the monitor rather than merely some of it: a stage pushed half off
// the edge would leave the pet walking into a region that is not there, and
// one pushed fully off could not be dragged back.
func confine(x, y, w, h float64, monitor Rect) (float64, float64) {
	// A stage wider than the monitor has nowhere to go; pin it to the corner
	// rather than letting the arithmetic push it off the other side.
	maxX := math.Max(monitor.W-w, 0)
	maxY := math.Max(monitor.H-h, 0)
	return clamp(x, 0, maxX), clamp(y, 0, maxY)
}

func clamp(v, lo, hi float64) float64 {
	if hi < lo {
		return lo
	}
	return math.Min(math.Max(v, lo), hi)
}
