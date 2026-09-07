package pet

import (
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/kaakaa/gumpet/internal/textwrap"
)

// faceMeasurer adapts an Ebitengine font face to textwrap.Measurer.
type faceMeasurer struct {
	face text.Face
}

func (f faceMeasurer) Advance(s string) float64 { return text.Advance(s, f.face) }

// lineHeight is the baseline-to-baseline distance for face.
func lineHeight(face text.Face) float64 {
	m := face.Metrics()
	return m.HAscent + m.HDescent + m.HLineGap
}

func (g *Game) wrap(s string, maxWidth float64) []string {
	return textwrap.Wrap(s, faceMeasurer{g.face}, maxWidth)
}

func (g *Game) blockWidth(lines []string) float64 {
	return textwrap.BlockWidth(lines, faceMeasurer{g.face})
}
