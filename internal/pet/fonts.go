package pet

import (
	"bytes"
	"fmt"
	"os"

	"github.com/hajimehoshi/bitmapfont/v4"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/text/language"

	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/fontfile"
	"github.com/kaakaa/gumpet/internal/textwrap"
)

// baseSize is the size text is drawn at when the scale is 1. It matches the
// bundled bitmap font, so switching fonts does not change how big anything is.
const baseSize = 12

// fontFace is a face together with what it takes to get from the face's own
// units to the screen.
//
// The two kinds of face need opposite treatment. A bitmap face has one fixed
// size and has to be scaled up by the drawing code, which is what makes it
// blocky. An outline face is instead built at the exact pixel size wanted, and
// drawn without scaling at all — which is what makes it smooth.
type fontFace struct {
	face text.Face
	// unit is how many logical pixels one of the face's own units is worth.
	unit float64
	// draw is the scale to draw the face at, given a position already in
	// physical pixels.
	draw float64
}

// Advance is the width of s in logical pixels, which is what makes fontFace a
// [textwrap.Measurer].
func (f fontFace) Advance(s string) float64 {
	return text.Advance(s, f.face) * f.unit
}

// lineHeight is the baseline-to-baseline distance in logical pixels.
func (f fontFace) lineHeight() float64 {
	return f.faceLineHeight() * f.unit
}

// faceLineHeight is the same distance in the face's own units, which is what
// [text.DrawOptions.LineSpacing] wants.
func (f fontFace) faceLineHeight() float64 {
	m := f.face.Metrics()
	return m.HAscent + m.HDescent + m.HLineGap
}

// fonts are the faces gumpet is currently drawing with, and what they were
// built for. Rebuilding them is only worth doing when one of those changes.
type fonts struct {
	// name is the font in use, for the log.
	name    string
	message fontFace
	menu    fontFace
	// stamp is the timestamp in the corner of a balloon, which follows the
	// message size so that turning the text up does not leave it behind.
	stamp fontFace

	source      *text.GoTextFaceSource
	path        string
	system      bool
	textScale   float64
	deviceScale float64
	built       bool
}

// ensureFonts rebuilds the faces when the settings or the display have changed
// under them.
func (g *Game) ensureFonts() {
	want := g.cfg.Font
	if g.fonts.built &&
		g.fonts.path == want.Path &&
		g.fonts.system == want.System &&
		g.fonts.textScale == g.cfg.Message.TextScale &&
		g.fonts.deviceScale == g.deviceScale {
		return
	}

	source, name := g.fontSource(want)
	g.fonts = fonts{
		name:        name,
		source:      source,
		path:        want.Path,
		system:      want.System,
		textScale:   g.cfg.Message.TextScale,
		deviceScale: g.deviceScale,
		built:       true,
	}
	g.fonts.message = g.buildFace(source, g.cfg.Message.TextScale)
	g.fonts.menu = g.buildFace(source, menuTextScale)
	g.fonts.stamp = g.buildFace(source, g.cfg.Message.TextScale*stampTextScale)
	g.panelDirty = true
	if g.menu != nil {
		g.menu = g.buildMenu()
	}
	g.log.Info("using font", "font", name, "size", baseSize*g.cfg.Message.TextScale)
}

// fontSource loads the configured or discovered font. Anything that goes wrong
// is reported and then shrugged off: a pet that will not start because of a
// font is worse than a blocky one.
func (g *Game) fontSource(cfg config.Font) (*text.GoTextFaceSource, string) {
	path, err := fontfile.Resolve(cfg.Path, cfg.System)
	if err != nil {
		g.log.Error("could not use the configured font, falling back to the bundled one", "error", err)
		return nil, bundledFontName
	}
	if path == "" {
		return nil, bundledFontName
	}
	source, err := loadFontSource(path)
	if err != nil {
		g.log.Error("could not read the font, falling back to the bundled one", "path", path, "error", err)
		return nil, bundledFontName
	}
	return source, path
}

const bundledFontName = "bundled bitmap font"

// buildFace makes a face for one scale. An outline face is built at the exact
// size it will be drawn at, including the display's own scaling, so nothing
// ever has to enlarge a rendered glyph.
func (g *Game) buildFace(source *text.GoTextFaceSource, scale float64) fontFace {
	if source == nil {
		return fontFace{
			face: text.NewGoXFace(bitmapfont.FaceEA),
			unit: scale,
			draw: scale * g.deviceScale,
		}
	}
	return fontFace{
		face: &text.GoTextFace{
			Source:   source,
			Size:     baseSize * scale * g.deviceScale,
			Language: language.Japanese,
		},
		unit: 1 / g.deviceScale,
		draw: 1,
	}
}

// loadFontSource reads a font file. Collections hold several faces; the first
// is the regular weight in every collection gumpet looks for.
func loadFontSource(path string) (*text.GoTextFaceSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if sources, err := text.NewGoTextFaceSourcesFromCollection(bytes.NewReader(data)); err == nil && len(sources) > 0 {
		return sources[0], nil
	}
	source, err := text.NewGoTextFaceSource(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	return source, nil
}

func (g *Game) wrap(f fontFace, s string, maxWidth float64) []string {
	return textwrap.Wrap(s, f, maxWidth)
}

func (g *Game) blockWidth(f fontFace, lines []string) float64 {
	return textwrap.BlockWidth(lines, f)
}
