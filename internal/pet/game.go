// Package pet draws the desktop pet, walks it around the screen, and runs the
// menu it opens when clicked.
//
// gumpet's window is only as big as the pet and whatever it is currently
// saying, and it is moved around the monitor to follow the pet. A window big
// enough to roam inside would be a screen-sized rectangle swallowing every
// click that landed on it.
package pet

import (
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/kaakaa/gumpet/internal/chatter"
	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/display"
	"github.com/kaakaa/gumpet/internal/history"
	"github.com/kaakaa/gumpet/internal/hover"
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
	// idle marks a remark the pet made up itself. It is never recorded, never
	// queued, and gives way the moment a real message arrives.
	idle bool
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

	// chatter is the pet talking to itself between messages, or nil when the
	// setting is off.
	chatter *chatter.Sayer
	// chatterSrc is what the current Sayer was built from, so it is only
	// rebuilt when the settings behind it actually change.
	chatterSrc config.Chatter

	menu *menu
	// menuIdle is how long the cursor has been away from the window, which is
	// the only sign we get that the user has moved on.
	menuIdle time.Duration
	// hovered is whether the cursor is holding the pet still so that it, or
	// one of its balloons, can be clicked. [hover.Tracker] decides: a cursor
	// merely lying where the pet wandered does not count.
	hovered bool
	cursor  hover.Tracker

	started bool
	// settled says the window has reached the monitor it was asked for, and is
	// only kept so that arriving is logged once rather than every tick.
	settled bool
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
	g.advanceChatter(dt)
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

func (g *Game) reshapeWalker() {
	g.walker.Reshape(g.cfg.Behavior.Roam, g.stage(), g.petWidth(), g.petHeight(), g.cfg.Behavior.Speed)
}

func (g *Game) stage() layout.Rect {
	return layout.Stage(g.cfg.Stage, int(g.monitor.W), int(g.monitor.H))
}

// canRoam reports whether the pet should be walking. It carries on walking
// while it talks — the balloons are drawn above it and travel with it — but
// holds still while its menu is open, so the rows stay under the cursor, and
// while someone is plainly reaching for it with the cursor.
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

	if cfg.Stage.Display != old.Stage.Display {
		UseDisplay(cfg.Stage.Display)
		// The monitor moved to may be the same size as the one left behind,
		// which readMonitor would take for nothing having changed.
		g.monitor = layout.Rect{}
		g.started = false
		g.settled = false
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

	if len(g.queue) > 0 {
		g.dropIdleTalk()
	}

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

// advanceChatter lets the pet say something of its own accord.
//
// A remark is put straight on screen rather than into the queue: the queue is
// for messages somebody sent, and a remark must not take a place in it or push
// a real message out of one. For the same reason it is never recorded — the
// messages page is a log of what arrived, and filling it with the pet talking
// to itself would age real messages out of the record.
func (g *Game) advanceChatter(dt time.Duration) {
	g.ensureChatter()
	if g.chatter == nil {
		return
	}

	// Anything else on screen or waiting means the pet has better to do. So
	// does an open menu: interrupting someone reading it would be rude.
	quiet := len(g.showing) == 0 && len(g.queue) == 0 && g.menu == nil
	text, ok := g.chatter.Tick(dt, quiet)
	if !ok {
		return
	}

	g.showing = append(g.showing, shown{
		msg:       message.Message{Text: text, Level: message.LevelInfo},
		remaining: time.Duration(g.cfg.Message.DurationSec * float64(time.Second)),
		idle:      true,
	})
	g.panelDirty = true
	g.resetAnimation()
}

// ensureChatter builds the Sayer when the settings behind it change, and drops
// it when the feature is off.
//
// In "on-message" mode the pet is hidden until something arrives, so a remark
// would summon it onto the screen and defeat the mode. It stays quiet there
// whatever the chatter setting says.
func (g *Game) ensureChatter() {
	want := g.cfg.Behavior.Chatter
	if !want.Enabled || g.cfg.Behavior.Mode == config.ModeOnMessage {
		g.chatter, g.chatterSrc = nil, config.Chatter{}
		return
	}
	if g.chatter != nil && g.chatterSrc == want {
		return
	}

	sayings, from := g.sayings(want.Source)
	g.chatterSrc = want
	g.chatter = chatter.New(
		sayings,
		time.Duration(want.IntervalSec*float64(time.Second)),
		rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
	)
	g.log.Info("idle chatter on", "sayings", len(sayings), "from", from,
		"every", time.Duration(want.IntervalSec*float64(time.Second)))
}

// sayings reads the configured file, falling back to the bundled list. A
// missing or unreadable file is worth a line in the log and nothing more: the
// pet should carry on talking, not stop working over a list of proverbs.
func (g *Game) sayings(path string) ([]string, string) {
	if path == "" {
		return chatter.Bundled(), "the bundled list"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		g.log.Error("could not read the sayings file, using the bundled list",
			"path", path, "error", err)
		return chatter.Bundled(), "the bundled list"
	}
	sayings := chatter.Parse(data)
	if len(sayings) == 0 {
		g.log.Error("the sayings file has nothing in it, using the bundled list", "path", path)
		return chatter.Bundled(), "the bundled list"
	}
	return sayings, path
}

// dropIdleTalk takes down anything the pet was saying to itself. A message
// somebody actually sent takes the screen back at once rather than queueing
// behind a proverb.
func (g *Game) dropIdleTalk() {
	kept := g.showing[:0]
	for _, s := range g.showing {
		if !s.idle {
			kept = append(kept, s)
		}
	}
	if len(kept) != len(g.showing) {
		g.panelDirty = true
	}
	g.showing = kept
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
		b := g.layoutBalloon(g.showing[i].msg)
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
	if !g.onWantedMonitor() {
		return
	}
	if !g.settled {
		g.settled = true
		g.log.Info("on monitor", "name", ebiten.Monitor().Name(),
			"size", fmt.Sprintf("%.0fx%.0f", g.monitor.W, g.monitor.H))
	}
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
