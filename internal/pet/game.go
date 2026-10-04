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

	"github.com/kaakaa/gumpet/internal/ask"
	"github.com/kaakaa/gumpet/internal/chatter"
	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/drag"
	"github.com/kaakaa/gumpet/internal/feed"
	"github.com/kaakaa/gumpet/internal/history"
	"github.com/kaakaa/gumpet/internal/hover"
	"github.com/kaakaa/gumpet/internal/lang"
	"github.com/kaakaa/gumpet/internal/layout"
	"github.com/kaakaa/gumpet/internal/message"
	"github.com/kaakaa/gumpet/internal/petpack"
	"github.com/kaakaa/gumpet/internal/quiet"
	"github.com/kaakaa/gumpet/internal/react"
	"github.com/kaakaa/gumpet/internal/roam"
	"github.com/kaakaa/gumpet/internal/settings"
)

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
	// Remarks records what the pet says of its own accord, apart from
	// History. It may be nil, in which case nothing is recorded.
	Remarks *history.Store
	// FeedReports is told how every read of a feed went, for the messages
	// page. It may be nil.
	FeedReports *feed.Reports
	// Asks brings questions to put on screen as balloons with buttons, and
	// takes the answers. It may be nil.
	Asks *ask.Broker
	// Quit ends the game loop when it is closed.
	Quit <-chan struct{}
	// Restart stops gumpet and starts it again, the way an update does. The
	// menu offers it for when something on screen has gone wrong and a fresh
	// start is quicker than working out what. Nil hides the row.
	Restart func()
	Log     *slog.Logger
	Version string
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
	remarks *history.Store
	// feedReports: see Options.FeedReports.
	feedReports *feed.Reports
	asks        *ask.Broker
	quit        <-chan struct{}
	restart     func()
	log         *slog.Logger
	version     string
	debug       bool

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
	// sinceWindowCheck counts towards the next look at where the window really
	// is, and drifted says the last look found it somewhere else, so that a
	// window the system keeps resizing is reported once rather than every
	// second. See checkWindow.
	sinceWindowCheck time.Duration
	drifted          bool
	// windowSet says placeWindow asked for a new size or position this tick.
	windowSet bool
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
	panel  layout.Panel
	placed []layout.Point
	// at is where each balloon sits in the panel as drawn: placed, turned the
	// right way up for where the window put the panel. See [layout.Orient].
	at         []layout.Point
	panelDirty bool

	// chatter is the pet talking to itself between messages, or nil when the
	// setting is off.
	chatter *chatter.Sayer
	// seen is which headlines have been said already, so a repeat can be
	// told from a new one.
	seen chatter.Seen
	// reaction is how the pet is moving in answer to a message just shown,
	// and reactFor how long it has been at it. See package react.
	reaction react.Kind
	reactFor time.Duration
	// hush follows the quiet window. wasQuiet is what it said last frame, so
	// the moment quiet begins can be told from every frame after it.
	hush     quiet.Hush
	wasQuiet bool
	// lang is what the pet's own words are said in. locale is the system's,
	// found once at startup: asking macOS means running a program, and the
	// answer does not change while gumpet runs.
	lang   lang.Lang
	locale string
	// localSayings is what the pet says when the feeds have nothing for it:
	// the file or the bundled list, kept so that falling back does not mean
	// reading it off disk again.
	localSayings []chatter.Remark
	// chatterSrc is what the current Sayer was built from, so it is only
	// rebuilt when the settings behind it actually change. It holds a slice
	// now, so it is compared field by field rather than with ==.
	chatterSrc config.Chatter
	// headlines carries what the feed reader found back to the game loop. The
	// fetch runs in its own goroutine and the loop only ever reads this, which
	// is why neither of them needs a lock.
	headlines chan [][]chatter.Remark
	// fetching stops a second fetch being started while one is in flight, and
	// fetchWait counts down to the next one.
	fetching  bool
	fetchWait time.Duration

	menu *menu
	// menuIdle is how long the cursor has been away from the window, which is
	// the only sign we get that the user has moved on.
	menuIdle time.Duration
	// hovered is whether the cursor is holding the pet still so that it, or
	// one of its balloons, can be clicked. [hover.Tracker] decides: a cursor
	// merely lying where the pet wandered does not count.
	hovered bool
	// drag follows a press-to-release gesture on the pet. dragActive says the
	// stage is currently being moved by one, and dragX/dragY are where it has
	// got to — kept here rather than written to the settings on every tick, so
	// that a drag is one saved change rather than hundreds.
	// Whether a gesture is in progress lives in the tracker alone: keeping a
	// second copy here is what let the two disagree, and the drag never
	// started.
	drag         drag.Tracker
	dragActive   bool
	dragX, dragY float64
	// reading says the cursor is being held on a balloon, which stops the
	// countdown: a message being read should not vanish mid-sentence, and a
	// link cannot be clicked if it disappears while being aimed at.
	reading bool
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
		remarks:     o.Remarks,
		feedReports: o.FeedReports,
		asks:        o.Asks,
		quit:        o.Quit,
		restart:     o.Restart,
		log:         o.Log,
		version:     o.Version,
		debug:       os.Getenv("GUMPET_DEBUG") != "",
		// Buffered, so a fetch that lands while the loop is elsewhere does not
		// leave its goroutine parked on the send.
		headlines:   make(chan [][]chatter.Remark, 1),
		deviceScale: 1,
		passthrough: cfg.Window.ClickThrough,
	}
	// A placeholder until the first Update can ask what monitor we are on.
	g.monitor = layout.Rect{W: float64(cfg.Stage.Width), H: float64(cfg.Stage.Height)}
	g.walker = roam.New(cfg.Behavior.Roam, g.stage(), g.petWidth(), g.petHeight(), cfg.Behavior.Speed, nil)
	g.walker.SetJumping(cfg.Behavior.Jump)
	g.winW, g.winH = int(math.Ceil(g.petWidth())), int(math.Ceil(g.petHeight()))
	g.setQuietWindow(cfg.Behavior.Quiet)
	// The taskbar button shows whatever pet is on screen, and follows it when
	// it changes. See applyConfig.
	ebiten.SetWindowIcon(o.Pack.Icon)
	g.locale = lang.Detect()
	g.setLanguage(cfg.Language)
	return g
}

// setLanguage adopts the language setting. Balloons already on screen keep
// what they said; the menu is rebuilt by applyConfig.
func (g *Game) setLanguage(setting string) {
	g.lang = lang.Resolve(setting, func() string { return g.locale })
}

// tr says one of the pet's own words in its language. Everything gumpet
// writes for the pet to show goes through here, and a test reads this
// package's source to find every word said this way.
func (g *Game) tr(en string) string { return g.lang.T(en) }

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
	g.advanceQuiet()
	g.drainAsks()
	g.drainInbox()
	g.advanceMessages(dt)
	g.advanceChatter(dt)
	g.advanceFeed(dt)
	if err := g.handleInput(dt); err != nil {
		return err
	}
	if g.canRoam() {
		g.walker.Step(dt)
	}
	g.advanceAnimation(dt)
	g.advanceReaction(dt)
	g.rebuildPanel()
	g.placeWindow()
	g.checkWindow(dt)
	return nil
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
			ebiten.SetWindowIcon(pack.Icon)
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
	if cfg.Behavior.Quiet != old.Behavior.Quiet {
		g.setQuietWindow(cfg.Behavior.Quiet)
	}
	if cfg.Language != old.Language {
		g.setLanguage(cfg.Language)
	}
	g.reshapeWalker()
	g.panelDirty = true
	if g.menu != nil {
		g.menu = g.buildMenu()
	}
	if over := len(g.queue) - g.cfg.Message.MaxQueue; over > 0 {
		g.queue = g.queue[over:]
	}
}
