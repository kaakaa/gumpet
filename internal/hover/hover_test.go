package hover

import (
	"testing"
	"time"
)

const tick = time.Second / 30

// still runs the tracker for d with the cursor left exactly where it is, and
// reports whether it was holding at the end.
func still(t *testing.T, tr *Tracker, x, y float64, inside, over bool, d time.Duration) bool {
	t.Helper()
	holding := false
	for elapsed := time.Duration(0); elapsed < d; elapsed += tick {
		holding = tr.Update(x, y, inside, over, tick)
	}
	return holding
}

// reach walks the cursor towards the pet and onto it, the way a hand does.
func reach(t *testing.T, tr *Tracker) bool {
	t.Helper()
	holding := false
	// Approaching from outside the window, then the last stretch inside it.
	for x := 0.0; x < 60; x += 10 {
		holding = tr.Update(x, 0, false, false, tick)
	}
	for x := 60.0; x <= 100; x += 10 {
		holding = tr.Update(x, 0, true, true, tick)
	}
	return holding
}

func TestReachingForThePetStopsIt(t *testing.T) {
	var tr Tracker

	if !reach(t, &tr) {
		t.Error("a cursor moved onto the pet does not hold it")
	}
}

// The bug this package exists for: the pet wanders under a cursor that has
// been lying untouched, and stops dead.
func TestAForgottenCursorDoesNotStopThePet(t *testing.T) {
	var tr Tracker

	// The cursor is put down somewhere and left. It is outside the window, so
	// nothing about it can be read — which is the whole difficulty.
	still(t, &tr, 500, 500, false, false, 30*time.Second)

	// The pet walks over it. The window now contains the cursor, so a position
	// can be read again, and it is nowhere near the last one that was.
	if tr.Update(500, 500, true, true, tick) {
		t.Error("a cursor untouched for half a minute stopped the pet it was lying under")
	}
	if still(t, &tr, 500, 500, true, true, time.Second) {
		t.Error("it stopped the pet a moment later instead")
	}
}

func TestAHoldLapsesWhenTheCursorSettles(t *testing.T) {
	var tr Tracker
	if !reach(t, &tr) {
		t.Fatal("the hold did not start")
	}

	if !still(t, &tr, 100, 0, true, true, IdleLimit-time.Second) {
		t.Error("the hold lapsed before the limit was up")
	}
	if still(t, &tr, 100, 0, true, true, 2*time.Second) {
		t.Error("the hold outlasted the limit")
	}
}

// What the user sees after a hold lapses: the pet starts walking, and must not
// be caught again by the same cursor it just escaped.
func TestAPetThatGetsGoingIsNotCaughtAgain(t *testing.T) {
	var tr Tracker
	reach(t, &tr)
	still(t, &tr, 100, 0, true, true, IdleLimit+time.Second)

	// It walks off. The cursor stays exactly where it was: on screen it does
	// not move, whatever the window does around it.
	for range 60 {
		if tr.Update(100, 0, true, true, tick) {
			t.Fatal("caught again while walking out from under the cursor")
		}
	}
	// And once it is clear of the cursor, and later passes back under it.
	still(t, &tr, 100, 0, false, false, 5*time.Second)
	if tr.Update(100, 0, true, true, tick) {
		t.Error("caught again on the way back round")
	}
}

func TestMovingAgainTakesTheHoldBack(t *testing.T) {
	var tr Tracker
	reach(t, &tr)
	still(t, &tr, 100, 0, true, true, IdleLimit+time.Second)

	if !tr.Update(140, 40, true, true, tick) {
		t.Error("moving the cursor again did not stop the pet")
	}
}

func TestJitterIsNotMovement(t *testing.T) {
	var tr Tracker
	reach(t, &tr)

	drift := 0.0
	for elapsed := time.Duration(0); elapsed < IdleLimit+time.Second; elapsed += tick {
		drift += 0.1
		if drift > MoveThreshold-0.5 {
			drift = 0
		}
		tr.Update(100+drift, drift, true, true, tick)
	}

	if tr.Update(100, 0, true, true, tick) {
		t.Error("sub-threshold drift kept the hold alive")
	}
}

func TestMovementAtTheThresholdCounts(t *testing.T) {
	var tr Tracker
	reach(t, &tr)
	still(t, &tr, 100, 0, true, true, IdleLimit+time.Second)

	if !tr.Update(100+MoveThreshold, 0, true, true, tick) {
		t.Error("a move of exactly the threshold did not count")
	}
}

func TestNothingIsHeldFromOutsideTheWindow(t *testing.T) {
	var tr Tracker

	// Even claiming to be over the pet, which cannot happen, being outside the
	// window settles it.
	if tr.Update(10, 10, false, true, tick) {
		t.Error("held the pet from outside its window")
	}
}

func TestNothingIsHeldWhileTheCursorIsMerelyInTheWindow(t *testing.T) {
	var tr Tracker

	if tr.Update(10, 10, true, false, tick) {
		t.Error("held the pet with the cursor in the window but not on anything")
	}
	if tr.Update(40, 40, true, false, tick) {
		t.Error("held the pet with a moving cursor that is still on nothing")
	}
}

// Coming back into the window is not itself movement, however far the cursor
// appears to have jumped: nothing watched it cross.
func TestReturningToTheWindowIsNotMovement(t *testing.T) {
	var tr Tracker
	reach(t, &tr)
	still(t, &tr, 100, 0, true, true, IdleLimit+time.Second)

	tr.Update(100, 0, false, false, tick)
	if tr.Update(9000, 9000, true, true, tick) {
		t.Error("a jump across the screen while unwatched counted as movement")
	}
}

func TestResetForgetsEverything(t *testing.T) {
	var tr Tracker
	reach(t, &tr)
	still(t, &tr, 100, 0, true, true, IdleLimit+time.Second)
	if tr.Idle() == 0 {
		t.Fatal("the tracker did not accumulate any idle time to forget")
	}

	tr.Reset()

	if tr.Idle() != 0 {
		t.Errorf("Idle = %v after Reset, want 0", tr.Idle())
	}
}

func TestIdleReportsHowLongTheCursorHasSat(t *testing.T) {
	var tr Tracker
	reach(t, &tr)
	tr.Update(100, 0, true, true, tick) // settles, so idle starts counting

	still(t, &tr, 100, 0, true, true, time.Second)

	if got := tr.Idle(); got < 900*time.Millisecond || got > 1200*time.Millisecond {
		t.Errorf("Idle = %v, want about a second", got)
	}
}
