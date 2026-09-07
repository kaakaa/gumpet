package pet

import (
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Balloon and menu geometry, in logical pixels.
const (
	balloonPadding = 10.0
	balloonRadius  = 10.0
	balloonStroke  = 2.0
	tailWidth      = 16.0
	tailHeight     = 10.0
)

var (
	panelFill   = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xf7}
	panelBorder = color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xff}
	textColor   = color.NRGBA{R: 0x1a, G: 0x1a, B: 0x1a, A: 0xff}
	mutedColor  = color.NRGBA{R: 0x70, G: 0x70, B: 0x70, A: 0xff}
	hoverColor  = color.NRGBA{R: 0x00, G: 0xad, B: 0xd8, A: 0x33}
	ruleColor   = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x22}
	debugBorder = color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0x66}
)

// balloon is a message wrapped and measured, ready to draw.
type balloon struct {
	lines []string
	// width and height include the padding around the text.
	width, height float64
	textW, textH  float64
}

// currentBalloon lays out the message on screen, reusing the last layout until
// the message or the settings change.
func (g *Game) currentBalloon() *balloon {
	if g.balloon != nil {
		return g.balloon
	}
	scale := g.cfg.Message.TextScale
	// The balloon may not be wider than the monitor, whatever the setting says.
	maxWidth := math.Min(float64(g.cfg.Message.MaxWidth), g.monitor.W)
	maxText := math.Max(maxWidth-2*balloonPadding, 1)

	lines := g.wrap(g.current.Text, maxText/scale)
	b := &balloon{
		lines: lines,
		textW: g.blockWidth(lines) * scale,
		textH: float64(len(lines)) * lineHeight(g.face) * scale,
	}
	b.width = b.textW + 2*balloonPadding
	b.height = b.textH + 2*balloonPadding
	g.balloon = b
	return b
}

// Draw paints the pet, and whatever is above it. Everything it does not paint
// stays transparent, which is how the pet appears to sit on the desktop.
func (g *Game) Draw(screen *ebiten.Image) {
	if g.debug {
		g.drawWindowBounds(screen)
	}
	if g.hidden() {
		return
	}
	g.drawPet(screen)
	switch {
	case g.menu != nil:
		g.drawMenu(screen)
	case g.current != nil:
		g.drawBalloon(screen)
	}
}

func (g *Game) drawPet(screen *ebiten.Image) {
	frames := g.frames()
	frame := frames[g.frameIdx%len(frames)]

	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	if g.walker.FacingRight() && g.cfg.Pet.FlipWhenFacingRight {
		op.GeoM.Scale(-1, 1)
		op.GeoM.Translate(float64(frame.Image.Bounds().Dx()), 0)
	}
	s := g.cfg.Pet.Scale * g.deviceScale
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(g.win.PetX*g.deviceScale, g.win.PetY*g.deviceScale)
	screen.DrawImage(frame.Image, op)
}

func (g *Game) drawBalloon(screen *ebiten.Image) {
	b := g.currentBalloon()
	ds := g.deviceScale
	x, y := g.win.PanelX, g.win.PanelY

	path := balloonPath(
		float32(x*ds), float32(y*ds), float32(b.width*ds), float32(b.height*ds),
		float32(balloonRadius*ds), float32(g.win.TailX*ds),
		float32(tailWidth*ds), float32(tailHeight*ds),
	)
	fillAndStroke(screen, path, ds)

	g.drawText(screen, b.lines, (x+balloonPadding)*ds, (y+balloonPadding)*ds,
		g.cfg.Message.TextScale*ds, textColor)
}

func (g *Game) drawMenu(screen *ebiten.Image) {
	m := g.menu
	ds := g.deviceScale
	x, y := g.win.PanelX, g.win.PanelY

	fillAndStroke(screen, roundedRectPath(
		float32(x*ds), float32(y*ds), float32(m.width*ds), float32(m.height*ds),
		float32(menuRadius*ds),
	), ds)

	for i, it := range m.items {
		rx, ry, rw, rh := m.rowRect(x, y, i)

		if it.rule {
			vector.StrokeLine(screen,
				float32(rx*ds), float32(ry*ds), float32((rx+rw)*ds), float32(ry*ds),
				float32(ds), ruleColor, false)
		}
		if i == m.hover && it.action != nil {
			vector.DrawFilledRect(screen,
				float32(rx*ds), float32(ry*ds), float32(rw*ds), float32(rh*ds),
				hoverColor, false)
		}

		labelColor := textColor
		if it.action == nil {
			labelColor = mutedColor
		}
		textY := ry + menuRowPadY
		g.drawText(screen, []string{it.label}, rx*ds, textY*ds, menuTextScale*ds, labelColor)

		if it.detail != "" {
			w := g.blockWidth([]string{it.detail}) * menuTextScale
			g.drawText(screen, []string{it.detail}, (rx+rw-w)*ds, textY*ds, menuTextScale*ds, mutedColor)
		}
	}
}

// drawText draws lines with their top-left corner at (x, y) in physical pixels.
// scale converts font units to physical pixels.
func (g *Game) drawText(screen *ebiten.Image, lines []string, x, y, scale float64, clr color.Color) {
	op := &text.DrawOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(x, y)
	// A bitmap font scaled by a whole number stays crisp; smoothing only blurs
	// it.
	op.Filter = ebiten.FilterNearest
	op.ColorScale.ScaleWithColor(clr)
	op.LineSpacing = lineHeight(g.face)
	text.Draw(screen, strings.Join(lines, "\n"), g.face, op)
}

// drawWindowBounds outlines the window, so GUMPET_DEBUG=1 shows exactly how
// much of the screen the pet is covering.
func (g *Game) drawWindowBounds(screen *ebiten.Image) {
	ds := g.deviceScale
	vector.StrokeRect(screen, 0, 0,
		float32(g.win.W*ds), float32(g.win.H*ds), float32(ds), debugBorder, false)
}

func fillAndStroke(screen *ebiten.Image, path *vector.Path, ds float64) {
	fill := &vector.DrawPathOptions{AntiAlias: true}
	fill.ColorScale.ScaleWithColor(panelFill)
	vector.FillPath(screen, path, &vector.FillOptions{FillRule: vector.FillRuleNonZero}, fill)

	stroke := &vector.DrawPathOptions{AntiAlias: true}
	stroke.ColorScale.ScaleWithColor(panelBorder)
	vector.StrokePath(screen, path, &vector.StrokeOptions{
		Width:    float32(balloonStroke * ds),
		LineJoin: vector.LineJoinRound,
		LineCap:  vector.LineCapRound,
	}, stroke)
}

func roundedRectPath(x, y, w, h, radius float32) *vector.Path {
	radius = clampRadius(radius, w, h)

	var p vector.Path
	p.MoveTo(x+radius, y)
	p.LineTo(x+w-radius, y)
	p.ArcTo(x+w, y, x+w, y+radius, radius)
	p.LineTo(x+w, y+h-radius)
	p.ArcTo(x+w, y+h, x+w-radius, y+h, radius)
	p.LineTo(x+radius, y+h)
	p.ArcTo(x, y+h, x, y+h-radius, radius)
	p.LineTo(x, y+radius)
	p.ArcTo(x, y, x+radius, y, radius)
	p.Close()
	return &p
}

// balloonPath traces a rounded rectangle with a tail hanging off the bottom
// edge at tailX, as one closed outline so the fill and stroke stay seamless.
func balloonPath(x, y, w, h, radius, tailX, tailW, tailH float32) *vector.Path {
	radius = clampRadius(radius, w, h)
	// Keep the tail clear of the corners, and inside the balloon.
	lo, hi := x+radius+tailW/2, x+w-radius-tailW/2
	if hi < lo {
		lo, hi = x+w/2, x+w/2
	}
	tailX = float32(math.Min(math.Max(float64(tailX), float64(lo)), float64(hi)))

	var p vector.Path
	p.MoveTo(x+radius, y)
	p.LineTo(x+w-radius, y)
	p.ArcTo(x+w, y, x+w, y+radius, radius)
	p.LineTo(x+w, y+h-radius)
	p.ArcTo(x+w, y+h, x+w-radius, y+h, radius)
	p.LineTo(tailX+tailW/2, y+h)
	p.LineTo(tailX, y+h+tailH)
	p.LineTo(tailX-tailW/2, y+h)
	p.LineTo(x+radius, y+h)
	p.ArcTo(x, y+h, x, y+h-radius, radius)
	p.LineTo(x, y+radius)
	p.ArcTo(x, y, x+radius, y, radius)
	p.Close()
	return &p
}

func clampRadius(radius, w, h float32) float32 {
	return float32(math.Max(0, math.Min(float64(radius), math.Min(float64(w), float64(h))/2)))
}
