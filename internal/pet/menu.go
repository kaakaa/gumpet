package pet

import (
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/kaakaa/gumpet/internal/browser"
	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/message"
)

// The menu is drawn at a fixed size rather than following message.text_scale:
// a menu is for reading once, and a big message setting should not turn it into
// a wall.
const (
	menuTextScale = 1.5
	menuPadding   = 8
	menuRowPadY   = 5
	menuGutter    = 24
	menuRadius    = 8
)

// menuItem is one row. A row with no action is a heading.
type menuItem struct {
	label string
	// detail is shown right-aligned: the current value of whatever the row
	// changes, or a piece of information.
	detail string
	action func() error
	// closes says whether picking the row puts the menu away. Toggles leave it
	// up so several can be flipped at once.
	closes bool
	// rule draws a dividing line above the row.
	rule bool
}

// menu is the popup the pet opens when it is clicked.
type menu struct {
	items  []menuItem
	width  float64
	height float64
	rowH   float64
	// hover is the index of the row under the cursor, or -1.
	hover int
}

// buildMenu describes the menu for the settings in force right now. It is
// rebuilt whenever it opens, and again after a row changes something, so the
// values on the right are never stale.
func (g *Game) buildMenu() *menu {
	cfg := g.cfg
	m := &menu{hover: -1}

	m.items = append(m.items,
		menuItem{label: "gumpet " + g.version, detail: cfg.Server.Addr},
		menuItem{
			label:  "Say something",
			detail: "test",
			rule:   true,
			closes: true,
			action: func() error {
				text := "Hello! " + time.Now().Format("15:04:05")
				msg := message.Message{Text: text}
				if g.history != nil {
					msg.ID = g.history.Add(text, 0).ID
				}
				g.enqueue(msg)
				return nil
			},
		},
		menuItem{
			label:  "Settings…",
			detail: "browser",
			closes: true,
			action: func() error { return browser.Open("http://" + cfg.Server.Addr + "/") },
		},
		menuItem{
			label:  "Walk",
			detail: string(cfg.Behavior.Roam),
			rule:   true,
			action: func() error {
				return g.store.Update(func(c *config.Config) {
					c.Behavior.Roam = nextRoam(c.Behavior.Roam)
				})
			},
		},
		menuItem{
			label:  "Roam the whole screen",
			detail: onOff(cfg.Stage.Fullscreen),
			action: func() error {
				return g.store.Update(func(c *config.Config) {
					c.Stage.Fullscreen = !c.Stage.Fullscreen
				})
			},
		},
		menuItem{
			label:  "Always on top",
			detail: onOff(cfg.Window.AlwaysOnTop),
			action: func() error {
				return g.store.Update(func(c *config.Config) {
					c.Window.AlwaysOnTop = !c.Window.AlwaysOnTop
				})
			},
		},
		menuItem{
			label:  "When idle",
			detail: idleLabel(cfg.Behavior.Mode),
			action: func() error {
				return g.store.Update(func(c *config.Config) {
					c.Behavior.Mode = nextMode(c.Behavior.Mode)
				})
			},
		},
		menuItem{
			label:  "Quit",
			rule:   true,
			closes: true,
			action: func() error { return ebiten.Termination },
		},
	)

	m.measure(g)
	return m
}

// measure works out how much room the menu needs.
func (m *menu) measure(g *Game) {
	f := g.fonts.menu
	widest := 0.0
	for _, it := range m.items {
		w := f.Advance(it.label)
		if it.detail != "" {
			w += menuGutter + f.Advance(it.detail)
		}
		widest = math.Max(widest, w)
	}

	m.rowH = f.lineHeight() + 2*menuRowPadY
	// A long listen address must not push the menu wider than the screen.
	m.width = math.Min(widest+2*menuPadding, g.monitor.W)
	m.height = float64(len(m.items))*m.rowH + 2*menuPadding
}

// rowRect is where row i sits inside the window.
func (m *menu) rowRect(originX, originY float64, i int) (x, y, w, h float64) {
	return originX + menuPadding,
		originY + menuPadding + float64(i)*m.rowH,
		m.width - 2*menuPadding,
		m.rowH
}

// itemAt returns the index of the row at a point inside the window, or -1.
func (m *menu) itemAt(originX, originY, px, py float64) int {
	for i, it := range m.items {
		if it.action == nil {
			continue
		}
		x, y, w, h := m.rowRect(originX, originY, i)
		if px >= x && px < x+w && py >= y && py < y+h {
			return i
		}
	}
	return -1
}

// menuAutoCloseAfter is how long the menu stays up once the cursor has left
// the window. A click somewhere else on the desktop goes to that other window,
// never to us, so the cursor wandering off is the only hint we get that the
// user is done with the menu.
const menuAutoCloseAfter = 3 * time.Second

// handleInput runs the pointer half of the menu: opening it by clicking the
// pet, highlighting rows, picking one, and putting it away again.
//
// None of this happens while window.click_through is on, because the window
// then receives no mouse input at all — which is exactly what that setting
// asks for.
func (g *Game) handleInput(dt time.Duration) error {
	if g.passthrough {
		g.menu, g.hovered = nil, false
		g.cursor.Reset()
		return nil
	}

	cx, cy := ebiten.CursorPosition()
	px, py := float64(cx)/g.deviceScale, float64(cy)/g.deviceScale

	// Aiming at either the pet or one of its balloons brings it to a halt, so
	// that neither is a moving target. Whether anyone is aiming is judged from
	// the cursor's position on the screen rather than within the window: the
	// window travels with the pet, so a cursor nobody has touched appears to
	// slide across it.
	onPet := g.overPet(px, py)
	onBalloon := g.balloonAt(px, py)
	// winX and winY are what the window was last actually told to be, rather
	// than the unrounded figure it was computed from, so adding them back
	// recovers the position the cursor is really at.
	g.hovered = g.cursor.Update(
		float64(g.winX)+px, float64(g.winY)+py,
		onPet || onBalloon >= 0,
		dt,
	)

	if g.menu != nil {
		if g.overWindow(px, py) {
			g.menuIdle = 0
		} else {
			g.menuIdle += dt
			if g.menuIdle >= menuAutoCloseAfter {
				g.menu = nil
				return nil
			}
		}
		g.menu.hover = g.menu.itemAt(g.win.PanelX, g.win.PanelY, px, py)
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return nil
	}

	if g.menu != nil {
		if i := g.menu.itemAt(g.win.PanelX, g.win.PanelY, px, py); i >= 0 {
			return g.pick(g.menu.items[i])
		}
		// A click anywhere else in the window dismisses the menu.
		g.menu = nil
		return nil
	}

	// Clicking a balloon takes it down, so a message that has been read does
	// not have to be waited out.
	if onBalloon >= 0 {
		g.dismiss(onBalloon)
		return nil
	}

	if onPet {
		g.menu = g.buildMenu()
		g.menuIdle = 0
	}
	return nil
}

// pick runs a row's action. Settings changes come back through the store, so
// the menu is rebuilt from what was actually saved rather than from what the
// row assumed.
func (g *Game) pick(it menuItem) error {
	err := it.action()
	if err == ebiten.Termination {
		return err
	}
	if err != nil {
		g.log.Error("menu action failed", "item", it.label, "error", err)
	}
	if it.closes {
		g.menu = nil
	} else {
		g.menu = g.buildMenu()
	}
	return nil
}

func (g *Game) overPet(px, py float64) bool {
	return px >= g.win.PetX && px < g.win.PetX+g.petWidth() &&
		py >= g.win.PetY && py < g.win.PetY+g.petHeight()
}

func (g *Game) overWindow(px, py float64) bool {
	return px >= 0 && py >= 0 && px < g.win.W && py < g.win.H
}

// balloonAt returns the balloon under a point in the window, or -1. The stack
// is searched from the top down, matching the order they are drawn in, so a
// click on an overlap takes down the one actually visible there.
func (g *Game) balloonAt(px, py float64) int {
	if g.menu != nil {
		return -1
	}
	for i := len(g.showing) - 1; i >= 0; i-- {
		b := g.showing[i].balloon
		if b == nil || i >= len(g.placed) {
			continue
		}
		x := g.win.PanelX + g.placed[i].X
		y := g.win.PanelY + g.placed[i].Y
		if px >= x && px < x+b.width && py >= y && py < y+b.height {
			return i
		}
	}
	return -1
}

// idleLabel says what the pet does with itself between messages.
func idleLabel(m config.Mode) string {
	switch m {
	case config.ModeFaded:
		return "fade"
	case config.ModeOnMessage:
		return "hide"
	default:
		return "stay"
	}
}

func nextMode(m config.Mode) config.Mode {
	for i, candidate := range config.Modes {
		if candidate == m {
			return config.Modes[(i+1)%len(config.Modes)]
		}
	}
	return config.ModeAlways
}

func nextRoam(r config.Roam) config.Roam {
	for i, candidate := range config.Roams {
		if candidate == r {
			return config.Roams[(i+1)%len(config.Roams)]
		}
	}
	return config.RoamHorizontal
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
