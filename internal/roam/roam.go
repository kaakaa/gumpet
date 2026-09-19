// Package roam moves the pet around its stage.
//
// It is pure geometry driven by a caller-supplied source of randomness, so
// every style of wandering can be tested without a display.
package roam

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/layout"
)

// Rect is the area the pet may move around in, in monitor pixels.
type Rect = layout.Rect

// pauseChancePerSecond is how often a roaming pet stops to look around, which
// works out at a pause every twenty seconds or so.
const pauseChancePerSecond = 0.05

// pauseSeconds is the range a pause is drawn from.
const (
	minPauseSeconds = 1
	maxPauseSeconds = 4
)

// Walker carries the pet around the stage. Its zero value is not usable; call
// [New].
type Walker struct {
	mode  config.Roam
	area  Rect
	petW  float64
	petH  float64
	speed float64
	rnd   *rand.Rand

	x, y   float64
	vx, vy float64
	// leg is which edge of the stage a perimeter walk is currently on.
	leg      int
	pauseFor time.Duration
}

// New starts a walker somewhere random on the stage. rnd may be nil, in which
// case the global source is used.
func New(mode config.Roam, area Rect, petW, petH, speed float64, rnd *rand.Rand) *Walker {
	w := &Walker{mode: mode, area: area, petW: petW, petH: petH, speed: speed, rnd: rnd}
	w.placeRandomly()
	w.launch()
	return w
}

// Reshape adopts a new stage, pet size or style of roaming, keeping the pet
// inside the stage. It does not move the pet any more than the new style
// demands: a restart is a fresh start, but changing a setting is not.
func (w *Walker) Reshape(mode config.Roam, area Rect, petW, petH, speed float64) {
	changed := w.mode != mode
	w.mode, w.area, w.petW, w.petH, w.speed = mode, area, petW, petH, speed
	w.clamp()
	if !changed {
		return
	}
	w.pauseFor = 0
	switch mode {
	case config.RoamNone, config.RoamHorizontal:
		w.y = w.floor()
		w.vx, w.vy = w.speed*w.sign(), 0
	case config.RoamPerimeter:
		w.snapToNearestEdge()
	case config.RoamWander:
		w.heading(w.rand()*2*math.Pi, w.speed)
	}
}

// placeRandomly drops the pet anywhere on the stage, so that a restart does not
// always put it back in the same spot.
func (w *Walker) placeRandomly() {
	minX, maxX := w.xRange()
	minY, maxY := w.yRange()
	w.x = minX + w.rand()*(maxX-minX)
	w.y = minY + w.rand()*(maxY-minY)
}

// Pos is the pet's top-left corner, in monitor pixels.
// Translate moves the pet by a delta without changing anything else about how
// it is walking. It is for the stage being moved under it — dragging the pet
// should carry it along rather than merely shifting the walls it bounces off,
// which would leave it behind until an edge caught up with it.
func (w *Walker) Translate(dx, dy float64) {
	w.x += dx
	w.y += dy
	w.clamp()
}

func (w *Walker) Pos() (x, y float64) { return w.x, w.y }

// FacingRight reports which way the pet is looking.
func (w *Walker) FacingRight() bool { return w.vx > 0 }

// Moving reports whether the pet is walking rather than standing still, which
// is what decides whether its animation plays.
func (w *Walker) Moving() bool {
	return w.mode != config.RoamNone && w.pauseFor <= 0 && (w.vx != 0 || w.vy != 0)
}

// Step advances the walk by dt.
func (w *Walker) Step(dt time.Duration) {
	if w.mode == config.RoamNone || w.speed <= 0 {
		return
	}
	if w.pauseFor > 0 {
		w.pauseFor -= dt
		return
	}
	// Decide about stopping before moving, so that a step the pet is paused
	// for is always a step it stayed put on.
	if w.maybePause(); w.pauseFor > 0 {
		return
	}

	switch w.mode {
	case config.RoamHorizontal:
		w.stepHorizontal(dt)
	case config.RoamPerimeter:
		w.stepPerimeter(dt)
	case config.RoamWander:
		w.stepWander(dt)
	}
	w.clamp()
}

// Paused reports whether the pet has stopped to look around, which is what
// tells the caller to hold its animation on one frame.
func (w *Walker) Paused() bool { return w.pauseFor > 0 }

// launch settles the pet into its style of roaming from wherever it was
// dropped, and points it somewhere.
func (w *Walker) launch() {
	switch w.mode {
	case config.RoamNone:
		w.y = w.floor()
		w.vx, w.vy = 0, 0
	case config.RoamHorizontal:
		w.y = w.floor()
		w.vx, w.vy = w.speed*w.sign(), 0
	case config.RoamPerimeter:
		w.startOnRandomEdge()
	case config.RoamWander:
		w.heading(w.rand()*2*math.Pi, w.speed)
	}
}

// startOnRandomEdge puts the pet anywhere along the stage's border, walking the
// way that edge goes.
func (w *Walker) startOnRandomEdge() {
	minX, maxX := w.xRange()
	minY, maxY := w.yRange()
	w.leg = w.rnd_n(4)
	switch w.leg {
	case 0:
		w.x, w.y = minX+w.rand()*(maxX-minX), maxY
	case 1:
		w.x, w.y = maxX, minY+w.rand()*(maxY-minY)
	case 2:
		w.x, w.y = minX+w.rand()*(maxX-minX), minY
	default:
		w.x, w.y = minX, minY+w.rand()*(maxY-minY)
	}
	w.faceAlongLeg()
}

// snapToNearestEdge moves the pet the shortest distance onto the border, which
// is how it takes up a perimeter walk without teleporting across the screen.
func (w *Walker) snapToNearestEdge() {
	minX, maxX := w.xRange()
	minY, maxY := w.yRange()
	gaps := []float64{maxY - w.y, maxX - w.x, w.y - minY, w.x - minX}

	w.leg = 0
	for i, gap := range gaps {
		if gap < gaps[w.leg] {
			w.leg = i
		}
	}
	switch w.leg {
	case 0:
		w.y = maxY
	case 1:
		w.x = maxX
	case 2:
		w.y = minY
	default:
		w.x = minX
	}
	w.faceAlongLeg()
}

// faceAlongLeg points the pet the way its current edge runs: right along the
// bottom, up the right side, left along the top, down the left side.
func (w *Walker) faceAlongLeg() {
	switch w.leg {
	case 0:
		w.vx, w.vy = w.speed, 0
	case 1:
		w.vx, w.vy = 0, -w.speed
	case 2:
		w.vx, w.vy = -w.speed, 0
	default:
		w.vx, w.vy = 0, w.speed
	}
}

// floor is where the pet's top-left corner sits when it is standing on the
// bottom of the stage.
func (w *Walker) floor() float64 {
	_, maxY := w.yRange()
	return maxY
}

func (w *Walker) stepHorizontal(dt time.Duration) {
	w.y = w.area.Y + w.area.H - w.petH
	if w.vx == 0 {
		w.vx = w.speed * w.sign()
	}
	w.x += w.vx * dt.Seconds()

	minX, maxX := w.xRange()
	if w.x <= minX && w.vx < 0 {
		w.x, w.vx = minX, -w.vx
	}
	if w.x >= maxX && w.vx > 0 {
		w.x, w.vx = maxX, -w.vx
	}
}

// stepPerimeter walks the four edges in turn, anticlockwise on screen: along
// the bottom to the right, up the right side, back along the top, down the left.
func (w *Walker) stepPerimeter(dt time.Duration) {
	minX, maxX := w.xRange()
	minY, maxY := w.yRange()
	d := w.speed * dt.Seconds()

	switch w.leg {
	case 0: // bottom edge, heading right
		w.y = maxY
		w.x += d
		if w.x >= maxX {
			w.x, w.leg = maxX, 1
			w.vx, w.vy = 0, -w.speed
			return
		}
		w.vx, w.vy = w.speed, 0
	case 1: // right edge, heading up
		w.x = maxX
		w.y -= d
		if w.y <= minY {
			w.y, w.leg = minY, 2
			w.vx, w.vy = -w.speed, 0
			return
		}
		w.vx, w.vy = 0, -w.speed
	case 2: // top edge, heading left
		w.y = minY
		w.x -= d
		if w.x <= minX {
			w.x, w.leg = minX, 3
			w.vx, w.vy = 0, w.speed
			return
		}
		w.vx, w.vy = -w.speed, 0
	default: // left edge, heading down
		w.x = minX
		w.y += d
		if w.y >= maxY {
			w.y, w.leg = maxY, 0
			w.vx, w.vy = w.speed, 0
			return
		}
		w.vx, w.vy = 0, w.speed
	}
}

// stepWander drifts in whatever direction the pet last took, turning at the
// walls and, now and then, for no reason at all.
func (w *Walker) stepWander(dt time.Duration) {
	if w.vx == 0 && w.vy == 0 {
		w.heading(w.rand()*2*math.Pi, w.speed)
	}
	sec := dt.Seconds()
	w.x += w.vx * sec
	w.y += w.vy * sec

	minX, maxX := w.xRange()
	minY, maxY := w.yRange()
	if (w.x <= minX && w.vx < 0) || (w.x >= maxX && w.vx > 0) {
		w.vx = -w.vx
	}
	if (w.y <= minY && w.vy < 0) || (w.y >= maxY && w.vy > 0) {
		w.vy = -w.vy
	}

	// An occasional turn keeps the path from settling into one diagonal.
	if w.rand() < 0.01*sec/(1.0/60.0) {
		w.heading(math.Atan2(w.vy, w.vx)+(w.rand()-0.5)*math.Pi/2, w.speed)
	}
}

func (w *Walker) heading(angle, speed float64) {
	w.vx = math.Cos(angle) * speed
	w.vy = math.Sin(angle) * speed
}

func (w *Walker) maybePause() {
	// A perimeter walk looks wrong if it stops halfway along an edge.
	if w.mode == config.RoamPerimeter {
		return
	}
	if w.rand() < pauseChancePerSecond/60 {
		w.pauseFor = time.Duration(minPauseSeconds+w.rnd_n(maxPauseSeconds-minPauseSeconds)) * time.Second
	}
}

// xRange and yRange are where the pet's top-left corner may be, given that the
// whole pet has to stay on the stage.
func (w *Walker) xRange() (min, max float64) {
	return w.area.X, math.Max(w.area.X, w.area.X+w.area.W-w.petW)
}

func (w *Walker) yRange() (min, max float64) {
	return w.area.Y, math.Max(w.area.Y, w.area.Y+w.area.H-w.petH)
}

func (w *Walker) clamp() {
	minX, maxX := w.xRange()
	minY, maxY := w.yRange()
	w.x = math.Min(math.Max(w.x, minX), maxX)
	w.y = math.Min(math.Max(w.y, minY), maxY)
}

func (w *Walker) sign() float64 {
	if w.rnd_n(2) == 0 {
		return -1
	}
	return 1
}

func (w *Walker) rand() float64 {
	if w.rnd == nil {
		return rand.Float64()
	}
	return w.rnd.Float64()
}

func (w *Walker) rnd_n(n int) int {
	if n <= 0 {
		return 0
	}
	if w.rnd == nil {
		return rand.IntN(n)
	}
	return w.rnd.IntN(n)
}
