// Package gifseq flattens an animated GIF into standalone frames.
//
// GIF frames are usually partial updates governed by a disposal rule, so each
// one has to be composited onto a running canvas before it can be drawn on its
// own. This is kept apart from the rendering code so it can be tested without a
// graphics context.
package gifseq

import (
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"io"
	"time"
)

// defaultDelay is what a delay of 0 or 1 hundredths means in practice: those
// are how "as fast as possible" is written in the wild, and browsers render
// them at 100ms.
const defaultDelay = 100 * time.Millisecond

// Frame is one fully composited image and how long it stays up.
type Frame struct {
	Image    image.Image
	Duration time.Duration
}

// Decode reads an animated GIF and returns its frames, each one a complete
// picture of the canvas at that point in the animation.
func Decode(r io.Reader) ([]Frame, error) {
	g, err := gif.DecodeAll(r)
	if err != nil {
		return nil, fmt.Errorf("decode gif: %w", err)
	}
	if len(g.Image) == 0 {
		return nil, fmt.Errorf("gif has no frames")
	}

	bounds := image.Rect(0, 0, g.Config.Width, g.Config.Height)
	if bounds.Empty() {
		bounds = g.Image[0].Bounds()
	}
	canvas := image.NewRGBA(bounds)

	frames := make([]Frame, 0, len(g.Image))
	for i, src := range g.Image {
		var saved *image.RGBA
		if disposal(g, i) == gif.DisposalPrevious {
			saved = clone(canvas)
		}

		draw.Draw(canvas, src.Bounds(), src, src.Bounds().Min, draw.Over)
		frames = append(frames, Frame{Image: clone(canvas), Duration: delay(g, i)})

		switch disposal(g, i) {
		case gif.DisposalBackground:
			draw.Draw(canvas, src.Bounds(), image.Transparent, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			draw.Draw(canvas, canvas.Bounds(), saved, canvas.Bounds().Min, draw.Src)
		}
	}
	return frames, nil
}

func clone(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	draw.Draw(dst, src.Bounds(), src, src.Bounds().Min, draw.Src)
	return dst
}

func disposal(g *gif.GIF, i int) byte {
	if i >= len(g.Disposal) {
		return gif.DisposalNone
	}
	return g.Disposal[i]
}

func delay(g *gif.GIF, i int) time.Duration {
	if i >= len(g.Delay) || g.Delay[i] <= 1 {
		return defaultDelay
	}
	return time.Duration(g.Delay[i]) * 10 * time.Millisecond
}
