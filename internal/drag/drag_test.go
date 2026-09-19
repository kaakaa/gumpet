package drag

import "testing"

var monitor = Rect{W: 1920, H: 1080}

func stageAt(x, y float64) Rect { return Rect{X: x, Y: y, W: 520, H: 360} }

// The gesture that must not become a drag: press and release without moving,
// which is how the menu is opened.
func TestAPressThatDoesNotMoveIsAClick(t *testing.T) {
	var d Tracker
	stage := stageAt(100, 100)

	d.Press(200, 200, stage)
	if _, _, dragging := d.Move(200, 200, stage, monitor); dragging {
		t.Error("a still cursor started a drag")
	}
	if d.Release() {
		t.Error("Release said it was a drag, want a click")
	}
}

// A hand is never perfectly still, so a pixel or two must still be a click.
func TestATinyWobbleIsStillAClick(t *testing.T) {
	var d Tracker
	stage := stageAt(100, 100)

	d.Press(200, 200, stage)
	for _, p := range [][2]float64{{201, 200}, {200, 201}, {202, 202}, {199, 198}} {
		if _, _, dragging := d.Move(p[0], p[1], stage, monitor); dragging {
			t.Fatalf("moving to %v started a drag, want it under the threshold", p)
		}
	}
	if d.Release() {
		t.Error("Release said it was a drag")
	}
}

func TestMovingFarEnoughStartsADrag(t *testing.T) {
	var d Tracker
	stage := stageAt(100, 100)

	d.Press(200, 200, stage)
	x, y, dragging := d.Move(200+Threshold, 200, stage, monitor)
	if !dragging {
		t.Fatalf("moving %v did not start a drag", Threshold)
	}
	if x != 100+Threshold || y != 100 {
		t.Errorf("stage moved to (%v, %v), want (%v, 100)", x, y, 100+Threshold)
	}
	if !d.Release() {
		t.Error("Release said it was a click, want a drag")
	}
}

// The stage should follow the cursor rather than jump so that the point
// grabbed sits under the pointer.
func TestTheStageFollowsTheCursorByTheSameAmount(t *testing.T) {
	var d Tracker
	stage := stageAt(300, 400)

	d.Press(500, 500, stage)
	x, y, dragging := d.Move(560, 470, stage, monitor)
	if !dragging {
		t.Fatal("no drag")
	}
	if x != 360 || y != 370 {
		t.Errorf("stage at (%v, %v), want (360, 370) — moved by the cursor's own delta", x, y)
	}
}

// Once it is a drag it stays one, or releasing back at the start would open
// the menu after the pet had been dragged around the screen.
func TestADragThatReturnsHomeIsStillADrag(t *testing.T) {
	var d Tracker
	stage := stageAt(100, 100)

	d.Press(200, 200, stage)
	d.Move(400, 400, stage, monitor)
	if _, _, dragging := d.Move(200, 200, stage, monitor); !dragging {
		t.Error("coming back to the start ended the drag")
	}
	if !d.Release() {
		t.Error("Release said it was a click after a round trip")
	}
}

func TestMovingWithNoPressDoesNothing(t *testing.T) {
	var d Tracker
	stage := stageAt(100, 100)

	x, y, dragging := d.Move(900, 900, stage, monitor)
	if dragging {
		t.Error("moving without pressing started a drag")
	}
	if x != 100 || y != 100 {
		t.Errorf("stage moved to (%v, %v) without a press", x, y)
	}
}

// A stage dragged off the screen could not be dragged back, so it is kept
// wholly on the monitor.
func TestTheStageCannotBeThrownOffTheMonitor(t *testing.T) {
	cases := []struct {
		name  string
		to    [2]float64
		wantX float64
		wantY float64
	}{
		{"past the left", [2]float64{-5000, 500}, 0, 100},
		{"past the top", [2]float64{500, -5000}, 100, 0},
		{"past the right", [2]float64{5000, 500}, 1920 - 520, 100},
		{"past the bottom", [2]float64{500, 5000}, 100, 1080 - 360},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var d Tracker
			stage := stageAt(100, 100)
			d.Press(500, 500, stage)
			x, y, dragging := d.Move(c.to[0], c.to[1], stage, monitor)
			if !dragging {
				t.Fatal("no drag")
			}
			if x != c.wantX || y != c.wantY {
				t.Errorf("stage at (%v, %v), want (%v, %v)", x, y, c.wantX, c.wantY)
			}
		})
	}
}

// A stage as big as the monitor, or bigger, has nowhere to go.
func TestAStageTooBigToMoveStaysAtTheCorner(t *testing.T) {
	var d Tracker
	stage := Rect{X: 0, Y: 0, W: 3000, H: 2000}

	d.Press(500, 500, stage)
	x, y, dragging := d.Move(900, 900, stage, monitor)
	if !dragging {
		t.Fatal("no drag")
	}
	if x != 0 || y != 0 {
		t.Errorf("stage at (%v, %v), want the corner", x, y)
	}
}

func TestDraggingReportsTheStateBetweenPressAndRelease(t *testing.T) {
	var d Tracker
	stage := stageAt(100, 100)

	if d.Dragging() {
		t.Error("a fresh tracker says it is dragging")
	}
	d.Press(200, 200, stage)
	if d.Dragging() {
		t.Error("says it is dragging before the threshold")
	}
	d.Move(400, 400, stage, monitor)
	if !d.Dragging() {
		t.Error("says it is not dragging mid-drag")
	}
	d.Release()
	if d.Dragging() {
		t.Error("still dragging after release")
	}
}

// Cancel is for the cases where the gesture has to be abandoned — the settings
// turning click-through on mid-drag, say. It must count as neither.
func TestCancelEndsTheGestureAsNeither(t *testing.T) {
	var d Tracker
	stage := stageAt(100, 100)

	d.Press(200, 200, stage)
	d.Move(400, 400, stage, monitor)
	d.Cancel()

	if d.Dragging() {
		t.Error("still dragging after Cancel")
	}
	if d.Release() {
		t.Error("Release after Cancel said it was a drag")
	}
}

// Two gestures in a row must not leak into each other.
func TestASecondGestureStartsClean(t *testing.T) {
	var d Tracker

	stage := stageAt(100, 100)
	d.Press(200, 200, stage)
	d.Move(600, 600, stage, monitor)
	x, y, _ := d.Move(600, 600, stage, monitor)
	d.Release()

	stage = stageAt(x, y)
	d.Press(700, 700, stage)
	if _, _, dragging := d.Move(701, 700, stage, monitor); dragging {
		t.Error("the second press was already a drag")
	}
	if d.Release() {
		t.Error("the second gesture was reported as a drag")
	}
}
