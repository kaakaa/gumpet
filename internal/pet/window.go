package pet

import (
	"fmt"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/kaakaa/gumpet/internal/display"
	"github.com/kaakaa/gumpet/internal/layout"
	"github.com/kaakaa/gumpet/internal/react"
)

// panelGap is the space between the pet and the balloon or menu above it.
const panelGap = 6

// readMonitor picks up the size of the monitor the window is on, which is also
// what defines the stage. Ebitengine reports window positions relative to the
// current monitor's top-left corner, so that corner is the origin throughout.
func (g *Game) readMonitor() error {
	if !g.onWantedMonitor() {
		return nil
	}
	w, h := ebiten.Monitor().Size()
	if w == 0 || h == 0 {
		return nil
	}
	next := layout.Rect{W: float64(w), H: float64(h)}
	if next == g.monitor && g.started {
		return nil
	}
	g.monitor = next
	g.started = true
	g.reshapeWalker()
	return nil
}

// Monitors describes the displays attached to the machine, in the order
// stage.display numbers them.
func Monitors() []display.Monitor {
	var found []display.Monitor
	for i, m := range ebiten.AppendMonitors(nil) {
		w, h := m.Size()
		found = append(found, display.Monitor{
			Number: i + 1,
			Name:   m.Name(),
			Width:  w,
			Height: h,
		})
	}
	return found
}

// UseDisplay asks for the window to be put on the monitor the settings name.
// Ebitengine accepts the choice either side of Run, storing it for when the
// window is made or moving the window there if it already exists.
func UseDisplay(want int) {
	monitors := ebiten.AppendMonitors(nil)
	if len(monitors) == 0 {
		return
	}
	ebiten.SetMonitor(monitors[display.Pick(want, len(monitors))])
}

// wantedMonitor is the monitor the settings name, or nil when they name none
// that exists.
func (g *Game) wantedMonitor() *ebiten.MonitorType {
	monitors := ebiten.AppendMonitors(nil)
	if len(monitors) == 0 {
		return nil
	}
	return monitors[display.Pick(g.cfg.Stage.Display, len(monitors))]
}

// onWantedMonitor reports whether the window has arrived on the monitor the
// settings name.
//
// Moving there is not instant, and Ebitengine remembers which monitor a window
// is on for a second at a time. Until it catches up, every coordinate gumpet
// computes would be read against the monitor being left rather than the one
// being arrived at — and since the window is repositioned on every tick, that
// is enough to drag it straight back. So nothing is positioned until the move
// has plainly landed.
func (g *Game) onWantedMonitor() bool {
	want := g.wantedMonitor()
	return want == nil || want == ebiten.Monitor()
}

func (g *Game) stage() layout.Rect {
	r := layout.Stage(g.cfg.Stage, int(g.monitor.W), int(g.monitor.H))
	if g.dragActive {
		// Mid-drag the stage is wherever the cursor has taken it, which the
		// settings do not know about until the button comes up.
		r.X, r.Y = g.dragX, g.dragY
	}
	return r
}

// hidden reports whether there is nothing to draw, which is how "on-message"
// mode makes the pet disappear.
func (g *Game) hidden() bool {
	return !g.cfg.ShowsPet(len(g.showing) > 0, g.menu != nil)
}

// petOpacity is how solid to draw the pet right now.
func (g *Game) petOpacity() float64 {
	return g.cfg.PetOpacity(len(g.showing) > 0, g.menu != nil)
}

// activePanel is the balloon stack or menu currently sitting above the pet.
func (g *Game) activePanel() layout.Panel {
	switch {
	case g.hidden():
		return layout.Panel{}
	case g.menu != nil:
		return layout.Panel{W: g.menu.width, H: g.menu.height}
	default:
		return g.panel
	}
}

// windowCheckEvery is how often the window's real size and position are
// compared with what gumpet asked for. Asking the system costs a round trip to
// the main thread, and a window that was knocked out of place a second ago is
// not yet a problem anyone has noticed.
const windowCheckEvery = time.Second

// checkWindow puts the window back if something other than gumpet resized or
// moved it.
//
// placeWindow only tells the system about changes it made itself, comparing
// with what it last asked for. A window the system shrank or moved on its own
// — waking a display, a change of resolution or scale, a remote session —
// would stay that way for good: the pet and its balloons are still laid out
// for the full window, so only the top-left corner of a balloon shows, the pet
// is outside the window, and there is nothing left to click.
func (g *Game) checkWindow(dt time.Duration) {
	g.sinceWindowCheck += dt
	set := g.windowSet
	g.windowSet = false
	// A size or position asked for this tick has not reached the window yet,
	// and would read as drift. The check waits for a tick that left the window
	// alone, which while the pet walks is a few ticks away, and while it is
	// being dragged is when the drag ends.
	if g.sinceWindowCheck < windowCheckEvery || set {
		return
	}
	g.sinceWindowCheck = 0
	// Nothing placed yet, or mid-move to another monitor, where positions are
	// read against the wrong one; and a minimised window is meant to be small.
	if g.winW == 0 || g.winH == 0 || !g.onWantedMonitor() || ebiten.IsWindowMinimized() {
		return
	}
	gx, gy := ebiten.WindowPosition()
	gw, gh := ebiten.WindowSize()
	want := layout.Rect{X: float64(g.winX), Y: float64(g.winY), W: float64(g.winW), H: float64(g.winH)}
	got := layout.Rect{X: float64(gx), Y: float64(gy), W: float64(gw), H: float64(gh)}
	if !layout.Drifted(want, got) {
		g.drifted = false
		return
	}
	if !g.drifted {
		g.log.Warn("window was moved or resized from outside; putting it back",
			"want", fmt.Sprintf("%dx%d at %d,%d", g.winW, g.winH, g.winX, g.winY),
			"got", fmt.Sprintf("%dx%d at %d,%d", gw, gh, gx, gy))
		g.drifted = true
	}
	ebiten.SetWindowSize(g.winW, g.winH)
	ebiten.SetWindowPosition(g.winX, g.winY)
}

// placeWindow sizes the window around the pet and its panel and moves it to
// wherever the pet has walked to.
func (g *Game) placeWindow() {
	if !g.onWantedMonitor() {
		return
	}
	if !g.settled {
		g.settled = true
		g.log.Info("on monitor", "name", ebiten.Monitor().Name(),
			"size", fmt.Sprintf("%.0fx%.0f", g.monitor.W, g.monitor.H))
	}
	petX, petY := g.walker.Pos()
	// A reaction moves where the pet is drawn, not where it is walking: the
	// window goes with it, so nothing is clipped, and when the reaction is
	// over the pet is exactly where its walk had got to.
	dx, dy, _ := react.Offset(g.reaction, g.reactFor)
	petX, petY = petX+dx, petY+dy
	g.win = layout.PlaceWindow(petX, petY, g.petWidth(), g.petHeight(), g.activePanel(), panelGap, g.monitor)
	// A pet near the top of the screen has its balloons below it, stacked
	// downwards from the one with the tail.
	sizes := make([]layout.Size, len(g.placed))
	for i := range g.placed {
		if i < len(g.showing) && g.showing[i].balloon != nil {
			sizes[i] = layout.Size{W: g.showing[i].balloon.width, H: g.showing[i].balloon.height}
		}
	}
	g.at = layout.Orient(g.placed, sizes, g.panel.H, g.win.Below)

	w, h := int(math.Ceil(g.win.W)), int(math.Ceil(g.win.H))
	if w != g.winW || h != g.winH {
		ebiten.SetWindowSize(w, h)
		g.winW, g.winH = w, h
		g.windowSet = true
	}
	x, y := int(math.Round(g.win.X)), int(math.Round(g.win.Y))
	if x != g.winX || y != g.winY {
		ebiten.SetWindowPosition(x, y)
		g.winX, g.winY = x, y
		g.windowSet = true
	}

	// An invisible pet must not swallow clicks, whatever the setting says.
	want := g.cfg.Window.ClickThrough || g.hidden()
	if want != g.passthrough {
		ebiten.SetWindowMousePassthrough(want)
		g.passthrough = want
	}
}
