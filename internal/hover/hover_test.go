package hover

import (
	"testing"
	"time"
)

const tick = time.Second / 30

// hold runs the tracker for d with the cursor parked where it is, and reports
// whether it was still holding at the end.
func hold(t *testing.T, tr *Tracker, x, y float64, over bool, d time.Duration) bool {
	t.Helper()
	holding := false
	for elapsed := time.Duration(0); elapsed < d; elapsed += tick {
		holding = tr.Update(x, y, over, tick)
	}
	return holding
}

func TestReachingForThePetStopsIt(t *testing.T) {
	var tr Tracker

	// The cursor arrives from somewhere else and lands on the pet.
	tr.Update(100, 100, false, tick)
	if !tr.Update(200, 200, true, tick) {
		t.Error("a cursor that just moved onto the pet does not hold it")
	}
}

// The bug this package exists for: the pet wanders under a cursor that has
// been lying there untouched, and stops dead.
func TestAForgottenCursorDoesNotStopThePet(t *testing.T) {
	var tr Tracker

	// The cursor is put down and left alone, nowhere near the pet.
	if hold(t, &tr, 500, 500, false, 30*time.Second) {
		t.Fatal("holding before the pet even arrived")
	}

	// The pet walks under it. Nothing about the cursor changed.
	if tr.Update(500, 500, true, tick) {
		t.Error("a cursor that has not moved in half a minute still stopped the pet")
	}
}

func TestAHoldLapsesWhenTheCursorSettles(t *testing.T) {
	var tr Tracker
	tr.Update(100, 100, false, tick)

	if !tr.Update(200, 200, true, tick) {
		t.Fatal("the hold did not start")
	}
	if !hold(t, &tr, 200, 200, true, IdleLimit-time.Second) {
		t.Error("the hold lapsed before the limit was up")
	}
	if hold(t, &tr, 200, 200, true, 2*time.Second) {
		t.Error("the hold outlasted the limit")
	}
}

func TestMovingAgainTakesTheHoldBack(t *testing.T) {
	var tr Tracker
	tr.Update(200, 200, true, tick)
	hold(t, &tr, 200, 200, true, IdleLimit+time.Second)

	if !tr.Update(240, 240, true, tick) {
		t.Error("moving the cursor again did not stop the pet")
	}
}

// The window travels with the pet, so a still cursor reports a pixel or two of
// drift. That must not read as a hand reaching for the pet.
func TestJitterIsNotMovement(t *testing.T) {
	var tr Tracker
	tr.Update(200, 200, true, tick)

	drift := 0.0
	for elapsed := time.Duration(0); elapsed < IdleLimit+time.Second; elapsed += tick {
		drift += 0.1
		if drift > MoveThreshold-0.5 {
			drift = 0
		}
		tr.Update(200+drift, 200-drift, true, tick)
	}

	if tr.Update(200, 200, true, tick) {
		t.Error("sub-threshold drift kept the hold alive")
	}
}

func TestMovementAtTheThresholdCounts(t *testing.T) {
	var tr Tracker
	tr.Update(200, 200, true, tick)
	hold(t, &tr, 200, 200, true, IdleLimit+time.Second)

	if !tr.Update(200+MoveThreshold, 200, true, tick) {
		t.Error("a move of exactly the threshold did not count")
	}
}

func TestNothingIsHeldWhileTheCursorIsElsewhere(t *testing.T) {
	var tr Tracker

	if tr.Update(10, 10, false, tick) {
		t.Error("held the pet with the cursor nowhere near it")
	}
	if tr.Update(20, 20, false, tick) {
		t.Error("held the pet with a moving cursor that is still nowhere near it")
	}
}

// A cursor already resting on the pet when gumpet starts has not been seen
// move, but neither has it been seen sitting still — give it its moment rather
// than ignoring it.
func TestTheFirstPositionCountsAsMovement(t *testing.T) {
	var tr Tracker

	if !tr.Update(200, 200, true, tick) {
		t.Error("the very first reading did not hold the pet")
	}
}

func TestResetForgetsEverything(t *testing.T) {
	var tr Tracker
	tr.Update(200, 200, true, tick)
	hold(t, &tr, 200, 200, true, IdleLimit+time.Second)
	if tr.Idle() == 0 {
		t.Fatal("the tracker did not accumulate any idle time to forget")
	}

	tr.Reset()

	if tr.Idle() != 0 {
		t.Errorf("Idle = %v after Reset, want 0", tr.Idle())
	}
	if !tr.Update(200, 200, true, tick) {
		t.Error("after Reset the cursor was not treated as newly seen")
	}
}

func TestIdleReportsHowLongTheCursorHasSat(t *testing.T) {
	var tr Tracker
	tr.Update(200, 200, true, tick)

	hold(t, &tr, 200, 200, true, time.Second)

	if got := tr.Idle(); got < 900*time.Millisecond || got > 1100*time.Millisecond {
		t.Errorf("Idle = %v, want about a second", got)
	}
}
