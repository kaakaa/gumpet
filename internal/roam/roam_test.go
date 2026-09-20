package roam

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/kaakaa/gumpet/internal/config"
)

const tick = time.Second / 30

// stage is a 600x400 area offset from the origin, so a walker that forgets to
// account for the stage's own position shows up as a failure.
var stage = Rect{X: 100, Y: 50, W: 600, H: 400}

const (
	petW = 200.0
	petH = 200.0
)

func newWalker(t *testing.T, mode config.Roam) *Walker {
	t.Helper()
	// A fixed seed keeps a failure reproducible.
	return New(mode, stage, petW, petH, 60, rand.New(rand.NewPCG(1, 2)))
}

// walk runs the walker for d and reports every position it passed through.
func walk(w *Walker, d time.Duration) [][2]float64 {
	var seen [][2]float64
	for elapsed := time.Duration(0); elapsed < d; elapsed += tick {
		w.Step(tick)
		x, y := w.Pos()
		seen = append(seen, [2]float64{x, y})
	}
	return seen
}

func assertInsideStage(t *testing.T, positions [][2]float64) {
	t.Helper()
	const epsilon = 1e-9
	for i, p := range positions {
		if p[0] < stage.X-epsilon || p[0] > stage.X+stage.W-petW+epsilon {
			t.Fatalf("step %d: x = %v, outside [%v, %v]", i, p[0], stage.X, stage.X+stage.W-petW)
		}
		if p[1] < stage.Y-epsilon || p[1] > stage.Y+stage.H-petH+epsilon {
			t.Fatalf("step %d: y = %v, outside [%v, %v]", i, p[1], stage.Y, stage.Y+stage.H-petH)
		}
	}
}

func TestNoneStandsStill(t *testing.T) {
	w := newWalker(t, config.RoamNone)
	start, startY := w.Pos()

	walk(w, 30*time.Second)

	x, y := w.Pos()
	if x != start || y != startY {
		t.Errorf("moved to (%v, %v) from (%v, %v)", x, y, start, startY)
	}
	if w.Moving() {
		t.Error("Moving() is true for a pet that stands still")
	}
}

func TestNoneStartsOnTheFloor(t *testing.T) {
	w := newWalker(t, config.RoamNone)
	_, y := w.Pos()
	if want := stage.Y + stage.H - petH; y != want {
		t.Errorf("y = %v, want the floor at %v", y, want)
	}
}

// A horizontal walk keeps to the floor, except while hopping, which lifts the
// pet off it and puts it back. Never below it, in either case.
func TestHorizontalStaysOnTheFloorAndInsideTheStage(t *testing.T) {
	w := newWalker(t, config.RoamHorizontal)
	w.SetJumping(false)
	positions := walk(w, 60*time.Second)

	assertInsideStage(t, positions)
	floor := stage.Y + stage.H - petH
	for i, p := range positions {
		if p[1] != floor {
			t.Fatalf("step %d: y = %v, want the floor at %v", i, p[1], floor)
		}
	}
}

// With hopping on, the floor is a bound rather than a fixed position.
func TestAHoppingWalkNeverSinksBelowTheFloor(t *testing.T) {
	w := newWalker(t, config.RoamHorizontal)
	positions := walk(w, 120*time.Second)

	assertInsideStage(t, positions)
	floor := stage.Y + stage.H - petH
	for i, p := range positions {
		if p[1] > floor {
			t.Fatalf("step %d: y = %v, which is below the floor at %v", i, p[1], floor)
		}
	}
}

func TestHorizontalTurnsAround(t *testing.T) {
	w := newWalker(t, config.RoamHorizontal)
	positions := walk(w, 60*time.Second)

	var wentLeft, wentRight bool
	for i := 1; i < len(positions); i++ {
		switch d := positions[i][0] - positions[i-1][0]; {
		case d > 0:
			wentRight = true
		case d < 0:
			wentLeft = true
		}
	}
	if !wentLeft || !wentRight {
		t.Errorf("walked left = %v, right = %v; want both", wentLeft, wentRight)
	}
}

func TestPerimeterVisitsAllFourEdges(t *testing.T) {
	w := newWalker(t, config.RoamPerimeter)
	positions := walk(w, 120*time.Second)

	assertInsideStage(t, positions)

	left, right := stage.X, stage.X+stage.W-petW
	top, bottom := stage.Y, stage.Y+stage.H-petH
	var onLeft, onRight, onTop, onBottom bool
	for _, p := range positions {
		switch {
		case p[0] == left:
			onLeft = true
		case p[0] == right:
			onRight = true
		}
		switch {
		case p[1] == top:
			onTop = true
		case p[1] == bottom:
			onBottom = true
		}
	}
	if !onLeft || !onRight || !onTop || !onBottom {
		t.Errorf("edges reached: left=%v right=%v top=%v bottom=%v; want all four",
			onLeft, onRight, onTop, onBottom)
	}
}

func TestPerimeterHugsAnEdgeAtAllTimes(t *testing.T) {
	w := newWalker(t, config.RoamPerimeter)
	positions := walk(w, 120*time.Second)

	left, right := stage.X, stage.X+stage.W-petW
	top, bottom := stage.Y, stage.Y+stage.H-petH
	for i, p := range positions {
		if p[0] != left && p[0] != right && p[1] != top && p[1] != bottom {
			t.Fatalf("step %d: (%v, %v) is not on any edge", i, p[0], p[1])
		}
	}
}

func TestPerimeterNeverStopsPartWayAlongAnEdge(t *testing.T) {
	w := newWalker(t, config.RoamPerimeter)
	positions := walk(w, 120*time.Second)

	for i := 1; i < len(positions); i++ {
		if positions[i] == positions[i-1] {
			t.Fatalf("step %d: stopped at (%v, %v)", i, positions[i][0], positions[i][1])
		}
	}
}

func TestWanderCoversBothAxesAndStaysInside(t *testing.T) {
	w := newWalker(t, config.RoamWander)
	positions := walk(w, 120*time.Second)

	assertInsideStage(t, positions)

	minX, maxX := positions[0][0], positions[0][0]
	minY, maxY := positions[0][1], positions[0][1]
	for _, p := range positions {
		minX, maxX = min(minX, p[0]), max(maxX, p[0])
		minY, maxY = min(minY, p[1]), max(maxY, p[1])
	}
	// The pet should get around, not shuffle in one corner.
	if maxX-minX < (stage.W-petW)/2 {
		t.Errorf("horizontal range %v is too narrow for a stage %v wide", maxX-minX, stage.W-petW)
	}
	if maxY-minY < (stage.H-petH)/2 {
		t.Errorf("vertical range %v is too short for a stage %v tall", maxY-minY, stage.H-petH)
	}
}

// A wandering pet stops to look around now and then, and while it does its
// animation should hold rather than mime walking on the spot.
func TestAWanderingPetSometimesStopsAndThenGoesOnAgain(t *testing.T) {
	w := newWalker(t, config.RoamWander)

	var paused, movedAfterPause bool
	var lastX, lastY float64
	for elapsed := time.Duration(0); elapsed < 5*time.Minute; elapsed += tick {
		w.Step(tick)
		x, y := w.Pos()
		if w.Paused() {
			paused = true
			if x != lastX || y != lastY {
				t.Fatal("moved while paused")
			}
			if w.Moving() {
				t.Fatal("Moving() is true while paused")
			}
		} else if paused && (x != lastX || y != lastY) {
			movedAfterPause = true
		}
		lastX, lastY = x, y
	}
	if !paused {
		t.Fatal("never stopped to look around in five minutes")
	}
	if !movedAfterPause {
		t.Error("never started walking again")
	}
}

// A pet that no longer fits its stage — after a rescale, or a switch to a
// smaller monitor — has to end up somewhere sane rather than off in space.
func TestReshapeKeepsThePetInsideASmallerStage(t *testing.T) {
	w := newWalker(t, config.RoamHorizontal)
	walk(w, 10*time.Second)

	small := Rect{X: 0, Y: 0, W: 300, H: 250}
	w.Reshape(config.RoamHorizontal, small, petW, petH, 60)

	x, y := w.Pos()
	if x < small.X || x > small.X+small.W-petW {
		t.Errorf("x = %v, outside the new stage", x)
	}
	if y < small.Y || y > small.Y+small.H-petH {
		t.Errorf("y = %v, outside the new stage", y)
	}
	assertInsideStage2(t, walk(w, 30*time.Second), small)
}

func TestReshapeToAStageSmallerThanThePet(t *testing.T) {
	w := newWalker(t, config.RoamWander)
	tiny := Rect{X: 10, Y: 20, W: 50, H: 50}
	w.Reshape(config.RoamWander, tiny, petW, petH, 60)

	walk(w, 10*time.Second)

	x, y := w.Pos()
	if x != tiny.X || y != tiny.Y {
		t.Errorf("(%v, %v), want the pet pinned to the stage origin (%v, %v)", x, y, tiny.X, tiny.Y)
	}
}

func assertInsideStage2(t *testing.T, positions [][2]float64, area Rect) {
	t.Helper()
	const epsilon = 1e-9
	for i, p := range positions {
		if p[0] < area.X-epsilon || p[0] > area.X+area.W-petW+epsilon {
			t.Fatalf("step %d: x = %v is outside the stage", i, p[0])
		}
		if p[1] < area.Y-epsilon || p[1] > area.Y+area.H-petH+epsilon {
			t.Fatalf("step %d: y = %v is outside the stage", i, p[1])
		}
	}
}

func TestFacingFollowsTheDirectionOfTravel(t *testing.T) {
	w := newWalker(t, config.RoamHorizontal)
	left, right := stage.X, stage.X+stage.W-petW

	var checked int
	for range 600 {
		w.Step(tick)
		x, _ := w.Pos()
		w.Step(tick)
		next, _ := w.Pos()

		// The step that reaches a wall is the step the pet turns on, so its
		// new facing deliberately disagrees with the way it just travelled.
		if next == left || next == right || next == x {
			continue
		}
		checked++
		if next > x && !w.FacingRight() {
			t.Fatalf("moved right from %v to %v but is facing left", x, next)
		}
		if next < x && w.FacingRight() {
			t.Fatalf("moved left from %v to %v but is facing right", x, next)
		}
	}
	if checked == 0 {
		t.Fatal("no steps were checked")
	}
}

func TestZeroSpeedDoesNotMove(t *testing.T) {
	w := New(config.RoamWander, stage, petW, petH, 0, rand.New(rand.NewPCG(1, 2)))
	before, beforeY := w.Pos()

	walk(w, 10*time.Second)

	after, afterY := w.Pos()
	if before != after || beforeY != afterY {
		t.Errorf("moved at zero speed: (%v, %v) -> (%v, %v)", before, beforeY, after, afterY)
	}
}

func TestStartingPositionIsRandom(t *testing.T) {
	for _, mode := range []config.Roam{
		config.RoamNone, config.RoamHorizontal, config.RoamPerimeter, config.RoamWander,
	} {
		t.Run(string(mode), func(t *testing.T) {
			seen := map[[2]float64]bool{}
			for seed := range uint64(40) {
				w := New(mode, stage, petW, petH, 60, rand.New(rand.NewPCG(seed, seed+1)))
				x, y := w.Pos()
				seen[[2]float64{x, y}] = true
			}
			// A fixed start would give exactly one position across every seed.
			if len(seen) < 20 {
				t.Errorf("only %d distinct starting positions out of 40 runs", len(seen))
			}
		})
	}
}

func TestStartingPositionIsInsideTheStage(t *testing.T) {
	for _, mode := range []config.Roam{
		config.RoamNone, config.RoamHorizontal, config.RoamPerimeter, config.RoamWander,
	} {
		t.Run(string(mode), func(t *testing.T) {
			for seed := range uint64(40) {
				w := New(mode, stage, petW, petH, 60, rand.New(rand.NewPCG(seed, seed+1)))
				assertInsideStage(t, [][2]float64{posOf(w)})
			}
		})
	}
}

func TestAPetThatDoesNotWanderStartsOnTheFloor(t *testing.T) {
	floor := stage.Y + stage.H - petH
	for _, mode := range []config.Roam{config.RoamNone, config.RoamHorizontal} {
		t.Run(string(mode), func(t *testing.T) {
			for seed := range uint64(20) {
				w := New(mode, stage, petW, petH, 60, rand.New(rand.NewPCG(seed, seed+1)))
				if _, y := w.Pos(); y != floor {
					t.Fatalf("seed %d: y = %v, want the floor at %v", seed, y, floor)
				}
			}
		})
	}
}

func TestAPerimeterWalkStartsOnAnEdge(t *testing.T) {
	left, right := stage.X, stage.X+stage.W-petW
	top, bottom := stage.Y, stage.Y+stage.H-petH
	for seed := range uint64(40) {
		w := New(config.RoamPerimeter, stage, petW, petH, 60, rand.New(rand.NewPCG(seed, seed+1)))
		x, y := w.Pos()
		if x != left && x != right && y != top && y != bottom {
			t.Fatalf("seed %d: (%v, %v) is not on an edge", seed, x, y)
		}
	}
}

// Changing the style of roaming should not fling the pet across the screen.
func TestReshapeToPerimeterSnapsToTheNearestEdge(t *testing.T) {
	w := New(config.RoamWander, stage, petW, petH, 60, rand.New(rand.NewPCG(7, 8)))
	// Put it just under the top edge, well away from the other three.
	w.x, w.y = stage.X+(stage.W-petW)/2, stage.Y+4

	w.Reshape(config.RoamPerimeter, stage, petW, petH, 60)

	x, y := w.Pos()
	if y != stage.Y {
		t.Errorf("y = %v, want the top edge at %v", y, stage.Y)
	}
	if x != stage.X+(stage.W-petW)/2 {
		t.Errorf("x = %v, want it left where it was", x)
	}
}

func TestReshapeWithoutAModeChangeLeavesThePetAlone(t *testing.T) {
	w := newWalker(t, config.RoamWander)
	walk(w, 5*time.Second)
	before := posOf(w)

	w.Reshape(config.RoamWander, stage, petW, petH, 60)

	if posOf(w) != before {
		t.Errorf("moved to %v from %v", posOf(w), before)
	}
}

func posOf(w *Walker) [2]float64 {
	x, y := w.Pos()
	return [2]float64{x, y}
}

// Translate is for the stage moving under the pet, which is what dragging it
// does. The pet has to come along, or the window lags behind the cursor.
func TestTranslateCarriesThePetAlong(t *testing.T) {
	w := New(config.RoamWander, Rect{X: 0, Y: 0, W: 500, H: 500}, 50, 50, 40, rand.New(rand.NewPCG(1, 2)))
	x0, y0 := w.Pos()

	w.Translate(30, -20)

	x, y := w.Pos()
	if x != x0+30 || y != y0-20 {
		t.Errorf("Pos = (%v, %v), want (%v, %v)", x, y, x0+30, y0-20)
	}
}

// It must still respect the stage it is in, or a big move would put the pet
// outside its own walls.
func TestTranslateKeepsThePetInsideTheStage(t *testing.T) {
	area := Rect{X: 0, Y: 0, W: 500, H: 500}
	w := New(config.RoamWander, area, 50, 50, 40, rand.New(rand.NewPCG(1, 2)))

	w.Translate(10000, 10000)

	x, y := w.Pos()
	if x < area.X || x > area.X+area.W-50 {
		t.Errorf("x = %v, want it inside the stage", x)
	}
	if y < area.Y || y > area.Y+area.H-50 {
		t.Errorf("y = %v, want it inside the stage", y)
	}
}

// hopHeights runs the walker and returns how far above the floor it got on
// each step, which is what every test about hopping is really asking about.
func hopHeights(w *Walker, d time.Duration) []float64 {
	floor := stage.Y + stage.H - petH
	var out []float64
	for elapsed := time.Duration(0); elapsed < d; elapsed += tick {
		w.Step(tick)
		_, y := w.Pos()
		out = append(out, floor-y)
	}
	return out
}

func TestAPetOnTheFloorEventuallyHops(t *testing.T) {
	w := newWalker(t, config.RoamHorizontal)
	heights := hopHeights(w, 5*time.Minute)

	var highest float64
	for _, h := range heights {
		if h > highest {
			highest = h
		}
	}
	if highest <= 0 {
		t.Fatal("the pet never left the floor in five minutes")
	}
}

// The arc has to come back down. A hop that leaves the pet hanging is worse
// than no hop at all.
func TestEveryHopLands(t *testing.T) {
	w := newWalker(t, config.RoamHorizontal)
	heights := hopHeights(w, 5*time.Minute)

	airborne := 0
	longest := 0
	for _, h := range heights {
		if h > 0 {
			airborne++
			if airborne > longest {
				longest = airborne
			}
			continue
		}
		airborne = 0
	}
	if longest == 0 {
		t.Fatal("never hopped")
	}
	// jumpSeconds of air at the test's tick rate, with room for rounding.
	if maxTicks := int(jumpSeconds/tick.Seconds()) + 4; longest > maxTicks {
		t.Errorf("a hop lasted %d ticks, want at most about %d", longest, maxTicks)
	}
	if last := heights[len(heights)-1]; last < 0 {
		t.Errorf("finished %v below the floor", -last)
	}
}

// The height comes from the pet, so that artwork of any size hops by an amount
// that looks like its own.
func TestAHopIsAboutAsHighAsThePetAsksFor(t *testing.T) {
	w := newWalker(t, config.RoamHorizontal)
	heights := hopHeights(w, 5*time.Minute)

	var highest float64
	for _, h := range heights {
		if h > highest {
			highest = h
		}
	}
	want := jumpPeak * petH
	if highest < want*0.8 || highest > want*1.2 {
		t.Errorf("highest hop was %v, want roughly %v", highest, want)
	}
}

func TestAHopCarriesThePetForward(t *testing.T) {
	w := newWalker(t, config.RoamHorizontal)
	floor := stage.Y + stage.H - petH

	var movedWhileAirborne bool
	lastX, _ := w.Pos()
	for elapsed := time.Duration(0); elapsed < 5*time.Minute; elapsed += tick {
		w.Step(tick)
		x, y := w.Pos()
		if floor-y > 0 && x != lastX {
			movedWhileAirborne = true
			break
		}
		lastX = x
	}
	if !movedWhileAirborne {
		t.Error("the pet hopped on the spot, want it to hop along")
	}
}

// Airborne is what tells the drawing to hold a frame, so it has to agree with
// where the pet actually is.
func TestAirborneAgreesWithBeingOffTheFloor(t *testing.T) {
	w := newWalker(t, config.RoamHorizontal)
	floor := stage.Y + stage.H - petH

	for elapsed := time.Duration(0); elapsed < 5*time.Minute; elapsed += tick {
		w.Step(tick)
		_, y := w.Pos()
		if off := floor-y > 0; off != w.Airborne() {
			t.Fatalf("Airborne() = %v but the pet is %v above the floor", w.Airborne(), floor-y)
		}
	}
}

// Hopping belongs to a floor. Drifting about the middle of the screen has no
// floor to leave, and walking the edges would mean hopping off a wall.
func TestOnlyFloorWalkersHop(t *testing.T) {
	for _, mode := range []config.Roam{config.RoamWander, config.RoamPerimeter} {
		w := newWalker(t, mode)
		for elapsed := time.Duration(0); elapsed < 5*time.Minute; elapsed += tick {
			w.Step(tick)
			if w.Airborne() {
				t.Fatalf("%s hopped", mode)
			}
		}
	}
}

// A pet told to stand still has nothing else to show it is running.
func TestAStationaryPetStillHops(t *testing.T) {
	w := newWalker(t, config.RoamNone)
	heights := hopHeights(w, 5*time.Minute)

	var hopped bool
	startX, _ := w.Pos()
	for _, h := range heights {
		if h > 0 {
			hopped = true
		}
	}
	if !hopped {
		t.Error("a stationary pet never hopped")
	}
	if x, _ := w.Pos(); x != startX {
		t.Errorf("it wandered off to %v from %v; hopping should not move it sideways", x, startX)
	}
}

func TestJumpingCanBeTurnedOff(t *testing.T) {
	w := newWalker(t, config.RoamHorizontal)
	w.SetJumping(false)

	for elapsed := time.Duration(0); elapsed < 5*time.Minute; elapsed += tick {
		w.Step(tick)
		if w.Airborne() {
			t.Fatal("hopped with jumping off")
		}
	}
}

// Turning it off mid-hop must put the pet down, not leave it in the air.
func TestTurningJumpingOffLandsThePet(t *testing.T) {
	w := newWalker(t, config.RoamHorizontal)
	floor := stage.Y + stage.H - petH

	for elapsed := time.Duration(0); elapsed < 5*time.Minute; elapsed += tick {
		w.Step(tick)
		if !w.Airborne() {
			continue
		}
		w.SetJumping(false)
		w.Step(tick)
		if _, y := w.Pos(); y != floor {
			t.Fatalf("left at %v after jumping was turned off, want the floor at %v", y, floor)
		}
		return
	}
	t.Skip("never got airborne")
}

// A patrol that only turns at the walls is a patrol. Changing its mind now and
// then is most of what makes it read as alive.
func TestThePetSometimesTurnsBackWithoutReachingAnEdge(t *testing.T) {
	w := newWalker(t, config.RoamHorizontal)
	w.SetJumping(false)
	minX, maxX := w.xRange()

	var turnedMidFloor bool
	lastX, _ := w.Pos()
	var lastDir float64
	for elapsed := time.Duration(0); elapsed < 10*time.Minute; elapsed += tick {
		w.Step(tick)
		x, _ := w.Pos()
		d := x - lastX
		if d != 0 && lastDir != 0 && (d > 0) != (lastDir > 0) {
			// In the middle half of the floor, so the wall did not cause it.
			margin := (maxX - minX) / 4
			if x > minX+margin && x < maxX-margin {
				turnedMidFloor = true
				break
			}
		}
		if d != 0 {
			lastDir = d
		}
		lastX = x
	}
	if !turnedMidFloor {
		t.Error("the pet only ever turned at the walls")
	}
}
