// Package pet draws the desktop pet, walks it around the screen, and runs the
// menu it opens when clicked.
//
// gumpet's window is only as big as the pet and whatever it is currently
// saying, and it is moved around the monitor to follow the pet. A window big
// enough to roam inside would be a screen-sized rectangle swallowing every
// click that landed on it.
package pet

import (
	"log/slog"
	"math"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/history"
	"github.com/kaakaa/gumpet/internal/layout"
	"github.com/kaakaa/gumpet/internal/message"
	"github.com/kaakaa/gumpet/internal/petpack"
	"github.com/kaakaa/gumpet/internal/roam"
	"github.com/kaakaa/gumpet/internal/settings"
)

// panelGap is the space between the pet and the balloon or menu above it.
const panelGap = 6

// Options is what New needs to build a Game.
type Options struct {
	// Store is where settings are read from and written back to, shared with
	// the settings page.
	Store *settings.Store
	Pack  *petpack.Pack
	// Inbox carries messages to display, in the order they arrive.
	Inbox <-chan message.Message
	// History is told when a message actually reaches the screen. It may be
	// nil, in which case nothing is recorded.
	History *history.Store
	// Quit ends the game loop when it is closed.
	Quit    <-chan struct{}
	Log     *slog.Logger
	Version string
}

// shown is a message currently on screen and how long it has left.
type shown struct {
	msg       message.Message
	remaining time.Duration
	balloon   *balloon
}

// Game is the Ebitengine game that is gumpet's pet.
type Game struct {
	store   *settings.Store
	cfg     config.Config
	updates <-chan config.Config
	pack    *petpack.Pack
	fonts   fonts
	inbox   <-chan message.Message
	history *history.Store
	quit    <-chan struct{}
	log     *slog.Logger
	version string
	debug   bool

	// deviceScale converts the logical coordinates everything below is written
	// in into the physical pixels Draw is handed.
	deviceScale float64

	monitor layout.Rect
	walker  *roam.Walker

	// win is where the window and everything in it goes, recomputed each tick.
	win layout.Window
	// winW, winH and winX, winY are what the window was last asked to be, so
	// that it is only actually moved and resized when something changed.
	winW, winH int
	winX, winY int
	// passthrough is the last value handed to Ebitengine, which is not always
	// the configured one: a hidden pet must not swallow clicks.
	passthrough bool

	frameIdx     int
	frameElapsed time.Duration

	// showing is what is on screen right now, oldest first. The first one is
	// the one the tail points at.
	showing []shown
	queue   []message.Message
	// panel and placed are the stacked balloons, rebuilt only when what is on
	// screen or the settings change.
	panel      layout.Panel
	placed     []layout.Point
	panelDirty bool

	menu *menu
	// menuIdle is how long the cursor has been away from the window, which is
	// the only sign we get that the user has moved on.
	menuIdle time.Duration
	// hovered is whether the cursor is resting on the pet, which stops it so
	// that it can be clicked.
	hovered bool

	started bool
}

// New builds the game.
func New(o Options) *Game {
	cfg := o.Store.Get()
	g := &Game{
		store:       o.Store,
		cfg:         cfg,
		updates:     o.Store.Subscribe(8),
		pack:        o.Pack,
		inbox:       o.Inbox,
		history:     o.History,
		quit:        o.Quit,
		log:         o.Log,
		version:     o.Version,
		debug:       os.Getenv("GUMPET_DEBUG") != "",
		deviceScale: 1,
		passthrough: cfg.Window.ClickThrough,
	}
	// A placeholder until the first Update can ask what monitor we are on.
	g.monitor = layout.Rect{W: float64(cfg.Stage.Width), H: float64(cfg.Stage.Height)}
	g.walker = roam.New(cfg.Behavior.Roam, g.stage(), g.petWidth(), g.petHeight(), cfg.Behavior.Speed, nil)
	g.winW, g.winH = int(math.Ceil(g.petWidth())), int(math.Ceil(g.petHeight()))
	return g
}

// Layout is never called: LayoutF takes precedence and gives us the screen in
// physical pixels, which keeps the artwork and text sharp on HiDPI displays.
func (g *Game) Layout(int, int) (int, int) {
	return g.winW, g.winH
}

// LayoutF returns the logical screen size in physical pixels.
func (g *Game) LayoutF(outsideWidth, outsideHeight float64) (screenWidth, screenHeight float64) {
	g.deviceScale = ebiten.Monitor().DeviceScaleFactor()
	return outsideWidth * g.deviceScale, outsideHeight * g.deviceScale
}

// Update advances one tick.
func (g *Game) Update() error {
	select {
	case <-g.quit:
		return ebiten.Termination
	default:
	}

	if err := g.readMonitor(); err != nil {
		return err
	}
	dt := time.Second / time.Duration(ebiten.TPS())

	g.drainUpdates()
	g.ensureFonts()
	g.drainInbox()
	g.advanceMessages(dt)
	if err := g.handleInput(dt); err != nil {
		return err
	}
	if g.canRoam() {
		g.walker.Step(dt)
	}
	g.advanceAnimation(dt)
	g.rebuildPanel()
	g.placeWindow()
	return nil
}

// readMonitor picks up the size of the monitor the window is on, which is also
// what defines the stage. Ebitengine reports window positions relative to the
// current monitor's top-left corner, so that corner is the origin throughout.
func (g *Game) readMonitor() error {
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

func (g *Game) reshapeWalker() {
	g.walker.Reshape(g.cfg.Behavior.Roam, g.stage(), g.petWidth(), g.petHeight(), g.cfg.Behavior.Speed)
}

func (g *Game) stage() layout.Rect {
	return layout.Stage(g.cfg.Stage, int(g.monitor.W), int(g.monitor.H))
}

// canRoam reports whether the pet should be walking. It carries on walking
// while it talks — the balloon is drawn above it and travels with it — but
// holds still while its menu is open, so the rows stay under the cursor, and
// while the cursor is on it, so that a pet crossing the screen at speed is
// still something you can click.
func (g *Game) canRoam() bool {
	return g.menu == nil && !g.hovered
}

// drainUpdates applies any settings saved since the last tick, whether they
// came from the settings page or from the pet's own menu.
func (g *Game) drainUpdates() {
	for {
		select {
		case cfg := <-g.updates:
			g.applyConfig(cfg)
		default:
			return
		}
	}
}

// applyConfig swaps in new settings. server.addr and window.skip_taskbar are
// fixed once the process starts, so they are left for the next run; everything
// else takes effect here.
func (g *Game) applyConfig(cfg config.Config) {
	old := g.cfg
	g.cfg = cfg

	if cfg.Pet.Source != old.Pet.Source {
		pack, err := petpack.Load(cfg.Pet.Source)
		if err != nil {
			// Both the settings page and the menu check the path before
			// saving, so this is a file that went away in between. Keep
			// drawing the pet we have.
			g.log.Error("could not load the new pet, keeping the current one",
				"source", cfg.Pet.Source, "error", err)
			g.cfg.Pet.Source = old.Pet.Source
		} else {
			g.log.Info("loaded pet", "pack", pack.Name, "frames", len(pack.Walk), "size", pack.Size)
			g.pack = pack
			g.resetAnimation()
		}
	}

	ebiten.SetWindowFloating(cfg.Window.AlwaysOnTop)
	g.reshapeWalker()
	g.panelDirty = true
	if g.menu != nil {
		g.menu = g.buildMenu()
	}
	if over := len(g.queue) - g.cfg.Message.MaxQueue; over > 0 {
		g.queue = g.queue[over:]
	}
}

// drainInbox takes everything the HTTP server has published since the last tick.
func (g *Game) drainInbox() {
	for {
		select {
		case msg := <-g.inbox:
			g.enqueue(msg)
		default:
			return
		}
	}
}

func (g *Game) enqueue(msg message.Message) {
	g.queue = append(g.queue, msg)
	if over := len(g.queue) - g.cfg.Message.MaxQueue; over > 0 {
		g.queue = g.queue[over:]
	}
}

// advanceMessages ages out what is on screen and takes the next messages off
// the queue, oldest first. Several can be up at once: a burst that arrives
// together is shown together rather than made to queue politely.
func (g *Game) advanceMessages(dt time.Duration) {
	kept := g.showing[:0]
	for _, s := range g.showing {
		if s.remaining -= dt; s.remaining > 0 {
			kept = append(kept, s)
		}
	}
	if len(kept) != len(g.showing) {
		g.panelDirty = true
	}
	g.showing = kept

	for len(g.showing) < g.cfg.Message.MaxVisible && len(g.queue) > 0 {
		msg := g.queue[0]
		g.queue = g.queue[1:]

		remaining := msg.Duration
		if remaining <= 0 {
			remaining = time.Duration(g.cfg.Message.DurationSec * float64(time.Second))
		}
		g.showing = append(g.showing, shown{msg: msg, remaining: remaining})
		g.panelDirty = true

		if g.history != nil && msg.ID != "" {
			g.history.MarkShown(msg.ID)
		}
	}
	if g.panelDirty {
		g.resetAnimation()
	}
}

// dismiss takes one balloon off the screen early. Whatever is next in the
// queue takes its place on the following tick, as if it had timed out.
func (g *Game) dismiss(i int) {
	if i < 0 || i >= len(g.showing) {
		return
	}
	g.showing = append(g.showing[:i], g.showing[i+1:]...)
	g.panelDirty = true
}

// rebuildPanel lays the balloons out into one stack above the pet.
func (g *Game) rebuildPanel() {
	if !g.panelDirty {
		return
	}
	g.panelDirty = false

	if len(g.showing) == 0 {
		g.panel, g.placed = layout.Panel{}, nil
		return
	}
	sizes := make([]layout.Size, len(g.showing))
	for i := range g.showing {
		b := g.layoutBalloon(g.showing[i].msg.Text)
		g.showing[i].balloon = b
		sizes[i] = layout.Size{W: b.width, H: b.height}
	}
	g.panel, g.placed = layout.StackBalloons(sizes, balloonSideStep, balloonStackGap)
}

// advanceAnimation steps through the current animation. A pet that has stopped
// to look around holds its first frame.
func (g *Game) advanceAnimation(dt time.Duration) {
	frames := g.frames()
	if len(frames) < 2 {
		g.frameIdx = 0
		return
	}
	if g.walker.Paused() {
		g.resetAnimation()
		return
	}

	g.frameElapsed += dt
	for {
		d := frames[g.frameIdx%len(frames)].Duration
		if d <= 0 {
			d = time.Duration(float64(time.Second) / g.cfg.Pet.FPS)
		}
		if g.frameElapsed < d {
			return
		}
		g.frameElapsed -= d
		g.frameIdx = (g.frameIdx + 1) % len(frames)
	}
}

func (g *Game) resetAnimation() {
	g.frameIdx = 0
	g.frameElapsed = 0
}

// frames is the animation to play right now.
func (g *Game) frames() []petpack.Frame {
	if len(g.showing) > 0 {
		return g.pack.TalkFrames()
	}
	return g.pack.Walk
}

func (g *Game) petWidth() float64  { return float64(g.pack.Size.X) * g.cfg.Pet.Scale }
func (g *Game) petHeight() float64 { return float64(g.pack.Size.Y) * g.cfg.Pet.Scale }

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

// placeWindow sizes the window around the pet and its panel and moves it to
// wherever the pet has walked to.
func (g *Game) placeWindow() {
	petX, petY := g.walker.Pos()
	g.win = layout.PlaceWindow(petX, petY, g.petWidth(), g.petHeight(), g.activePanel(), panelGap, g.monitor)

	w, h := int(math.Ceil(g.win.W)), int(math.Ceil(g.win.H))
	if w != g.winW || h != g.winH {
		ebiten.SetWindowSize(w, h)
		g.winW, g.winH = w, h
	}
	x, y := int(math.Round(g.win.X)), int(math.Round(g.win.Y))
	if x != g.winX || y != g.winY {
		ebiten.SetWindowPosition(x, y)
		g.winX, g.winY = x, y
	}

	// An invisible pet must not swallow clicks, whatever the setting says.
	want := g.cfg.Window.ClickThrough || g.hidden()
	if want != g.passthrough {
		ebiten.SetWindowMousePassthrough(want)
		g.passthrough = want
	}
}
