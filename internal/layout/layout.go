// Package layout works out where gumpet's window goes on the monitor, and how
// the pet, its speech balloon and its menu sit inside that window.
//
// It is arithmetic only, deliberately free of any dependency on the renderer,
// so the placement rules can be tested without a display.
package layout

import (
	"math"

	"github.com/kaakaa/gumpet/internal/config"
)

// Rect is a rectangle in monitor pixels.
type Rect struct {
	X, Y, W, H float64
}

// Stage returns the area of the monitor the pet may move around in.
func Stage(s config.Stage, monitorW, monitorH int) Rect {
	full := Rect{W: float64(monitorW), H: float64(monitorH)}
	if s.Fullscreen {
		return full
	}

	w := math.Min(float64(s.Width), full.W)
	h := math.Min(float64(s.Height), full.H)
	mx, my := float64(s.MarginX), float64(s.MarginY)

	var x, y float64
	switch s.Anchor {
	case config.AnchorTopLeft:
		x, y = mx, my
	case config.AnchorTopRight:
		x, y = full.W-w-mx, my
	case config.AnchorBottomLeft:
		x, y = mx, full.H-h-my
	case config.AnchorCenter:
		x, y = (full.W-w)/2, (full.H-h)/2
	case config.AnchorCustom:
		x, y = float64(s.X), float64(s.Y)
	default: // AnchorBottomRight, and anything Validate would have rejected
		x, y = full.W-w-mx, full.H-h-my
	}
	return Rect{X: x, Y: y, W: w, H: h}
}

// Panel is whatever gumpet is showing above the pet — a speech balloon or the
// menu. A zero Panel means there is nothing to show.
type Panel struct {
	W, H float64
}

// Window is a placed window: where it goes on the monitor, and where the pet
// and its panel sit inside it.
type Window struct {
	// X, Y, W and H are the window itself, in monitor pixels.
	X, Y, W, H float64
	// PetX and PetY are the pet's top-left corner within the window.
	PetX, PetY float64
	// PanelX and PanelY are the panel's top-left corner within the window.
	PanelX, PanelY float64
	// TailX is where a balloon's tail should point, within the window.
	TailX float64
	// Below says the panel is under the pet rather than over it, because
	// there was no room above. Balloons stack downwards then, and the tail
	// points up. See [Orient].
	Below bool
}

// PlaceWindow sizes gumpet's window around the pet and whatever it is saying,
// and puts it on the monitor so the pet lands at (petX, petY).
//
// The window is only as big as it has to be. That is what keeps a click-eating
// rectangle from covering the screen, and it is why the window moves with the
// pet rather than the pet moving inside a fixed window.
func PlaceWindow(petX, petY, petW, petH float64, panel Panel, gap float64, monitor Rect) Window {
	w := math.Max(petW, panel.W)
	h := petH
	if panel.H > 0 {
		h += panel.H + gap
	}

	// The panel goes above the pet unless it would not fit there — a pet near
	// the top of the screen — and it would fit better below. Pushed back onto
	// the screen instead, it used to come to rest on top of the pet, covering
	// the very thing that was talking. Where neither side has room, the roomier
	// one covers less.
	need := panel.H + gap
	roomAbove := petY - monitor.Y
	roomBelow := monitor.Y + monitor.H - (petY + petH)
	below := panel.H > 0 && roomAbove < need && roomBelow > roomAbove

	// Where the window would go if nothing were in its way.
	x := petX - (w-petW)/2
	y := petY - (h - petH)
	if below {
		y = petY
	}

	// Sliding the window back on screen would drag the pet with it, so the pet's
	// offset inside the window is recomputed from wherever the window ends up.
	x = Clamp(x, monitor.X, monitor.X+monitor.W-w)
	y = Clamp(y, monitor.Y, monitor.Y+monitor.H-h)

	win := Window{X: x, Y: y, W: w, H: h, Below: below}
	win.PetX = petX - x
	win.PetY = petY - y

	if panel.H > 0 {
		center := win.PetX + petW/2
		win.PanelX = Clamp(center-panel.W/2, 0, w-panel.W)
		if below {
			win.PanelY = Clamp(win.PetY+petH+gap, 0, math.Max(h-panel.H, 0))
		} else {
			win.PanelY = Clamp(win.PetY-gap-panel.H, 0, math.Max(h-panel.H, 0))
		}
		win.TailX = Clamp(center, win.PanelX, win.PanelX+panel.W)
	}
	return win
}

// Drifted reports whether a window the system reports at got has moved or
// changed size from want, where it was last put. A pixel either way is not
// drift: on a display scaled by 125% or 150% the system rounds the size and
// position it hands back, and treating that as a change would have the window
// put back every time it was checked.
func Drifted(want, got Rect) bool {
	off := func(a, b float64) bool { return math.Abs(a-b) > 1 }
	return off(want.X, got.X) || off(want.Y, got.Y) || off(want.W, got.W) || off(want.H, got.H)
}

// Clamp keeps v within [lo, hi]. When the range is empty — a panel wider than
// the monitor, say — lo wins, which keeps the left and top edges on screen.
func Clamp(v, lo, hi float64) float64 {
	if hi < lo {
		return lo
	}
	return math.Min(math.Max(v, lo), hi)
}

// Size is a width and height in pixels.
type Size struct {
	W, H float64
}

// Point is a position in pixels.
type Point struct {
	X, Y float64
}

// Buttons lays a row of buttons out left to right: each as wide as its label
// plus pad either side, with gap between them. It returns each button's x and
// width within the row, and the row's total width.
func Buttons(labelWidths []float64, pad, gap float64) (xs, ws []float64, total float64) {
	x := 0.0
	for i, lw := range labelWidths {
		if i > 0 {
			x += gap
		}
		w := lw + 2*pad
		xs = append(xs, x)
		ws = append(ws, w)
		x += w
	}
	return xs, ws, x
}

// Orient turns a stack laid out by [StackBalloons] the right way up for where
// the panel ended up. Above the pet it is used as it is. Below the pet it is
// mirrored top to bottom, so the first balloon — the one with the tail — is
// still the one nearest the pet, and the rest pile away from it.
func Orient(points []Point, sizes []Size, panelH float64, below bool) []Point {
	out := make([]Point, len(points))
	copy(out, points)
	if !below {
		return out
	}
	for i := range out {
		out[i].Y = panelH - (points[i].Y + sizes[i].H)
	}
	return out
}

// StackBalloons piles balloons above the pet: the first at the bottom, where
// the tail is, and each later one above it and shifted to one side. Messages
// that arrive together are meant to look like a crowd talking at once, so the
// stack deliberately zigzags rather than lining up.
//
// The returned points are the balloons' top-left corners within the panel.
func StackBalloons(sizes []Size, offsetStep, gap float64) (Panel, []Point) {
	if len(sizes) == 0 {
		return Panel{}, nil
	}

	// Lay the stack out upwards from a baseline of zero. Y grows downwards, so
	// everything above the first balloon lands on the negative side, and the
	// whole thing is shifted back into place afterwards.
	points := make([]Point, len(sizes))
	y := 0.0
	for i, s := range sizes {
		y -= s.H
		points[i] = Point{X: sideStep(i, offsetStep), Y: y}
		y -= gap
	}

	minX, minY := points[0].X, points[0].Y
	maxX, maxY := points[0].X+sizes[0].W, points[0].Y+sizes[0].H
	for i, s := range sizes {
		minX = math.Min(minX, points[i].X)
		minY = math.Min(minY, points[i].Y)
		maxX = math.Max(maxX, points[i].X+s.W)
		maxY = math.Max(maxY, points[i].Y+s.H)
	}
	for i := range points {
		points[i].X -= minX
		points[i].Y -= minY
	}
	return Panel{W: maxX - minX, H: maxY - minY}, points
}

// sideStep fans the stack out to alternating sides: the first balloon square
// over the pet, then one to the right, one to the left, then further out.
func sideStep(i int, step float64) float64 {
	if i == 0 {
		return 0
	}
	out := float64((i+1)/2) * step
	if i%2 == 0 {
		return -out
	}
	return out
}
