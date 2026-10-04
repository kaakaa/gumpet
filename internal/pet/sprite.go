package pet

import (
	"time"

	"github.com/kaakaa/gumpet/internal/petpack"
)

func (g *Game) reshapeWalker() {
	g.walker.Reshape(g.cfg.Behavior.Roam, g.stage(), g.petWidth(), g.petHeight(), g.cfg.Behavior.Speed)
	g.walker.SetJumping(g.cfg.Behavior.Jump)
}

// canRoam reports whether the pet should be walking. It carries on walking
// while it talks — the balloons are drawn above it and travel with it — but
// holds still while its menu is open, so the rows stay under the cursor, and
// while someone is plainly reaching for it with the cursor.
func (g *Game) canRoam() bool {
	// A pet being held does not walk off. The cursor-over-the-pet check
	// usually covers that, but it lapses for a cursor held still, and a pet
	// that wandered out from under a held button would snap back as soon as
	// the cursor moved again.
	return g.menu == nil && !g.hovered && !g.drag.Pressed()
}

// advanceAnimation steps through the current animation. A pet that has stopped
// to look around holds its first frame.
func (g *Game) advanceAnimation(dt time.Duration) {
	frames := g.frames()
	if len(frames) < 2 {
		g.frameIdx = 0
		return
	}
	// A pet mid-hop holds its frame for the same reason a paused one does:
	// legs working in mid-air are running on nothing.
	if g.walker.Paused() || g.walker.Airborne() {
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

// petScale is how much the artwork is enlarged: the setting multiplied by
// whatever the artwork itself asks for. That is what lets pet.scale mean the
// same thing across a sprite drawn at twelve pixels and an illustration drawn
// at two hundred.
func (g *Game) petScale() float64 {
	scale := g.pack.Scale
	if scale <= 0 {
		scale = 1
	}
	return g.cfg.Pet.Scale * scale
}

func (g *Game) petWidth() float64 { return float64(g.pack.Size.X) * g.petScale() }

func (g *Game) petHeight() float64 { return float64(g.pack.Size.Y) * g.petScale() }
