package pet

import (
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/kaakaa/gumpet/internal/browser"
	"github.com/kaakaa/gumpet/internal/clipboard"
	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/drag"
	"github.com/kaakaa/gumpet/internal/petsrc"
	"github.com/kaakaa/gumpet/internal/richtext"
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
	tr := g.tr
	m := &menu{hover: -1}

	// The menu holds what is worth reaching for while looking at the pet: the
	// two pages, the two things people switch on the spot, and a way out — or
	// back in, when the screen has got into a state. Everything set
	// once and left alone lives on the settings page, where there is room to
	// explain it. A longer menu here was a list nobody read to the end of.
	m.items = append(m.items,
		menuItem{label: "gumpet " + g.version},
		// The messages page is where a balloon that went by too fast can be
		// read again, and the moment to want that is while looking at the pet.
		menuItem{
			label:  tr("Messages…"),
			rule:   true,
			closes: true,
			action: func() error { return browser.Open("http://" + cfg.Server.Addr + "/messages") },
		},
		menuItem{
			label:  tr("Settings…"),
			closes: true,
			action: func() error { return browser.Open("http://" + cfg.Server.Addr + "/") },
		},
		menuItem{
			label:  tr("Pet"),
			detail: petLabel(tr, cfg.Pet.Source),
			rule:   true,
			action: func() error {
				return g.store.Update(func(c *config.Config) {
					c.Pet.Source = nextPet(c.Pet.Source)
				})
			},
		},
		menuItem{
			label:  tr("Walk"),
			detail: roamLabel(tr, cfg.Behavior.Roam),
			action: func() error {
				return g.store.Update(func(c *config.Config) {
					c.Behavior.Roam = nextRoam(c.Behavior.Roam)
				})
			},
		},
	)
	if g.restart != nil {
		m.items = append(m.items, menuItem{
			label:  tr("Restart"),
			rule:   true,
			closes: true,
			action: func() error {
				g.restart()
				return nil
			},
		})
	}
	m.items = append(m.items, menuItem{
		label:  tr("Quit"),
		rule:   g.restart == nil,
		closes: true,
		action: func() error { return ebiten.Termination },
	})

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
		g.menu, g.hovered, g.reading = nil, false, false
		g.cursor.Reset()
		// A gesture in progress cannot be finished by a window that has
		// stopped receiving the mouse, so it is abandoned rather than left to
		// be completed by whatever the next press happens to be.
		g.endDrag(false)
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
	// Ask the window where it is now rather than using what it was last told
	// to be: the cursor is reported relative to where it actually sits, and
	// this runs a step before the window is moved again. A tick of difference
	// between the two is a tick of the pet's speed, which at any brisk setting
	// is more than the threshold below — it would read as the cursor lurching
	// every time the pet started or stopped.
	winX, winY := ebiten.WindowPosition()
	g.hovered = g.cursor.Update(
		float64(winX)+px, float64(winY)+py,
		g.overWindow(px, py),
		onPet || onBalloon >= 0,
		dt,
	)
	g.reading = g.hovered && onBalloon >= 0

	// The cursor in monitor pixels, which is what a drag has to work in: the
	// window moves with the pet and again with the drag, so a point inside it
	// is not a fixed place on the screen.
	mx, my := float64(winX)+px, float64(winY)+py
	if done, err := g.handleDrag(mx, my, onPet, onBalloon); done {
		return err
	}

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
	// A right-click copies the balloon under it, leaving the left button to do
	// what it always has. A balloon is up for seconds, and what it says is
	// often something to use rather than read: a path, an error, a question
	// to paste an answer to.
	if onBalloon >= 0 && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		g.copyBalloon(onBalloon)
		return nil
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

	// A link is checked before the balloon it sits in, because the balloon's
	// own click takes it down: without this, following a link would always be
	// the same gesture as throwing the message away.
	if onBalloon >= 0 {
		// A message still being said is finished rather than dismissed. Half a
		// sentence is not something anyone means to throw away, and waiting
		// for the pet to catch up before you may act on it would make the
		// effect an obstacle.
		if g.showing[onBalloon].stillTyping() {
			g.revealAll(onBalloon)
			return nil
		}
		if url := g.linkAt(onBalloon, px, py); url != "" {
			g.openLink(url)
			return nil
		}
		// Clicking a balloon takes it down, so a message that has been read
		// does not have to be waited out.
		g.dismiss(onBalloon)
		return nil
	}

	// Pressing the pet starts a gesture rather than opening the menu. Which it
	// was is only known at release: see [Game.handleDrag].
	return nil
}

// handleDrag moves the stage while the pet is held, and reports whether it has
// dealt with this tick's input.
//
// The pet's menu opens on release rather than on press, because until the
// button comes up there is no telling whether this was a click or the start of
// a drag. Balloons are left alone: they are dismissed on press as before, so
// only the pet itself can be grabbed.
func (g *Game) handleDrag(mx, my float64, onPet bool, onBalloon int) (done bool, err error) {
	// A stage filling the monitor has nowhere to be dragged to, so there the
	// pet itself is carried instead, and set down to walk on from where it was
	// left. Anywhere else the whole stage moves, and the pet with it.
	carryPet := g.cfg.Stage.Fullscreen

	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		wasPress := g.drag.Pressed()
		if wasDrag := g.drag.Release(); wasDrag {
			if carryPet {
				// Nothing to save: where a roaming pet stands is not a setting.
				g.endDrag(true)
			} else {
				g.saveDraggedStage()
			}
			return true, nil
		}
		if wasPress {
			// A press and release that went nowhere is a click after all.
			g.menu = g.buildMenu()
			g.menuIdle = 0
			return true, nil
		}
	}

	// Driven from Pressed, not Dragging: Dragging cannot become true until
	// Move has been called, so asking it here first is a loop that never
	// starts — which is exactly what it did.
	if g.drag.Pressed() {
		st := g.stage()
		if carryPet {
			px, py := g.walker.Pos()
			x, y, dragging := g.drag.Move(mx, my,
				drag.Rect{X: px, Y: py, W: g.petWidth(), H: g.petHeight()},
				drag.Rect{W: st.W, H: st.H})
			if dragging {
				g.walker.Translate(x-px, y-py)
			}
			return true, nil
		}
		x, y, dragging := g.drag.Move(mx, my,
			drag.Rect{X: st.X, Y: st.Y, W: st.W, H: st.H},
			drag.Rect{W: g.monitor.W, H: g.monitor.H})
		if dragging {
			// Carry the pet with the stage. Reshaping alone would only move
			// the walls it walks between, so the pet would sit still until an
			// edge pushed it — the window would lag behind the cursor.
			was := g.stage()
			g.dragActive = true
			g.dragX, g.dragY = x, y
			g.walker.Translate(x-was.X, y-was.Y)
			g.reshapeWalker()
		}
		return true, nil
	}

	// Every press on the pet starts a gesture: the menu opens when one ends,
	// so a press that started none could never be a click. What is carried —
	// the pet or its stage — only decides what moving does.
	if g.menu == nil && onBalloon < 0 && onPet &&
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if carryPet {
			px, py := g.walker.Pos()
			g.drag.Press(mx, my, drag.Rect{X: px, Y: py, W: g.petWidth(), H: g.petHeight()})
		} else {
			st := g.stage()
			g.drag.Press(mx, my, drag.Rect{X: st.X, Y: st.Y, W: st.W, H: st.H})
		}
		return true, nil
	}
	return false, nil
}

// saveDraggedStage writes where the pet was put down. It goes through the
// shared store like every other settings change, so the settings page sees it
// too rather than the two of them disagreeing about where the pet lives.
func (g *Game) saveDraggedStage() {
	x, y := int(math.Round(g.dragX)), int(math.Round(g.dragY))
	g.endDrag(true)

	if err := g.store.Update(func(c *config.Config) {
		c.Stage.Anchor = config.AnchorCustom
		c.Stage.X, c.Stage.Y = x, y
	}); err != nil {
		g.log.Error("could not save where the pet was dragged to", "error", err)
	}
}

// endDrag clears the drag state. keep says the position has been saved and the
// walker will be reshaped by the settings coming back; otherwise the stage
// snaps back to what the settings say.
func (g *Game) endDrag(keep bool) {
	g.drag.Cancel()
	g.dragActive = false
	if !keep {
		g.reshapeWalker()
	}
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

// linkAt returns the URL under a point in balloon i, or "" if the point is not
// on a link.
func (g *Game) linkAt(i int, px, py float64) string {
	if i < 0 || i >= len(g.showing) || i >= len(g.placed) {
		return ""
	}
	b := g.showing[i].balloon
	if b == nil {
		return ""
	}
	x := g.win.PanelX + g.placed[i].X
	y := g.win.PanelY + g.placed[i].Y

	for _, l := range b.links() {
		if px >= x+l.x && px < x+l.x+l.w && py >= y+l.y && py < y+l.y+l.h {
			return l.url
		}
	}
	return ""
}

// copyBalloon puts balloon i on the clipboard, and lights its border if that
// worked. It does not take the balloon down: copying is for using what it
// says, and the text should still be there to check against.
//
// The copy runs here rather than in the background because every clipboard
// command reads its input and returns at once, and doing it in line is what
// lets the flash mean it actually worked. [clipboard.Timeout] covers the case
// where it does not return.
func (g *Game) copyBalloon(i int) {
	if err := clipboard.Write(g.showing[i].msg.Copied()); err != nil {
		g.log.Warn("could not copy a message", "error", err)
		return
	}
	g.showing[i].flash = copyFlash
}

// openLink hands a URL to the browser, if the settings allow it and the URL is
// one gumpet is prepared to open at all.
//
// Anything that can reach the HTTP API can put a link in front of whoever is at
// this desktop. A click is required, which is most of the protection, but the
// scheme check is what keeps that click from doing anything other than opening
// a web page.
func (g *Game) openLink(url string) {
	if !g.cfg.Message.OpenLinks {
		g.log.Info("not opening a link, message.open_links is off", "url", url)
		return
	}
	if !richtext.Openable(url) {
		g.log.Warn("refusing to open a link that is not http or https", "url", url)
		return
	}
	if err := browser.Open(url); err != nil {
		g.log.Error("could not open a link", "url", url, "error", err)
	}
}

// nextPet steps through the bundled pets. Artwork of one's own is left alone:
// cycling away from a path the user typed in would lose it, and there is no
// sensible place in the ring to put it back.
func nextPet(source string) string {
	if source != "" {
		if _, ok := petsrc.BuiltinNamed(source); !ok {
			return source
		}
	}
	current := source
	if current == "" {
		current = petsrc.Builtins[0].Name
	}
	for i, p := range petsrc.Builtins {
		if p.Name == current {
			return petsrc.Builtins[(i+1)%len(petsrc.Builtins)].Name
		}
	}
	return petsrc.Builtins[0].Name
}

// petLabel names what is showing, for the menu's right-hand column.
//
// The bundled pets' names are names, and stay as they are in every language.
func petLabel(tr func(string) string, source string) string {
	if source == "" {
		return petsrc.Builtins[0].Label
	}
	if p, ok := petsrc.BuiltinNamed(source); ok {
		return p.Label
	}
	return tr("custom")
}

// roamLabel names a way of roaming. In English it is the setting's own value,
// as it always was; other languages say what it means.
func roamLabel(tr func(string) string, r config.Roam) string {
	switch r {
	case config.RoamNone:
		return tr("none")
	case config.RoamPerimeter:
		return tr("perimeter")
	case config.RoamWander:
		return tr("wander")
	default:
		return tr("horizontal")
	}
}

func nextRoam(r config.Roam) config.Roam {
	for i, candidate := range config.Roams {
		if candidate == r {
			return config.Roams[(i+1)%len(config.Roams)]
		}
	}
	return config.RoamHorizontal
}
