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

	// Where the window would go if nothing were in its way.
	x := petX - (w-petW)/2
	y := petY - (h - petH)

	// Sliding the window back on screen would drag the pet with it, so the pet's
	// offset inside the window is recomputed from wherever the window ends up.
	x = Clamp(x, monitor.X, monitor.X+monitor.W-w)
	y = Clamp(y, monitor.Y, monitor.Y+monitor.H-h)

	win := Window{X: x, Y: y, W: w, H: h}
	win.PetX = petX - x
	win.PetY = petY - y

	if panel.H > 0 {
		center := win.PetX + petW/2
		win.PanelX = Clamp(center-panel.W/2, 0, w-panel.W)
		win.PanelY = Clamp(win.PetY-gap-panel.H, 0, math.Max(h-panel.H, 0))
		win.TailX = Clamp(center, win.PanelX, win.PanelX+panel.W)
	}
	return win
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

// Display turns the stage.display setting into an index into the monitors the
// system reports.
//
// The setting counts from one, the way the monitors are numbered in a display
// arrangement rather than the way a slice is. A machine can lose a monitor
// between one run and the next, so a number that no longer names one falls
// back to the first rather than failing: a pet on the wrong screen is a
// nuisance, a pet that will not start is worse.
func Display(want, available int) int {
	if available <= 0 {
		return 0
	}
	if want < 1 || want > available {
		return 0
	}
	return want - 1
}
