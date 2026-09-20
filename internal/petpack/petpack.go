// Package petpack turns a pet's artwork into the textures gumpet draws.
//
// The reading and decoding is [petsrc]'s job; this is only the part that needs
// a graphics context.
package petpack

import (
	"image"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/kaakaa/gumpet/internal/petsrc"
)

// Frame is one drawable image plus how long it stays up. A duration of zero
// means the configured frame rate applies.
type Frame struct {
	Image    *ebiten.Image
	Duration time.Duration
}

// Pack is everything gumpet needs to draw one pet.
type Pack struct {
	// Name identifies the pack in log messages.
	Name string
	// Walk is the idle/walking animation, and always has at least one frame.
	Walk []Frame
	// Talk is played while a message is up. It may be empty, in which case
	// Walk keeps playing.
	Talk []Frame
	// Size is the size of the artwork in source pixels.
	Size image.Point
	// Scale and Smooth are the artwork's own idea of how it should be drawn:
	// how much to enlarge it before pet.scale applies, and whether enlarging
	// it should be smoothed. See [petsrc.Source].
	Scale  float64
	Smooth bool
}

// TalkFrames returns the animation to play while a message is up.
func (p *Pack) TalkFrames() []Frame {
	if len(p.Talk) > 0 {
		return p.Talk
	}
	return p.Walk
}

// Load reads the artwork named by a config's pet.source and uploads it.
func Load(source string) (*Pack, error) {
	src, err := petsrc.Load(source)
	if err != nil {
		return nil, err
	}
	return &Pack{
		Name:   src.Name,
		Walk:   toFrames(src.Walk),
		Talk:   toFrames(src.Talk),
		Size:   src.Size,
		Scale:  src.Scale,
		Smooth: src.Smooth,
	}, nil
}

func toFrames(in []petsrc.Frame) []Frame {
	if len(in) == 0 {
		return nil
	}
	out := make([]Frame, 0, len(in))
	for _, f := range in {
		out = append(out, Frame{Image: ebiten.NewImageFromImage(f.Image), Duration: f.Duration})
	}
	return out
}
