package pet

import (
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/kaakaa/gumpet/internal/layout"
	"github.com/kaakaa/gumpet/internal/message"
	"github.com/kaakaa/gumpet/internal/richtext"
)

// Balloon and menu geometry, in logical pixels.
const (
	balloonPadding = 10.0
	balloonRadius  = 10.0
	balloonStroke  = 2.0
	tailWidth      = 16.0
	tailHeight     = 10.0
	// balloonSideStep and balloonStackGap set how a pile of balloons zigzags,
	// so that several at once read as a crowd rather than a list.
	balloonSideStep = 26.0
	balloonStackGap = 5.0
	// titleGap separates a message's heading from its text.
	titleGap = 3.0
	// stampTextScale shrinks the timestamp relative to the message. It is
	// chrome rather than content: enough to read when looked for, not enough
	// to compete with what the pet is actually saying.
	stampTextScale = 0.7
	// stampGap keeps the timestamp clear of a heading sharing its line.
	stampGap = 10.0
	// copyFlash is how long a copied balloon's border stays lit. Long enough
	// to catch from the corner of an eye, short enough not to look like a
	// change of level.
	copyFlash = 700 * time.Millisecond
	// underlineDrop is how far below the baseline box a link's underline sits.
	underlineDrop = 1.0
)

var (
	panelFill   = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xf7}
	panelBorder = color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xff}
	textColor   = color.NRGBA{R: 0x1a, G: 0x1a, B: 0x1a, A: 0xff}
	// seenColor is the text of a headline the pet has said before: still
	// readable, but plainly not the news the dark text is.
	seenColor   = color.NRGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xff}
	mutedColor  = color.NRGBA{R: 0x70, G: 0x70, B: 0x70, A: 0xff}
	hoverColor  = color.NRGBA{R: 0x00, G: 0xad, B: 0xd8, A: 0x33}
	ruleColor   = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x22}
	debugBorder = color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0x66}
	linkColor   = color.NRGBA{R: 0x00, G: 0x5f, B: 0xa8, A: 0xff}
	// stampColor is fainter than mutedColor, and stays the same whatever the
	// level: when a message arrived says nothing about how loud it is.
	stampColor = color.NRGBA{R: 0x70, G: 0x70, B: 0x70, A: 0xaa}
	// copiedColor is the gopher's own blue, which no level uses.
	copiedColor = color.NRGBA{R: 0x00, G: 0xad, B: 0xd8, A: 0xff}
)

// levelColors give each level a border for its balloon and a colour for its
// heading. Info keeps the ordinary border, so a message that says nothing
// about itself looks exactly as it always has.
var levelColors = map[message.Level]struct{ border, accent color.NRGBA }{
	message.LevelInfo:    {panelBorder, mutedColor},
	message.LevelSuccess: {color.NRGBA{R: 0x1e, G: 0x7a, B: 0x43, A: 0xff}, color.NRGBA{R: 0x16, G: 0x65, B: 0x3a, A: 0xff}},
	message.LevelWarn:    {color.NRGBA{R: 0xb5, G: 0x7d, B: 0x11, A: 0xff}, color.NRGBA{R: 0x8a, G: 0x5a, B: 0x10, A: 0xff}},
	message.LevelError:   {color.NRGBA{R: 0xc0, G: 0x2c, B: 0x2c, A: 0xff}, color.NRGBA{R: 0x9b, G: 0x1c, B: 0x1c, A: 0xff}},
}

func colorsFor(level message.Level) (border, accent color.NRGBA) {
	c, ok := levelColors[level]
	if !ok {
		c = levelColors[message.LevelInfo]
	}
	return c.border, c.accent
}

// balloon is a message wrapped and measured, ready to draw.
type balloon struct {
	// seen is a headline said before, drawn fainter. See [message.Message.Seen].
	seen bool
	// title is the heading above the text, empty for most messages.
	title []richtext.Line
	lines []richtext.Line
	// width and height include the padding around the text.
	width, height float64
	textW, textH  float64
	// headH is the height of the heading row — the title, the timestamp, or
	// both — and the gap under it, or zero when there is neither.
	headH float64
	// stamp is when the message arrived or when its feed dated it, drawn small
	// and faint at the end of the heading row. Empty when nothing is known.
	stamp string
	// stampX and stampY place it relative to the balloon's top-left corner.
	stampX, stampY float64
	// lineH is the baseline-to-baseline distance the lines were laid out at,
	// kept so that a click can be turned back into a line number.
	lineH float64
	// runes is how many characters the message has, which is how far the
	// typing has to get. The heading is not counted: it names where the
	// message came from, and a source revealed a letter at a time would be
	// useless until it finished.
	runes          int
	border, accent color.NRGBA
}

// layoutBalloon wraps and measures one message.
func (g *Game) layoutBalloon(msg message.Message) *balloon {
	f := g.fonts.message
	// The balloon may not be wider than the monitor, whatever the setting says.
	maxWidth := math.Min(float64(g.cfg.Message.MaxWidth), g.monitor.W)
	maxText := math.Max(maxWidth-2*balloonPadding, 1)

	lines := richtext.Wrap(richtext.Parse(msg.Text), f, maxText)
	b := &balloon{
		lines: lines,
		lineH: f.lineHeight(),
		textW: richtext.BlockWidth(lines),
		textH: float64(len(lines)) * f.lineHeight(),
	}
	b.runes = richtext.Runes(lines)
	b.border, b.accent = colorsFor(msg.Level)

	// The stamp is measured before the heading is wrapped, because it shares
	// that line: what it takes is not available to wrap the heading into.
	b.stamp = message.Stamp(msg.At, time.Now())
	b.seen = msg.Seen
	if msg.Seen {
		// Said in the corner with the time, where it costs no room: a repeat
		// is worth knowing about, not worth a line of its own.
		if b.stamp == "" {
			b.stamp = g.tr("seen")
		} else {
			b.stamp = g.tr("seen") + " · " + b.stamp
		}
	}
	var stampW, headW, headLineH float64
	if b.stamp != "" {
		stampW = g.fonts.stamp.Advance(b.stamp)
		headW, headLineH = stampW, g.fonts.stamp.lineHeight()
	}

	if msg.Title != "" {
		// A heading is not scanned for links: it names where the message came
		// from, and a sender that wants a link puts it in the text.
		titleMax := maxText
		if b.stamp != "" {
			titleMax = math.Max(maxText-stampGap-stampW, 1)
		}
		b.title = richtext.Wrap([]richtext.Span{{Text: msg.Title}}, f, titleMax)
		headLineH = math.Max(headLineH, float64(len(b.title))*f.lineHeight())
		headW = richtext.BlockWidth(b.title)
		if b.stamp != "" {
			headW += stampGap + stampW
		}
	}
	if headLineH > 0 {
		b.headH = headLineH + titleGap
	}
	if headW > b.textW {
		b.textW = headW
	}

	b.width = b.textW + 2*balloonPadding
	b.height = b.headH + b.textH + 2*balloonPadding

	if b.stamp != "" {
		b.stampX = b.width - balloonPadding - stampW
		// Sit the stamp on the baseline of the heading's *first* line, rather
		// than its own top or the bottom of a heading that wrapped: a smaller
		// face aligned at the top floats above the words beside it, and one
		// aligned at the bottom drifts away from them entirely.
		firstLineH := g.fonts.stamp.lineHeight()
		if len(b.title) > 0 {
			firstLineH = f.lineHeight()
		}
		b.stampY = balloonPadding + firstLineH - g.fonts.stamp.lineHeight()
	}
	return b
}

// linkRect is where one link sits inside a balloon, relative to the balloon's
// top-left corner, together with where it goes.
type linkRect struct {
	x, y, w, h float64
	url        string
}

// links lists every clickable piece of the balloon. It is worked out from the
// same numbers the text is drawn with, so what is underlined and what can be
// clicked cannot drift apart.
func (b *balloon) links() []linkRect {
	var out []linkRect
	top := balloonPadding + b.headH
	for i, line := range b.lines {
		for _, run := range line.Runs {
			if !run.Style.IsLink() {
				continue
			}
			out = append(out, linkRect{
				x:   balloonPadding + run.X,
				y:   top + float64(i)*b.lineH,
				w:   run.Width,
				h:   b.lineH,
				url: run.Style.Link,
			})
		}
	}
	return out
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
	case len(g.showing) > 0:
		g.drawBalloons(screen)
	}
}

func (g *Game) drawPet(screen *ebiten.Image) {
	frames := g.frames()
	frame := frames[g.frameIdx%len(frames)]

	// Pixel art is enlarged without smoothing, for the same reason the bundled
	// bitmap font is: the squares are the drawing, and smoothing them only
	// blurs what was deliberate. The artwork says which it is, and the setting
	// can overrule it for artwork gumpet did not ship.
	filter := ebiten.FilterLinear
	if !g.cfg.Pet.Smooth.Smoothed(g.pack.Smooth) {
		filter = ebiten.FilterNearest
	}

	op := &ebiten.DrawImageOptions{Filter: filter}
	// In "faded" mode an idle pet is drawn faint rather than not at all.
	op.ColorScale.ScaleAlpha(float32(g.petOpacity()))
	if g.walker.FacingRight() && g.cfg.Pet.FlipWhenFacingRight {
		op.GeoM.Scale(-1, 1)
		op.GeoM.Translate(float64(frame.Image.Bounds().Dx()), 0)
	}
	s := g.petScale() * g.deviceScale
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(g.win.PetX*g.deviceScale, g.win.PetY*g.deviceScale)
	screen.DrawImage(frame.Image, op)
}

// drawBalloons paints the stack. Only the bottom one gets a tail: several
// tails converging on one pet looks like a mistake rather than a crowd.
func (g *Game) drawBalloons(screen *ebiten.Image) {
	ds := g.deviceScale

	// Newest on top of older ones, so the most recent is never buried.
	for i := len(g.showing) - 1; i >= 0; i-- {
		b := g.showing[i].balloon
		if b == nil || i >= len(g.placed) {
			continue
		}
		x := g.win.PanelX + g.placed[i].X
		y := g.win.PanelY + g.placed[i].Y

		var path *vector.Path
		if i == 0 {
			// Keep the tail on the balloon it belongs to, wherever the stack
			// has ended up relative to the pet.
			tailX := layout.Clamp(g.win.TailX, x, x+b.width)
			path = balloonPath(
				float32(x*ds), float32(y*ds), float32(b.width*ds), float32(b.height*ds),
				float32(balloonRadius*ds), float32(tailX*ds),
				float32(tailWidth*ds), float32(tailHeight*ds),
			)
		} else {
			path = roundedRectPath(
				float32(x*ds), float32(y*ds), float32(b.width*ds), float32(b.height*ds),
				float32(balloonRadius*ds),
			)
		}
		border := b.border
		if g.showing[i].flash > 0 {
			border = copiedColor
		}
		fillAndStroke(screen, path, ds, border)

		if len(b.title) > 0 {
			g.drawRichText(screen, b.title, (x+balloonPadding)*ds, (y+balloonPadding)*ds,
				g.fonts.message, b.accent)
		}
		// The stamp appears whole from the start, like the heading: a time
		// revealed a digit at a time would be unreadable until it finished.
		if b.stamp != "" {
			g.drawText(screen, []string{b.stamp}, (x+b.stampX)*ds, (y+b.stampY)*ds,
				g.fonts.stamp, stampColor)
		}
		// The balloon was sized and wrapped for the whole message, so what is
		// drawn here is only the part said so far. Nothing moves as the rest
		// arrives.
		lines := b.lines
		if typed := int(g.showing[i].typed); typed < b.runes {
			lines = richtext.Reveal(lines, typed, g.fonts.message)
		}
		ink := textColor
		if b.seen {
			ink = seenColor
		}
		g.drawRichText(screen, lines, (x+balloonPadding)*ds, (y+balloonPadding+b.headH)*ds,
			g.fonts.message, ink)
	}
}

// drawRichText draws wrapped lines run by run, so that a link can be a
// different colour from the words either side of it. Each run is positioned
// from the offset the wrapper measured rather than from a running total, which
// keeps the underline under the text it belongs to.
func (g *Game) drawRichText(screen *ebiten.Image, lines []richtext.Line, x, y float64, f fontFace, clr color.Color) {
	ds := g.deviceScale
	lineH := f.lineHeight()

	for i, line := range lines {
		top := y + float64(i)*lineH*ds
		for _, run := range line.Runs {
			runColor := clr
			if run.Style.IsLink() {
				runColor = linkColor
			}
			g.drawText(screen, []string{run.Text}, x+run.X*ds, top, f, runColor)

			if run.Style.IsLink() {
				under := top + (lineH-underlineDrop)*ds
				vector.StrokeLine(screen,
					float32(x+run.X*ds), float32(under),
					float32(x+(run.X+run.Width)*ds), float32(under),
					float32(ds), linkColor, false)
			}
		}
	}
}

func (g *Game) drawMenu(screen *ebiten.Image) {
	m := g.menu
	ds := g.deviceScale
	x, y := g.win.PanelX, g.win.PanelY

	fillAndStroke(screen, roundedRectPath(
		float32(x*ds), float32(y*ds), float32(m.width*ds), float32(m.height*ds),
		float32(menuRadius*ds),
	), ds, panelBorder)

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
		g.drawText(screen, []string{it.label}, rx*ds, textY*ds, g.fonts.menu, labelColor)

		if it.detail != "" {
			w := g.blockWidth(g.fonts.menu, []string{it.detail})
			g.drawText(screen, []string{it.detail}, (rx+rw-w)*ds, textY*ds, g.fonts.menu, mutedColor)
		}
	}
}

// drawText draws lines with their top-left corner at (x, y) in physical pixels.
func (g *Game) drawText(screen *ebiten.Image, lines []string, x, y float64, f fontFace, clr color.Color) {
	op := &text.DrawOptions{}
	if f.draw != 1 {
		op.GeoM.Scale(f.draw, f.draw)
	}
	op.GeoM.Translate(x, y)
	// The bitmap font is the only one that gets enlarged, and enlarging it by a
	// whole number keeps it crisp; smoothing would only blur it. An outline
	// face is built at its final size and never scaled, so the filter is moot.
	op.Filter = ebiten.FilterNearest
	op.ColorScale.ScaleWithColor(clr)
	op.LineSpacing = f.faceLineHeight()
	text.Draw(screen, strings.Join(lines, "\n"), f.face, op)
}

// drawWindowBounds outlines the window, so GUMPET_DEBUG=1 shows exactly how
// much of the screen the pet is covering.
func (g *Game) drawWindowBounds(screen *ebiten.Image) {
	ds := g.deviceScale
	vector.StrokeRect(screen, 0, 0,
		float32(g.win.W*ds), float32(g.win.H*ds), float32(ds), debugBorder, false)
}

func fillAndStroke(screen *ebiten.Image, path *vector.Path, ds float64, border color.NRGBA) {
	fill := &vector.DrawPathOptions{AntiAlias: true}
	fill.ColorScale.ScaleWithColor(panelFill)
	vector.FillPath(screen, path, &vector.FillOptions{FillRule: vector.FillRuleNonZero}, fill)

	stroke := &vector.DrawPathOptions{AntiAlias: true}
	stroke.ColorScale.ScaleWithColor(border)
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
